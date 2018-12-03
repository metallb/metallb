// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// API allows to create expose metrics using prometheus metrics.
type API interface {
	// NewRegistry creates new registry exposed at defined URL path (must begin with '/' character), path is used to reference
	// registry while adding new metrics into registry, opts adjust the behavior of exposed registry. Must be called before
	// AfterInit phase of the Prometheus plugin. An attempt to create  a registry with path that is already used
	// by different registry returns an error.
	NewRegistry(path string, opts promhttp.HandlerOpts) error

	// Register registers prometheus metric (e.g.: created by prometheus.NewGaugeVec, prometheus.NewHistogram,...)
	// to a specified registry
	Register(registryPath string, collector prometheus.Collector) error

	// Unregister unregisters the given metric. The function
	// returns whether a Collector was unregistered.
	Unregister(registryPath string, collector prometheus.Collector) bool

	// RegisterGauge registers custom gauge with specific valueFunc to report status when invoked. RegistryPath identifies
	// the registry. The aim of this method is to simply common use case - adding Gauge with value func.
	RegisterGaugeFunc(registryPath string, namespace string, subsystem string, name string, help string,
		labels prometheus.Labels, valueFunc func() float64) error
}
