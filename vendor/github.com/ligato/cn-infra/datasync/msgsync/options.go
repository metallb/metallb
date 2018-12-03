//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package msgsync

import (
	"github.com/ligato/cn-infra/messaging"
)

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{}

	p.PluginName = "msgsync"

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
		p.Config = conf
	}
}

// UseMessaging returns Option that sets Messaging.
func UseMessaging(m messaging.Mux) Option {
	return func(p *Plugin) {
		p.Deps.Messaging = m
	}
}
