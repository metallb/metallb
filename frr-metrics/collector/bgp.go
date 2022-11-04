// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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

	prefixesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, bgpstats.Prefixes.Name),
		bgpstats.Prefixes.Help,
		bgpstats.Labels,
		nil,
	)

	opensSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "opens_sent"),
		"Number of BGP open messages sent",
		bgpstats.Labels,
		nil,
	)

	opensReceivedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "opens_received"),
		"Number of BGP open messages received",
		bgpstats.Labels,
		nil,
	)

	notificationsSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "notifications_sent"),
		"Number of BGP notification messages sent",
		bgpstats.Labels,
		nil,
	)

	updatesSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, bgpstats.UpdatesSent.Name),
		bgpstats.UpdatesSent.Help,
		bgpstats.Labels,
		nil,
	)

	updatesReceivedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "updates_total_received"),
		"Number of BGP UPDATE messages received",
		bgpstats.Labels,
		nil,
	)

	keepalivesSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "keepalives_sent"),
		"Number of BGP keepalive messages sent",
		bgpstats.Labels,
		nil,
	)

	keepalivesReceivedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "keepalives_received"),
		"Number of BGP keepalive messages received",
		bgpstats.Labels,
		nil,
	)

	routeRefreshSentedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "route_refresh_sent"),
		"Number of BGP route refresh messages sent",
		bgpstats.Labels,
		nil,
	)

	totalSentDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "total_sent"),
		"Number of total BGP messages sent",
		bgpstats.Labels,
		nil,
	)

	totalReceivedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, bgpstats.Subsystem, "total_received"),
		"Number of total BGP messages received",
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
	ch <- prefixesDesc
	ch <- opensSentDesc
	ch <- opensReceivedDesc
	ch <- notificationsSentDesc
	ch <- updatesSentDesc
	ch <- updatesReceivedDesc
	ch <- keepalivesSentDesc
	ch <- keepalivesReceivedDesc
	ch <- routeRefreshSentedDesc
	ch <- totalSentDesc
	ch <- totalReceivedDesc
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
		ch <- prometheus.MustNewConstMetric(prefixesDesc, prometheus.GaugeValue, float64(n.PrefixSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(opensSentDesc, prometheus.CounterValue, float64(n.MsgStats.OpensSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(opensReceivedDesc, prometheus.CounterValue, float64(n.MsgStats.OpensReceived), peerLabel)
		ch <- prometheus.MustNewConstMetric(notificationsSentDesc, prometheus.CounterValue, float64(n.MsgStats.NotificationsSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(updatesSentDesc, prometheus.CounterValue, float64(n.MsgStats.UpdatesSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(updatesReceivedDesc, prometheus.CounterValue, float64(n.MsgStats.UpdatesReceived), peerLabel)
		ch <- prometheus.MustNewConstMetric(keepalivesSentDesc, prometheus.CounterValue, float64(n.MsgStats.KeepalivesSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(keepalivesReceivedDesc, prometheus.CounterValue, float64(n.MsgStats.KeepalivesReceived), peerLabel)
		ch <- prometheus.MustNewConstMetric(routeRefreshSentedDesc, prometheus.CounterValue, float64(n.MsgStats.RouteRefreshSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(totalSentDesc, prometheus.CounterValue, float64(n.MsgStats.TotalSent), peerLabel)
		ch <- prometheus.MustNewConstMetric(totalReceivedDesc, prometheus.CounterValue, float64(n.MsgStats.TotalReceived), peerLabel)
	}
}

func getBGPNeighbors(frrCli func(args ...string) (string, error)) ([]*bgpfrr.Neighbor, error) {
	res, err := frrCli("show bgp neighbors json")
	if err != nil {
		return nil, err
	}

	return bgpfrr.ParseNeighbours(res)
}
