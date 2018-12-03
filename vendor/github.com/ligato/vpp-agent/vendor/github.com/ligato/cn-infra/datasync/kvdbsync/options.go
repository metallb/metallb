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

package kvdbsync

import (
	"fmt"

	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
)

// NewPlugin creates a new Plugin with the provided Options.
func NewPlugin(opts ...Option) *Plugin {
	p := &Plugin{}

	p.PluginName = "kvdb"
	p.ServiceLabel = &servicelabel.DefaultPlugin
	p.ResyncOrch = &resync.DefaultPlugin

	for _, o := range opts {
		o(p)
	}

	name := p.String()
	if p.Deps.KvPlugin != nil {
		name = fmt.Sprintf("%s-%s", name, p.Deps.KvPlugin.String())
	}
	p.Deps.SetName(name + "-datasync")

	if p.Deps.Log == nil {
		p.Deps.Log = logging.ForPlugin(p.String())
	}

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

// UseKV returns Option that sets KvPlugin dependency.
func UseKV(kv keyval.KvProtoPlugin) Option {
	return func(p *Plugin) {
		p.KvPlugin = kv
	}
}
