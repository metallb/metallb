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

//go:generate descriptor-adapter --descriptor-name ARP --value-type *linux_l3.ARPEntry --import "github.com/ligato/vpp-agent/api/models/linux/l3" --output-dir "descriptor"
//go:generate descriptor-adapter --descriptor-name Route --value-type *linux_l3.Route --import "github.com/ligato/vpp-agent/api/models/linux/l3" --output-dir "descriptor"

package l3plugin

import (
	"github.com/ligato/cn-infra/infra"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"

	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin"
	"github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/linuxv2/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
)

const (
	// by default, at most 10 go routines will split the configured namespaces
	// to execute the Dump operation in parallel.
	defaultDumpGoRoutinesCnt = 10
)

// L3Plugin configures Linux routes and ARP entries using Netlink API.
type L3Plugin struct {
	Deps

	// From configuration file
	disabled bool

	// system handlers
	l3Handler linuxcalls.NetlinkAPI

	// descriptors
	arpDescriptor   *descriptor.ARPDescriptor
	routeDescriptor *descriptor.RouteDescriptor
}

// Deps lists dependencies of the interface p.
type Deps struct {
	infra.PluginDeps
	KVScheduler kvs.KVScheduler
	NsPlugin    nsplugin.API
	IfPlugin    ifplugin.API
}

// Config holds the l3plugin configuration.
type Config struct {
	Disabled          bool `json:"disabled"`
	DumpGoRoutinesCnt int  `json:"dump-go-routines-count"`
}

// Init initializes and registers descriptors for Linux ARPs and Routes.
func (p *L3Plugin) Init() error {
	// parse configuration file
	config, err := p.retrieveConfig()
	if err != nil {
		return err
	}
	p.Log.Debugf("Linux L3 plugin config: %+v", config)
	if config.Disabled {
		p.disabled = true
		p.Log.Infof("Disabling Linux L3 plugin")
		return nil
	}

	// init handlers
	p.l3Handler = linuxcalls.NewNetLinkHandler()

	// init & register descriptors
	arpDescriptor := adapter.NewARPDescriptor(descriptor.NewARPDescriptor(
		p.KVScheduler, p.IfPlugin, p.NsPlugin, p.l3Handler, p.Log, config.DumpGoRoutinesCnt).GetDescriptor())

	routeDescriptor := adapter.NewRouteDescriptor(descriptor.NewRouteDescriptor(
		p.KVScheduler, p.IfPlugin, p.NsPlugin, p.l3Handler, p.Log, config.DumpGoRoutinesCnt).GetDescriptor())

	p.Deps.KVScheduler.RegisterKVDescriptor(arpDescriptor)
	p.Deps.KVScheduler.RegisterKVDescriptor(routeDescriptor)

	return nil
}

// Close does nothing here.
func (p *L3Plugin) Close() error {
	return nil
}

// retrieveConfig loads L3Plugin configuration file.
func (p *L3Plugin) retrieveConfig() (*Config, error) {
	config := &Config{
		// default configuration
		DumpGoRoutinesCnt: defaultDumpGoRoutinesCnt,
	}
	found, err := p.Cfg.LoadValue(config)
	if !found {
		p.Log.Debug("Linux L3Plugin config not found")
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	return config, err
}
