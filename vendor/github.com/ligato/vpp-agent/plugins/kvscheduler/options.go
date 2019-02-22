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

package kvscheduler

import (
	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/rpc/rest"
)

// DefaultPlugin is a default instance of Plugin.
var DefaultPlugin = *NewPlugin()

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *Scheduler {
	p := &Scheduler{}

	p.PluginName = "kvscheduler"
	p.HTTPHandlers = &rest.DefaultPlugin

	for _, o := range opts {
		o(p)
	}

	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}
	if p.Deps.Cfg == nil {
		p.Deps.Cfg = config.ForPlugin(p.String())
	}

	return p
}

// Option is a function that can be used in NewPlugin to customize Plugin.
type Option func(*Scheduler)

// UseDeps returns Option that can inject custom dependencies.
func UseDeps(cb func(*Deps)) Option {
	return func(p *Scheduler) {
		cb(&p.Deps)
	}
}
