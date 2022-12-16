// SPDX-License-Identifier:Apache-2.0

package native

import (
	"github.com/prometheus/client_golang/prometheus"
	bgpmetrics "go.universe.tf/metallb/internal/bgp/metrics"
)

var labels = []string{"peer"}

var stats = metrics{
	sessionUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bgpmetrics.Namespace,
		Subsystem: bgpmetrics.Subsystem,
		Name:      bgpmetrics.SessionUp.Name,
		Help:      bgpmetrics.SessionUp.Help,
	}, labels),

	updatesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: bgpmetrics.Namespace,
		Subsystem: bgpmetrics.Subsystem,
		Name:      bgpmetrics.UpdatesSent.Name,
		Help:      bgpmetrics.UpdatesSent.Help,
	}, labels),

	prefixes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bgpmetrics.Namespace,
		Subsystem: bgpmetrics.Subsystem,
		Name:      bgpmetrics.Prefixes.Name,
		Help:      bgpmetrics.Prefixes.Help,
	}, labels),

	pendingPrefixes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bgpmetrics.Namespace,
		Subsystem: bgpmetrics.Subsystem,
		Name:      "pending_prefixes_total",
		Help:      "Number of prefixes that should be advertised on the BGP session",
	}, labels),
}

type metrics struct {
	sessionUp       *prometheus.GaugeVec
	updatesSent     *prometheus.CounterVec
	prefixes        *prometheus.GaugeVec
	pendingPrefixes *prometheus.GaugeVec
}

func init() {
	prometheus.MustRegister(stats.sessionUp)
	prometheus.MustRegister(stats.updatesSent)
	prometheus.MustRegister(stats.prefixes)
	prometheus.MustRegister(stats.pendingPrefixes)
}

func (m *metrics) NewSession(addr string) {
	m.sessionUp.WithLabelValues(addr).Set(0)
	m.prefixes.WithLabelValues(addr).Set(0)
	m.pendingPrefixes.WithLabelValues(addr).Set(0)
	m.updatesSent.WithLabelValues(addr).Add(0) // just creates the metric
}

func (m *metrics) DeleteSession(addr string) {
	m.sessionUp.DeleteLabelValues(addr)
	m.prefixes.DeleteLabelValues(addr)
	m.pendingPrefixes.DeleteLabelValues(addr)
	m.updatesSent.DeleteLabelValues(addr)
}

func (m *metrics) SessionUp(addr string) {
	m.sessionUp.WithLabelValues(addr).Set(1)
	m.prefixes.WithLabelValues(addr).Set(0)
}

func (m *metrics) SessionDown(addr string) {
	m.sessionUp.WithLabelValues(addr).Set(0)
	m.prefixes.WithLabelValues(addr).Set(0)
}

func (m *metrics) UpdateSent(addr string) {
	m.updatesSent.WithLabelValues(addr).Inc()
}

func (m *metrics) PendingPrefixes(addr string, n int) {
	m.pendingPrefixes.WithLabelValues(addr).Set(float64(n))
}

func (m *metrics) AdvertisedPrefixes(addr string, n int) {
	m.prefixes.WithLabelValues(addr).Set(float64(n))
	m.pendingPrefixes.WithLabelValues(addr).Set(float64(n))
}
