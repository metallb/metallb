package logmanager

import (
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/rpc/rest"
	"github.com/ligato/cn-infra/servicelabel"
)

// DefaultPlugin is a default instance of Plugin.
var DefaultPlugin = *NewPlugin()

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{}

	p.PluginName = "logs"
	p.LogRegistry = logging.DefaultRegistry
	p.HTTP = &rest.DefaultPlugin
	p.ServiceLabel = &servicelabel.DefaultPlugin

	for _, o := range opts {
		o(p)
	}

	p.PluginDeps.Setup()

	return p
}

// Option is a function that can be used in NewPlugin to customize Plugin.
type Option func(*Plugin)

// UseDeps returns Option that can inject custom dependencies.
func UseDeps(cb func(*Deps)) Option {
	return func(p *Plugin) {
		cb(&p.Deps)
	}
}

// UseConf returns Option which injects a particular configuration.
func UseConf(conf Config) Option {
	return func(p *Plugin) {
		p.Config = &conf
	}
}
