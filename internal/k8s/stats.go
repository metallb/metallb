package k8s

import "github.com/prometheus/client_golang/prometheus"

var (
	updates = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "k8s_client",
		Name:      "updates_total",
		Help:      "Number of k8s object updates that have been processed.",
	})

	updateErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "metallb",
		Subsystem: "k8s_client",
		Name:      "update_errors_total",
		Help:      "Number of k8s object updates that failed for some reason.",
	})
)

func init() {
	prometheus.MustRegister(updates)
	prometheus.MustRegister(updateErrors)
}
