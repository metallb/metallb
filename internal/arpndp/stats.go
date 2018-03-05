package arp

import "github.com/prometheus/client_golang/prometheus"

var stats = metrics{
	arpIn: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arp",
		Name:      "requests_received",
		Help:      "Number of ARP requests received for owned IPs",
	}, []string{
		"ip",
	}),

	arpOut: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arp",
		Name:      "responses_sent",
		Help:      "Number of ARP responses sent for owned IPs in response to requests",
	}, []string{
		"ip",
	}),

	arpGratuitous: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arp",
		Name:      "gratuitous_sent",
		Help:      "Number of gratuitous ARP packets sent for owned IPs as a result of failovers",
	}, []string{
		"ip",
	}),
}

type metrics struct {
	arpIn         *prometheus.CounterVec
	arpOut        *prometheus.CounterVec
	arpGratuitous *prometheus.CounterVec
}

func init() {
	prometheus.MustRegister(stats.arpIn)
	prometheus.MustRegister(stats.arpOut)
	prometheus.MustRegister(stats.arpGratuitous)
}

func (m *metrics) GotRequest(addr string) {
	m.arpIn.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentResponse(addr string) {
	m.arpOut.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentGratuitous(addr string) {
	m.arpGratuitous.WithLabelValues(addr).Add(1)
}
