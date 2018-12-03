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

package main

import (
	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/vpp"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"log"
)

const agent1, agent2 = "agent1", "agent2"

// Start Agent plugins selected for this example.
func main() {
	// Agent 1 datasync plugin
	serviceLabel1 := servicelabel.NewPlugin(servicelabel.UseLabel(agent1))
	serviceLabel1.SetName(agent1)
	etcdDataSyncAgent1 := kvdbsync.NewPlugin(kvdbsync.UseKV(&etcd.DefaultPlugin), kvdbsync.UseDeps(func(deps *kvdbsync.Deps) {
		deps.Log = logging.ForPlugin(agent1)
		deps.ServiceLabel = serviceLabel1
	}))
	etcdDataSyncAgent1.SetName("etcd-datasync-" + agent1)

	// Agent 2 datasync plugin
	serviceLabel2 := servicelabel.NewPlugin(servicelabel.UseLabel(agent2))
	serviceLabel2.SetName(agent2)
	etcdDataSyncAgent2 := kvdbsync.NewPlugin(kvdbsync.UseKV(&etcd.DefaultPlugin), kvdbsync.UseDeps(func(deps *kvdbsync.Deps) {
		deps.Log = logging.ForPlugin(agent2)
		deps.ServiceLabel = serviceLabel2
	}))
	etcdDataSyncAgent2.SetName("etcd-datasync-" + agent2)

	// Example plugin datasync
	etcdDataSync := kvdbsync.NewPlugin(kvdbsync.UseKV(&etcd.DefaultPlugin))

	// VPP plugin
	watcher := datasync.KVProtoWatchers{
		etcdDataSync,
	}
	vppPlugin := vpp.NewPlugin(vpp.UseDeps(func(deps *vpp.Deps) {
		deps.Watcher = watcher
	}))

	// Inject dependencies to example plugin
	ep := &ExamplePlugin{
		exampleFinished: make(chan struct{}),
		Deps: Deps{
			Log:          logging.DefaultLogger,
			ETCDDataSync: etcdDataSync,
			VPP:          vppPlugin,
			Agent1:       etcdDataSyncAgent1,
			Agent2:       etcdDataSyncAgent2,
		},
	}

	// Start Agent
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(ep.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal()
	}
}

// PluginName represents name of plugin.
const PluginName = "idx-iface-cache-example"

// ExamplePlugin used for demonstration of SwIfIndexes - see Init()
type ExamplePlugin struct {
	Deps

	swIfIdxLocal  ifaceidx.SwIfIndex
	swIfIdxAgent1 ifaceidx.SwIfIndex
	swIfIdxAgent2 ifaceidx.SwIfIndex

	// Fields below are used to properly finish the example.
	exampleFinished chan struct{}
}

// Deps is a helper struct which is grouping all dependencies injected to the plugin
type Deps struct {
	Log          logging.Logger
	ETCDDataSync *kvdbsync.Plugin
	VPP          vpp.API
	Agent1       *kvdbsync.Plugin
	Agent2       *kvdbsync.Plugin
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init initializes transport & SwIfIndexes then watch, publish & lookup.
func (plugin *ExamplePlugin) Init() (err error) {
	// Get access to local interface indexes.
	plugin.swIfIdxLocal = plugin.VPP.GetSwIfIndexes()

	// Run consumer
	go plugin.consume()

	// Cache other agent's interface index mapping using injected plugin and local plugin name.
	// /vnf-agent/agent1/vpp/config/v1/interface/
	plugin.swIfIdxAgent1 = ifaceidx.Cache(plugin.Agent1)
	// /vnf-agent/agent2/vpp/config/v1/interface/
	plugin.swIfIdxAgent2 = ifaceidx.Cache(plugin.Agent2)

	return nil
}

// AfterInit - call Cache()
func (plugin *ExamplePlugin) AfterInit() error {
	// Publish test data.
	plugin.publish()

	return nil
}

// Close is called by Agent Core when the Agent is shutting down. It is supposed
// to clean up resources that were allocated by the plugin during its lifetime.
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.Agent1, plugin.Agent2, plugin.swIfIdxLocal, plugin.swIfIdxAgent1,
		plugin.swIfIdxAgent2)
}

// Test data are published to different agents (including local).
func (plugin *ExamplePlugin) publish() (err error) {
	// Create interface in local agent.
	iface0 := newExampleInterface("iface0", "192.168.0.1")
	err = plugin.ETCDDataSync.Put(interfaces.InterfaceKey(iface0.Name), iface0)
	if err != nil {
		return err
	}
	// Create interface in agent1.
	iface1 := newExampleInterface("iface1", "192.168.0.2")
	err = plugin.Agent1.Put(interfaces.InterfaceKey(iface1.Name), iface1)
	if err != nil {
		return err
	}
	// Create interface in agent2.
	iface2 := newExampleInterface("iface2", "192.168.0.3")
	err = plugin.Agent2.Put(interfaces.InterfaceKey(iface2.Name), iface2)
	return err
}

// Use the NameToIndexMapping to watch changes.
func (plugin *ExamplePlugin) consume() {
	plugin.Log.Info("Watching started")
	swIfIdxChan := make(chan ifaceidx.SwIfIdxDto)
	// Subscribe local iface-idx-mapping and both of cache mapping.
	plugin.swIfIdxLocal.WatchNameToIdx(PluginName, swIfIdxChan)
	plugin.swIfIdxAgent1.WatchNameToIdx(PluginName, swIfIdxChan)
	plugin.swIfIdxAgent2.WatchNameToIdx(PluginName, swIfIdxChan)

	counter := 0

	watching := true
	for watching {
		select {
		case ifaceIdxEvent := <-swIfIdxChan:
			plugin.Log.Info("Event received: interface ", ifaceIdxEvent.Name, " of ", ifaceIdxEvent.RegistryTitle)
			counter++
		}
		// Example is expecting 3 events
		if counter == 3 {
			watching = false
		}
	}

	// Do a lookup whether all mappings were registered.
	plugin.lookup()
}

// Use the NameToIndexMapping to lookup local mapping + external cached mappings.
func (plugin *ExamplePlugin) lookup() {
	plugin.Log.Info("Lookup in progress")

	if index, _, found := plugin.swIfIdxLocal.LookupIdx("iface0"); found {
		plugin.Log.Infof("interface iface0 (index %v) found in local mapping", index)
	}

	if index, _, found := plugin.swIfIdxAgent1.LookupIdx("iface1"); found {
		plugin.Log.Infof("interface iface1 (index %v) found in local mapping", index)
	}

	if index, _, found := plugin.swIfIdxAgent2.LookupIdx("iface2"); found {
		plugin.Log.Infof("interface iface2 (index %v) found in local mapping", index)
	}

	// End the example.
	plugin.Log.Infof("idx-iface-cache example finished, sending shutdown ...")
	close(plugin.exampleFinished)
}

func newExampleInterface(ifName, ipAddr string) *interfaces.Interfaces_Interface {
	return &interfaces.Interfaces_Interface{
		Name:        ifName,
		Enabled:     true,
		IpAddresses: []string{ipAddr},
	}
}
