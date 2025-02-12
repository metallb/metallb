// SPDX-License-Identifier:Apache-2.0

package allocator

import "github.com/prometheus/client_golang/prometheus"

var stats = struct {
	poolCapacity     *prometheus.GaugeVec
	ipv4PoolCapacity *prometheus.GaugeVec
	ipv6PoolCapacity *prometheus.GaugeVec
	poolActive       *prometheus.GaugeVec
	ipv4PoolActive   *prometheus.GaugeVec
	ipv6PoolActive   *prometheus.GaugeVec
	poolAllocated    *prometheus.GaugeVec
}{
	poolCapacity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "addresses_total",
		Help:      "Number of usable IP addresses, per pool",
	}, []string{
		"pool",
	}),
	ipv4PoolCapacity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "ipv4_addresses_total",
		Help:      "Number of usable IPV4 addresses, per pool",
	}, []string{
		"pool",
	}),
	ipv6PoolCapacity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "ipv6_addresses_total",
		Help:      "Number of usable IPV6 addresses, per pool",
	}, []string{
		"pool",
	}),
	poolActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "addresses_in_use_total",
		Help:      "Number of IP addresses in use, per pool",
	}, []string{
		"pool",
	}),
	ipv4PoolActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "ipv4_addresses_in_use_total",
		Help:      "Number of IPV4 addresses in use, per pool",
	}, []string{
		"pool",
	}),
	ipv6PoolActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "ipv6_addresses_in_use_total",
		Help:      "Number of IPV6 addresses in use, per pool",
	}, []string{
		"pool",
	}),
	poolAllocated: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "metallb",
		Subsystem: "allocator",
		Name:      "services_allocated_total",
		Help:      "Number of services allocated, per pool",
	}, []string{
		"pool",
	}),
}

func deleteStatsFor(pool string) {
	stats.poolCapacity.DeleteLabelValues(pool)
	stats.ipv4PoolCapacity.DeleteLabelValues(pool)
	stats.ipv6PoolCapacity.DeleteLabelValues(pool)
	stats.poolActive.DeleteLabelValues(pool)
	stats.poolAllocated.DeleteLabelValues(pool)
	stats.ipv4PoolActive.DeleteLabelValues(pool)
	stats.ipv6PoolActive.DeleteLabelValues(pool)
}

func init() {
	prometheus.MustRegister(stats.poolCapacity)
	prometheus.MustRegister(stats.ipv4PoolCapacity)
	prometheus.MustRegister(stats.ipv6PoolCapacity)
	prometheus.MustRegister(stats.poolActive)
	prometheus.MustRegister(stats.ipv4PoolActive)
	prometheus.MustRegister(stats.ipv6PoolActive)
	prometheus.MustRegister(stats.poolAllocated)
}
