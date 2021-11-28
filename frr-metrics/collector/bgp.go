// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	bgpstats "go.universe.tf/metallb/internal/bgp/native"
)

var (
	sessionUpDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, bgpstats.SessionUp.Name),
		bgpstats.SessionUp.Help,
		bgpstats.Labels,
		nil,
	)

	updatesSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, bgpstats.UpdatesSent.Name),
		bgpstats.UpdatesSent.Help,
		bgpstats.Labels,
		nil,
	)

	prefixesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, bgpstats.Prefixes.Name),
		bgpstats.Prefixes.Help,
		bgpstats.Labels,
		nil,
	)
)

type bgp struct {
	Log    log.Logger
	frrCli func(args ...string) (string, error)
}

func NewBGP(l log.Logger) *bgp {
	log := log.With(l, "collector", bgpstats.Subsystem)
	return &bgp{Log: log, frrCli: runVtysh}
}

func (c *bgp) Describe(ch chan<- *prometheus.Desc) {
	ch <- sessionUpDesc
	ch <- updatesSentDesc
	ch <- prefixesDesc
}

func (c *bgp) Collect(ch chan<- prometheus.Metric) {
	neighbors, err := getBGPNeighbors(c.frrCli)
	if err != nil {
		level.Error(c.Log).Log("error", err, "msg", "failed to fetch BGP neighbors from FRR")
		return
	}

	updateNeighborsMetrics(ch, neighbors)
}

func updateNeighborsMetrics(ch chan<- prometheus.Metric, neighbors []*bgpfrr.Neighbor) {
	for _, n := range neighbors {
		sessionUp := 1
		if !n.Connected {
			sessionUp = 0
		}
		peerLabel := fmt.Sprintf("%s:%d", n.Ip.String(), n.Port)

		ch <- prometheus.MustNewConstMetric(sessionUpDesc, prometheus.GaugeValue, float64(sessionUp), peerLabel)
		ch <- prometheus.MustNewConstMetric(updatesSentDesc, prometheus.CounterValue, float64(n.UpdatesSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(prefixesDesc, prometheus.GaugeValue, float64(n.PrefixSent), peerLabel)
	}
}

func getBGPNeighbors(frrCli func(args ...string) (string, error)) ([]*bgpfrr.Neighbor, error) {
	res, err := frrCli("show bgp neighbors json")
	if err != nil {
		return nil, err
	}

	return bgpfrr.ParseNeighbours(res)
}
