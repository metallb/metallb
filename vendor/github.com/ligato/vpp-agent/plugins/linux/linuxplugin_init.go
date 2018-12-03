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

// Package linux implements the Linux plugin that handles management
// of Linux VETH interfaces.
package linux

import (
	"context"
	"sync"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	ifLinuxcalls "github.com/ligato/vpp-agent/plugins/linux/ifplugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"
	l3Linuxcalls "github.com/ligato/vpp-agent/plugins/linux/l3plugin/linuxcalls"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/ligato/vpp-agent/plugins/vpp"
	ifaceVPP "github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
)

// Plugin implements Plugin interface, therefore it can be loaded with other plugins.
type Plugin struct {
	Deps

	disabled bool

	// Configurators
	ifConfigurator      *ifplugin.LinuxInterfaceConfigurator
	ifLinuxStateUpdater *ifplugin.LinuxInterfaceStateUpdater
	arpConfigurator     *l3plugin.LinuxArpConfigurator
	routeConfigurator   *l3plugin.LinuxRouteConfigurator

	// Shared indexes
	ifIndexes    ifaceidx.LinuxIfIndexRW
	vppIfIndexes ifaceVPP.SwIfIndex

	// Interface/namespace handling
	ifHandler ifLinuxcalls.NetlinkAPI
	nsHandler nsplugin.NamespaceAPI

	// Channels (watch, notification, ...) which should be closed
	ifIndexesWatchChan    chan ifaceidx.LinuxIfIndexDto
	vppIfIndexesWatchChan chan ifaceVPP.SwIfIdxDto
	ifLinuxNotifChan      chan *ifplugin.LinuxInterfaceStateNotification
	ifMicroserviceNotif   chan *nsplugin.MicroserviceEvent
	resyncChan            chan datasync.ResyncEvent
	changeChan            chan datasync.ChangeEvent // TODO dedicated type abstracted from ETCD
	msChan                chan *nsplugin.MicroserviceCtx

	// Registrations
	watchDataReg datasync.WatchRegistration

	// Common
	cancel context.CancelFunc // Cancel can be used to cancel all goroutines and their jobs inside of the plugin.
	wg     sync.WaitGroup     // Wait group allows to wait until all goroutines of the plugin have finished.
}

// Deps groups injected dependencies of plugin
// so that they do not mix with other plugin fields.
type Deps struct {
	infra.PluginDeps
	StatusCheck  statuscheck.PluginStatusWriter
	ServiceLabel servicelabel.ReaderAPI

	Watcher          datasync.KeyValProtoWatcher // injected
	VPP              *vpp.Plugin
	WatchEventsMutex *sync.Mutex
}

// Config holds the linuxplugin configuration.
type Config struct {
	Stopwatch bool `json:"stopwatch"`
	Disabled  bool `json:"disabled"`
}

// GetLinuxIfIndexes gives access to mapping of logical names (used in ETCD configuration)
// interface indexes.
func (plugin *Plugin) GetLinuxIfIndexes() ifaceidx.LinuxIfIndex {
	return plugin.ifIndexes
}

// GetLinuxARPIndexes gives access to mapping of logical names (used in ETCD configuration) to corresponding Linux
// ARP entry indexes.
func (plugin *Plugin) GetLinuxARPIndexes() l3idx.LinuxARPIndex {
	return plugin.arpConfigurator.GetArpIndexes()
}

// GetLinuxRouteIndexes gives access to mapping of logical names (used in ETCD configuration) to corresponding Linux
// route indexes.
func (plugin *Plugin) GetLinuxRouteIndexes() l3idx.LinuxRouteIndex {
	return plugin.routeConfigurator.GetRouteIndexes()
}

// GetNamespaceHandler gives access to namespace API which allows plugins to manipulate with linux namespaces
func (plugin *Plugin) GetNamespaceHandler() nsplugin.NamespaceAPI {
	return plugin.nsHandler
}

// InjectVppIfIndexes injects VPP interfaces mapping into Linux plugin
func (plugin *Plugin) InjectVppIfIndexes(indexes ifaceVPP.SwIfIndex) {
	plugin.vppIfIndexes = indexes
	plugin.vppIfIndexes.WatchNameToIdx(plugin.String(), plugin.vppIfIndexesWatchChan)
}

// Init gets handlers for ETCD and Kafka and delegates them to ifConfigurator.
func (plugin *Plugin) Init() error {
	plugin.Log.Debug("Initializing Linux plugin")

	config, err := plugin.retrieveLinuxConfig()
	if err != nil {
		return err
	}
	if config != nil {
		if config.Disabled {
			plugin.disabled = true
			plugin.Log.Infof("Disabling Linux plugin")
			return nil
		}
	} else {
		plugin.Log.Infof("stopwatch disabled for %v", plugin.PluginName)
	}

	plugin.resyncChan = make(chan datasync.ResyncEvent)
	plugin.changeChan = make(chan datasync.ChangeEvent)
	plugin.msChan = make(chan *nsplugin.MicroserviceCtx)
	plugin.ifMicroserviceNotif = make(chan *nsplugin.MicroserviceEvent, 100)
	plugin.ifIndexesWatchChan = make(chan ifaceidx.LinuxIfIndexDto, 100)
	plugin.vppIfIndexesWatchChan = make(chan ifaceVPP.SwIfIdxDto, 100)

	// Create plugin context and save cancel function into the plugin handle.
	var ctx context.Context
	ctx, plugin.cancel = context.WithCancel(context.Background())

	// Run event handler go routines
	go plugin.watchEvents(ctx)

	err = plugin.initNs()
	if err != nil {
		return err
	}

	err = plugin.initIF(ctx)
	if err != nil {
		return err
	}

	err = plugin.initL3()
	if err != nil {
		return err
	}

	return plugin.subscribeWatcher()
}

// Close cleans up the resources.
func (plugin *Plugin) Close() error {
	if plugin.disabled {
		return nil
	}

	plugin.cancel()
	plugin.wg.Wait()

	return safeclose.Close(
		// Configurators
		plugin.ifConfigurator, plugin.arpConfigurator, plugin.routeConfigurator,
		// Status updater
		plugin.ifLinuxStateUpdater,
		// Channels
		plugin.ifIndexesWatchChan, plugin.ifMicroserviceNotif, plugin.changeChan, plugin.resyncChan,
		plugin.msChan,
		// Registrations
		plugin.watchDataReg,
	)
}

// Initialize namespace handler plugin
func (plugin *Plugin) initNs() error {
	plugin.Log.Infof("Init Linux namespace handler")

	namespaceHandler := &nsplugin.NsHandler{}
	plugin.nsHandler = namespaceHandler
	return namespaceHandler.Init(plugin.Log, nsplugin.NewSystemHandler(), plugin.msChan,
		plugin.ifMicroserviceNotif)
}

// Initialize linux interface plugin
func (plugin *Plugin) initIF(ctx context.Context) error {
	plugin.Log.Infof("Init Linux interface plugin")

	// Init shared interface index mapping
	plugin.ifIndexes = ifaceidx.NewLinuxIfIndex(nametoidx.NewNameToIdx(plugin.Log, "linux_if_indexes", nil))

	// Shared interface linux calls handler
	plugin.ifHandler = ifLinuxcalls.NewNetLinkHandler(plugin.nsHandler, plugin.ifIndexes, plugin.Log)

	// Linux interface configurator
	plugin.ifLinuxNotifChan = make(chan *ifplugin.LinuxInterfaceStateNotification, 10)
	plugin.ifConfigurator = &ifplugin.LinuxInterfaceConfigurator{}
	if err := plugin.ifConfigurator.Init(plugin.Log, plugin.ifHandler, plugin.nsHandler, nsplugin.NewSystemHandler(),
		plugin.ifIndexes, plugin.ifMicroserviceNotif, plugin.ifLinuxNotifChan); err != nil {
		return plugin.ifConfigurator.LogError(err)
	}
	plugin.ifLinuxStateUpdater = &ifplugin.LinuxInterfaceStateUpdater{}
	if err := plugin.ifLinuxStateUpdater.Init(ctx, plugin.Log, plugin.ifIndexes, plugin.ifLinuxNotifChan); err != nil {
		return plugin.ifConfigurator.LogError(err)
	}

	return nil
}

// Initialize linux L3 plugin
func (plugin *Plugin) initL3() error {
	plugin.Log.Infof("Init Linux L3 plugin")

	// Init shared ARP/Route index mapping
	arpIndexes := l3idx.NewLinuxARPIndex(nametoidx.NewNameToIdx(plugin.Log, "linux_arp_indexes", nil))
	routeIndexes := l3idx.NewLinuxRouteIndex(nametoidx.NewNameToIdx(plugin.Log, "linux_route_indexes", nil))

	// L3 linux calls handler
	l3Handler := l3Linuxcalls.NewNetLinkHandler(plugin.nsHandler, plugin.ifIndexes, arpIndexes, routeIndexes, plugin.Log)

	// Linux ARP configurator
	plugin.arpConfigurator = &l3plugin.LinuxArpConfigurator{}
	if err := plugin.arpConfigurator.Init(plugin.Log, l3Handler, plugin.nsHandler, arpIndexes, plugin.ifIndexes); err != nil {
		return plugin.arpConfigurator.LogError(err)
	}

	// Linux Route configurator
	plugin.routeConfigurator = &l3plugin.LinuxRouteConfigurator{}
	if err := plugin.routeConfigurator.Init(plugin.Log, l3Handler, plugin.nsHandler, routeIndexes, plugin.ifIndexes); err != nil {
		plugin.routeConfigurator.LogError(err)
	}

	return nil
}

func (plugin *Plugin) retrieveLinuxConfig() (*Config, error) {
	config := &Config{}
	found, err := plugin.Cfg.LoadValue(config)
	if !found {
		plugin.Log.Debug("Linuxplugin config not found")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plugin.Log.Debug("Linuxplugin config found")
	return config, err
}
