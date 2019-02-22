// Copyright (c) 2018 Cisco and/or its affiliates.
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

package puntplugin

import (
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	"github.com/ligato/vpp-agent/plugins/kvscheduler"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin"
)

// DefaultPlugin is a default instance of punt plugin.
var DefaultPlugin = *NewPlugin()

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *PuntPlugin {
	p := &PuntPlugin{}

	p.PluginName = "vpp-punt-plugin"
	p.KVScheduler = &kvscheduler.DefaultPlugin
	p.GoVppmux = &govppmux.DefaultPlugin
	p.IfPlugin = &ifplugin.DefaultPlugin

	for _, o := range opts {
		o(p)
	}

	if p.Log == nil {
		p.Log = logging.ForPlugin(p.String())
	}

	return p
}

// Option is a function that can be used in NewPlugin to customize Plugin.
type Option func(puntPlugin *PuntPlugin)

// UseDeps returns Option that can inject custom dependencies.
func UseDeps(f func(deps *Deps)) Option {
	return func(p *PuntPlugin) {
		f(&p.Deps)
	}
}
