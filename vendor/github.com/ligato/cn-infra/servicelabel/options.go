package servicelabel

// DefaultPlugin is a default instance of Plugin.
var DefaultPlugin = *NewPlugin()

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{}

	p.PluginName = "service-label"

	for _, o := range opts {
		o(p)
	}

	return p
}

// Option is a function that can be used in NewPlugin to customize Plugin.
type Option func(*Plugin)

// UseLabel sets microservice label to given string
func UseLabel(label string) Option {
	return func(p *Plugin) {
		p.MicroserviceLabel = label
	}
}
