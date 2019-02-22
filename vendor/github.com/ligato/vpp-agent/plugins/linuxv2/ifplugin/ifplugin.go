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

//go:generate descriptor-adapter --descriptor-name Interface  --value-type *linux_interfaces.Interface --meta-type *ifaceidx.LinuxIfMetadata --import "github.com/ligato/vpp-agent/api/models/linux/interfaces" --import "ifaceidx" --output-dir "descriptor"

package ifplugin

import (
	"github.com/pkg/errors"

	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/servicelabel"

	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/descriptor"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/descriptor/adapter"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linuxv2/ifplugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linuxv2/nsplugin"
)

const (
	// by default, at most 10 go routines will split the configured namespaces
	// to execute the Dump operation in parallel.
	defaultDumpGoRoutinesCnt = 10
)

// IfPlugin configures Linux VETH and TAP interfaces using Netlink API.
type IfPlugin struct {
	Deps

	// From configuration file
	disabled bool

	// system handlers
	ifHandler linuxcalls.NetlinkAPI

	// descriptors
	ifDescriptor *descriptor.InterfaceDescriptor
	ifWatcher    *descriptor.InterfaceWatcher

	// index map
	ifIndex ifaceidx.LinuxIfMetadataIndex
}

// Deps lists dependencies of the interface plugin.
type Deps struct {
	infra.PluginDeps
	ServiceLabel servicelabel.ReaderAPI
	KVScheduler  kvs.KVScheduler
	NsPlugin     nsplugin.API
	VppIfPlugin  descriptor.VPPIfPluginAPI /* mandatory if TAP_TO_VPP interfaces are used */
}

// Config holds the ifplugin configuration.
type Config struct {
	Disabled          bool `json:"disabled"`
	DumpGoRoutinesCnt int  `json:"dump-go-routines-count"`
}

// Init registers interface-related descriptors and starts watching of the default
// network namespace for interface changes.
func (p *IfPlugin) Init() error {
	// parse configuration file
	config, err := p.retrieveConfig()
	if err != nil {
		return err
	}
	p.Log.Debugf("Linux interface plugin config: %+v", config)
	if config.Disabled {
		p.disabled = true
		p.Log.Infof("Disabling Linux Interface plugin")
		return nil
	}

	// init handlers
	p.ifHandler = linuxcalls.NewNetLinkHandler()

	// init & register descriptors
	p.ifDescriptor = descriptor.NewInterfaceDescriptor(p.KVScheduler,
		p.ServiceLabel, p.NsPlugin, p.VppIfPlugin, p.ifHandler, p.Log, config.DumpGoRoutinesCnt)
	ifDescriptor := adapter.NewInterfaceDescriptor(p.ifDescriptor.GetDescriptor())
	p.Deps.KVScheduler.RegisterKVDescriptor(ifDescriptor)

	p.ifWatcher = descriptor.NewInterfaceWatcher(p.KVScheduler, p.ifHandler, p.Log)
	p.Deps.KVScheduler.RegisterKVDescriptor(p.ifWatcher.GetDescriptor())

	// obtain read-only reference to index map
	var withIndex bool
	metadataMap := p.Deps.KVScheduler.GetMetadataMap(ifDescriptor.Name)
	p.ifIndex, withIndex = metadataMap.(ifaceidx.LinuxIfMetadataIndex)
	if !withIndex {
		return errors.New("missing index with interface metadata")
	}

	// start interface watching
	if err = p.ifWatcher.StartWatching(); err != nil {
		return err
	}

	return nil
}

// Close stops watching of the default network namespace.
func (p *IfPlugin) Close() error {
	if p.disabled {
		return nil
	}
	p.ifWatcher.StopWatching()
	return nil
}

// GetInterfaceIndex gives read-only access to map with metadata of all configured
// linux interfaces.
func (p *IfPlugin) GetInterfaceIndex() ifaceidx.LinuxIfMetadataIndex {
	return p.ifIndex
}

// retrieveConfig loads IfPlugin configuration file.
func (p *IfPlugin) retrieveConfig() (*Config, error) {
	config := &Config{
		// default configuration
		DumpGoRoutinesCnt: defaultDumpGoRoutinesCnt,
	}
	found, err := p.Cfg.LoadValue(config)
	if !found {
		p.Log.Debug("Linux IfPlugin config not found")
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	p.Log.Debug("Linux IfPlugin config found")
	return config, err
}
