package arp

import "github.com/prometheus/client_golang/prometheus"

var stats = metrics{
	in: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arpndp",
		Name:      "requests_received",
		Help:      "Number of ARP/NDP requests received for owned IPs",
	}, []string{
		"ip",
	}),

	out: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arpndp",
		Name:      "responses_sent",
		Help:      "Number of ARP/NDP responses sent for owned IPs in response to requests",
	}, []string{
		"ip",
	}),

	gratuitous: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "arpndp",
		Name:      "gratuitous_sent",
		Help:      "Number of gratuitous ARP/NDP packets sent for owned IPs as a result of failovers",
	}, []string{
		"ip",
	}),
}

type metrics struct {
	in         *prometheus.CounterVec
	out        *prometheus.CounterVec
	gratuitous *prometheus.CounterVec
}

func init() {
	prometheus.MustRegister(stats.in)
	prometheus.MustRegister(stats.out)
	prometheus.MustRegister(stats.gratuitous)
}

func (m *metrics) GotRequest(addr string) {
	m.in.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentResponse(addr string) {
	m.out.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentGratuitous(addr string) {
	m.gratuitous.WithLabelValues(addr).Add(1)
}
