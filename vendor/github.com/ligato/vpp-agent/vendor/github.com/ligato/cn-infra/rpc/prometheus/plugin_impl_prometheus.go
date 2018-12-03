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
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/unrolled/render"
)

// DefaultRegistry default Prometheus metrics URL
const DefaultRegistry = "/metrics"

var (
	// ErrPathInvalidFormat is returned if the path doesn't start with slash
	ErrPathInvalidFormat = errors.New("path is invalid, it must start with '/' character")
	// ErrPathAlreadyRegistry is returned on attempt to register a path used by a registry
	ErrPathAlreadyRegistry = errors.New("registry with the path is already registered")
	// ErrRegistryNotFound is returned on attempt to use register that has not been created
	ErrRegistryNotFound = errors.New("registry was not found")
)

// Plugin struct holds all plugin-related data.
type Plugin struct {
	Deps

	sync.Mutex
	// regs is a map of URL path(symbolic names) to registries. Registries group metrics and can be exposed at different urls.
	regs map[string]*registry
}

// Deps lists dependencies of the plugin.
type Deps struct {
	infra.PluginName
	Log logging.PluginLogger
	// HTTP server used to expose metrics
	HTTP rest.HTTPHandlers // inject
}

type registry struct {
	prometheus.Gatherer
	prometheus.Registerer
	// httpOpts applied when exposing registry using http
	httpOpts promhttp.HandlerOpts
}

// Init initializes the internal structures
func (p *Plugin) Init() error {
	p.regs = map[string]*registry{}

	// add default registry
	p.regs[DefaultRegistry] = &registry{
		Gatherer:   prometheus.DefaultGatherer,
		Registerer: prometheus.DefaultRegisterer,
	}

	return nil
}

// AfterInit registers HTTP handlers.
func (p *Plugin) AfterInit() error {
	if p.HTTP != nil {
		p.Lock()
		defer p.Unlock()
		for path, reg := range p.regs {
			p.HTTP.RegisterHTTPHandler(path, p.createHandlerHandler(reg.Gatherer, reg.httpOpts), "GET")
			p.Log.Infof("Serving %s on port %d", path, p.HTTP.GetPort())

		}
	} else {
		p.Log.Info("Unable to register Prometheus metrics handlers, HTTP is nil")
	}

	return nil
}

// Close cleans up the allocated resources.
func (p *Plugin) Close() error {
	return nil
}

// NewRegistry creates new registry exposed at defined URL path (must begin with '/' character), path is used to reference
// registry while adding new metrics into registry, opts adjust the behavior of exposed registry. Must be called before
// AfterInit phase of the Prometheus plugin. An attempt to create  a registry with path that is already used
// by different registry returns an error.
func (p *Plugin) NewRegistry(path string, opts promhttp.HandlerOpts) error {
	p.Lock()
	defer p.Unlock()

	if !strings.HasPrefix(path, "/") {
		p.Log.WithField("path", path).Error(ErrPathInvalidFormat)
		return ErrPathInvalidFormat
	}
	if _, found := p.regs[path]; found {
		p.Log.WithField("path", path).Error(ErrPathAlreadyRegistry)
		return ErrPathAlreadyRegistry
	}
	newReg := prometheus.NewRegistry()
	p.regs[path] = &registry{
		Registerer: newReg,
		Gatherer:   newReg,
		httpOpts:   opts,
	}
	return nil
}

// Register registers prometheus metric to a specified registry. In order to add metrics
// to default registry use prometheus.DefaultRegistry const.
func (p *Plugin) Register(registryPath string, collector prometheus.Collector) error {
	p.Lock()
	defer p.Unlock()

	reg, found := p.regs[registryPath]
	if !found {
		p.Log.WithField("path", registryPath).Error(ErrRegistryNotFound)
		return ErrRegistryNotFound
	}
	return reg.Register(collector)
}

// Unregister unregisters the given metric. The function
// returns whether a Collector was unregistered.
func (p *Plugin) Unregister(registryPath string, collector prometheus.Collector) bool {
	p.Lock()
	defer p.Unlock()

	reg, found := p.regs[registryPath]
	if !found {
		return false
	}
	return reg.Unregister(collector)
}

// RegisterGaugeFunc registers custom gauge with specific valueFunc to report status when invoked.
// This method simplifies using of Register for common use case. If you want create metric different from
// GagugeFunc or you're adding a metric that will be unregister later on, use generic Register method instead.
// RegistryPath identifies the registry where gauge is added.
func (p *Plugin) RegisterGaugeFunc(registryPath string, namespace string, subsystem string, name string, help string,
	labels prometheus.Labels, valueFunc func() float64) error {

	p.Lock()
	defer p.Unlock()

	reg, found := p.regs[registryPath]
	if !found {
		p.Log.WithField("path", registryPath).Error(ErrRegistryNotFound)
		return ErrRegistryNotFound
	}

	gaugeName := name
	if subsystem != "" {
		gaugeName = subsystem + "_" + gaugeName
	}
	if namespace != "" {
		gaugeName = namespace + "_" + gaugeName
	}

	err := reg.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		},
		valueFunc,
	))
	if err != nil {
		p.Log.Errorf("GaugeFunc('%s') registration failed: %s", gaugeName, err)
		return err
	}
	p.Log.Infof("GaugeFunc('%s') registered.", gaugeName)
	return nil
}

func (p *Plugin) createHandlerHandler(gatherer prometheus.Gatherer, opts promhttp.HandlerOpts) func(formatter *render.Render) http.HandlerFunc {
	return func(formatter *render.Render) http.HandlerFunc {
		return promhttp.HandlerFor(gatherer, opts).ServeHTTP
	}
}
