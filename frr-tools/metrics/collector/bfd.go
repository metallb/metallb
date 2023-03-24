// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"encoding/json"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"go.universe.tf/metallb/frr-tools/metrics/vtysh"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	bgpmetrics "go.universe.tf/metallb/internal/bgp/metrics"
)

type bfdPeerCounters struct {
	Peer                string `json:"peer"`
	ControlPacketInput  int    `json:"control-packet-input"`
	ControlPacketOutput int    `json:"control-packet-output"`
	EchoPacketInput     int    `json:"echo-packet-input"`
	EchoPacketOutput    int    `json:"echo-packet-output"`
	SessionUpEvents     int    `json:"session-up"`
	SessionDownEvents   int    `json:"session-down"`
	ZebraNotifications  int    `json:"zebra-notifications"`
}

const subsystem = "bfd"

var (
	bfdSessionUpDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, bgpmetrics.SessionUp.Name),
		"BFD session state (1 is up, 0 is down)",
		labels,
		nil,
	)

	controlPacketInputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "control_packet_input"),
		"Number of received BFD control packets",
		labels,
		nil,
	)

	controlPacketOutputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "control_packet_output"),
		"Number of sent BFD control packets",
		labels,
		nil,
	)

	echoPacketInputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "echo_packet_input"),
		"Number of received BFD echo packets",
		labels,
		nil,
	)

	echoPacketOutputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "echo_packet_output"),
		"Number of sent BFD echo packets",
		labels,
		nil,
	)

	sessionUpEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "session_up_events"),
		"Number of BFD session up events",
		labels,
		nil,
	)

	sessionDownEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "session_down_events"),
		"Number of BFD session down events",
		labels,
		nil,
	)

	zebraNotificationsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpmetrics.Namespace, subsystem, "zebra_notifications"),
		"Number of BFD zebra notifications",
		labels,
		nil,
	)
)

type bfd struct {
	Log    log.Logger
	frrCli vtysh.Cli
}

func NewBFD(l log.Logger) prometheus.Collector {
	log := log.With(l, "collector", subsystem)
	return &bfd{Log: log, frrCli: vtysh.Run}
}

func mockNewBFD(l log.Logger) *bfd {
	log := log.With(l, "collector", subsystem)
	return &bfd{Log: log, frrCli: vtysh.Run}
}

func (c *bfd) Describe(ch chan<- *prometheus.Desc) {
	ch <- bfdSessionUpDesc
	ch <- controlPacketInputDesc
	ch <- controlPacketOutputDesc
	ch <- echoPacketInputDesc
	ch <- echoPacketOutputDesc
	ch <- sessionUpEventsDesc
	ch <- sessionDownEventsDesc
	ch <- zebraNotificationsDesc
}

func (c *bfd) Collect(ch chan<- prometheus.Metric) {
	peers, err := getBFDPeers(c.frrCli)
	if err != nil {
		level.Error(c.Log).Log("error", err, "msg", "failed to fetch BFD peers from FRR")
		return
	}

	updatePeersMetrics(ch, peers)

	peersCounters, err := getBFDPeersCounters(c.frrCli)
	if err != nil {
		level.Error(c.Log).Log("error", err, "msg", "failed to fetch BFD peers counters from FRR")
		return
	}

	updatePeersCountersMetrics(ch, peersCounters)
}

func updatePeersMetrics(ch chan<- prometheus.Metric, peersPerVRF map[string][]bgpfrr.BFDPeer) {
	for vrf, peers := range peersPerVRF {
		for _, p := range peers {
			sessionUp := 1
			if p.Status != "up" {
				sessionUp = 0
			}

			ch <- prometheus.MustNewConstMetric(bfdSessionUpDesc, prometheus.GaugeValue, float64(sessionUp), p.Peer, vrf)
		}
	}
}

func updatePeersCountersMetrics(ch chan<- prometheus.Metric, peersCountersPerVRF map[string][]bfdPeerCounters) {
	for vrf, peersCounters := range peersCountersPerVRF {
		for _, p := range peersCounters {
			ch <- prometheus.MustNewConstMetric(controlPacketInputDesc, prometheus.CounterValue, float64(p.ControlPacketInput), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(controlPacketOutputDesc, prometheus.CounterValue, float64(p.ControlPacketOutput), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(echoPacketInputDesc, prometheus.CounterValue, float64(p.EchoPacketInput), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(echoPacketOutputDesc, prometheus.CounterValue, float64(p.EchoPacketOutput), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(sessionUpEventsDesc, prometheus.CounterValue, float64(p.SessionUpEvents), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(sessionDownEventsDesc, prometheus.CounterValue, float64(p.SessionDownEvents), p.Peer, vrf)
			ch <- prometheus.MustNewConstMetric(zebraNotificationsDesc, prometheus.CounterValue, float64(p.ZebraNotifications), p.Peer, vrf)
		}
	}
}

func getBFDPeers(frrCli vtysh.Cli) (map[string][]bgpfrr.BFDPeer, error) {
	vrfs, err := vtysh.VRFs(frrCli)
	if err != nil {
		return nil, err
	}
	res := make(map[string][]bgpfrr.BFDPeer)
	for _, vrf := range vrfs {
		peersJSON, err := frrCli(fmt.Sprintf("show bfd vrf %s peers json", vrf))
		if err != nil {
			return nil, err
		}
		peers, err := bgpfrr.ParseBFDPeers(peersJSON)
		if err != nil {
			return nil, err
		}
		res[vrf] = peers
	}
	return res, nil
}

func getBFDPeersCounters(frrCli vtysh.Cli) (map[string][]bfdPeerCounters, error) {
	vrfs, err := vtysh.VRFs(frrCli)
	if err != nil {
		return nil, err
	}

	res := make(map[string][]bfdPeerCounters)
	for _, vrf := range vrfs {
		countersJSON, err := frrCli(fmt.Sprintf("show bfd vrf %s peers counters json", vrf))
		if err != nil {
			return nil, err
		}

		parseRes := []bfdPeerCounters{}
		err = json.Unmarshal([]byte(countersJSON), &parseRes)
		if err != nil {
			return nil, err
		}
		res[vrf] = parseRes
	}
	return res, nil
}
