// SPDX-License-Identifier:Apache-2.0

package collector

import (
	"encoding/json"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	bgpstats "go.universe.tf/metallb/internal/bgp/native"
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
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, bgpstats.SessionUp.Name),
		"BFD session state (1 is up, 0 is down)",
		bgpstats.Labels,
		nil,
	)

	controlPacketInputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "control_packet_input"),
		"Number of received BFD control packets",
		bgpstats.Labels,
		nil,
	)

	controlPacketOutputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "control_packet_output"),
		"Number of sent BFD control packets",
		bgpstats.Labels,
		nil,
	)

	echoPacketInputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "echo_packet_input"),
		"Number of received BFD echo packets",
		bgpstats.Labels,
		nil,
	)

	echoPacketOutputDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "echo_packet_output"),
		"Number of sent BFD echo packets",
		bgpstats.Labels,
		nil,
	)

	sessionUpEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "session_up_events"),
		"Number of BFD session up events",
		bgpstats.Labels,
		nil,
	)

	sessionDownEventsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "session_down_events"),
		"Number of BFD session down events",
		bgpstats.Labels,
		nil,
	)

	zebraNotificationsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(bgpstats.Namespace, subsystem, "zebra_notifications"),
		"Number of BFD zebra notifications",
		bgpstats.Labels,
		nil,
	)
)

type bfd struct {
	Log    log.Logger
	frrCli func(args string) (string, error)
}

func NewBFD(l log.Logger) *bfd {
	log := log.With(l, "collector", subsystem)
	return &bfd{Log: log, frrCli: runVtysh}
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

func updatePeersMetrics(ch chan<- prometheus.Metric, peers map[string]bgpfrr.BFDPeer) {
	for _, p := range peers {
		sessionUp := 1
		if p.Status != "up" {
			sessionUp = 0
		}

		ch <- prometheus.MustNewConstMetric(bfdSessionUpDesc, prometheus.GaugeValue, float64(sessionUp), p.Peer)
	}
}

func updatePeersCountersMetrics(ch chan<- prometheus.Metric, peersCounters []bfdPeerCounters) {
	for _, p := range peersCounters {
		ch <- prometheus.MustNewConstMetric(controlPacketInputDesc, prometheus.CounterValue, float64(p.ControlPacketInput), p.Peer)
		ch <- prometheus.MustNewConstMetric(controlPacketOutputDesc, prometheus.CounterValue, float64(p.ControlPacketOutput), p.Peer)
		ch <- prometheus.MustNewConstMetric(echoPacketInputDesc, prometheus.CounterValue, float64(p.EchoPacketInput), p.Peer)
		ch <- prometheus.MustNewConstMetric(echoPacketOutputDesc, prometheus.CounterValue, float64(p.EchoPacketOutput), p.Peer)
		ch <- prometheus.MustNewConstMetric(sessionUpEventsDesc, prometheus.CounterValue, float64(p.SessionUpEvents), p.Peer)
		ch <- prometheus.MustNewConstMetric(sessionDownEventsDesc, prometheus.CounterValue, float64(p.SessionDownEvents), p.Peer)
		ch <- prometheus.MustNewConstMetric(zebraNotificationsDesc, prometheus.CounterValue, float64(p.ZebraNotifications), p.Peer)
	}
}

func getBFDPeers(frrCli func(args string) (string, error)) (map[string]bgpfrr.BFDPeer, error) {
	res, err := frrCli("show bfd peers json")
	if err != nil {
		return nil, err
	}

	return bgpfrr.ParseBFDPeers(res)
}

func getBFDPeersCounters(frrCli func(args string) (string, error)) ([]bfdPeerCounters, error) {
	res, err := frrCli("show bfd peers counters json")
	if err != nil {
		return nil, err
	}

	parseRes := []bfdPeerCounters{}
	err = json.Unmarshal([]byte(res), &parseRes)
	if err != nil {
		return nil, err
	}

	return parseRes, nil
}
