package ndp

import "github.com/prometheus/client_golang/prometheus"

var stats = metrics{
	ndpIn: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "ndp",
		Name:      "requests_received",
		Help:      "Number of NDP requests received for owned IPs",
	}, []string{
		"ip",
	}),

	ndpOut: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "ndp",
		Name:      "responses_sent",
		Help:      "Number of NDP responses sent for owned IPs in response to requests",
	}, []string{
		"ip",
	}),

	ndpGratuitous: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "ndp",
		Name:      "gratuitous_sent",
		Help:      "Number of gratuitous NDP packets sent for owned IPs as a result of failovers",
	}, []string{
		"ip",
	}),
}

type metrics struct {
	ndpIn         *prometheus.CounterVec
	ndpOut        *prometheus.CounterVec
	ndpGratuitous *prometheus.CounterVec
}

func init() {
	prometheus.MustRegister(stats.ndpIn)
	prometheus.MustRegister(stats.ndpOut)
	prometheus.MustRegister(stats.ndpGratuitous)
}

func (m *metrics) GotRequest(addr string) {
	m.ndpIn.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentResponse(addr string) {
	m.ndpOut.WithLabelValues(addr).Add(1)
}

func (m *metrics) SentGratuitous(addr string) {
	m.ndpGratuitous.WithLabelValues(addr).Add(1)
}
