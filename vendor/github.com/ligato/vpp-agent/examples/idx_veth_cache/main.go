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
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/plugins/linux"
	linux_if "github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	linux_intf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
)

// *************************************************************************
// This file contains an examples of linux plugin name-to-index cache operations.
//
// Two more transport adapters for different agents are registered using
// OfDifferentAgent() and their interface name-to-idx mapping is cached
// with linux_if.Cache() as a new map.
//
// These new maps are watched and all new events are logged.
//
// VETH interfaces are then created on agents using both the transports and
// data presence in cached name-to-idx map is verified.
// ************************************************************************/

// Init sets the default logging level.
func init() {
	logrus.DefaultLogger().SetOutput(os.Stdout)
	logrus.DefaultLogger().SetLevel(logging.InfoLevel)
}

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

	// Linux plugin
	watcher := datasync.KVProtoWatchers{
		etcdDataSync,
	}
	linuxPlugin := linux.NewPlugin(linux.UseDeps(func(deps *linux.Deps) {
		deps.Watcher = watcher
	}))

	// Inject dependencies to example plugin
	ep := &ExamplePlugin{
		exampleFinished: make(chan struct{}),
		Deps: Deps{
			Log:          logging.DefaultLogger,
			ETCDDataSync: etcdDataSync,
			Linux:        linuxPlugin,
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
const PluginName = "idx-veth-cache-example"

// ExamplePlugin demonstrates the use of the name-to-idx cache in linux plugin.
type ExamplePlugin struct {
	Deps

	linuxIfIdxLocal  linux_if.LinuxIfIndex
	linuxIfIdxAgent1 linux_if.LinuxIfIndex
	linuxIfIdxAgent2 linux_if.LinuxIfIndex
	wg               sync.WaitGroup

	// Fields below are used to properly finish the example.
	exampleFinished chan struct{}
}

// Deps is a helper struct which is grouping all dependencies injected to the plugin
type Deps struct {
	Log          logging.Logger
	ETCDDataSync *kvdbsync.Plugin
	Linux        *linux.Plugin
	Agent1       *kvdbsync.Plugin
	Agent2       *kvdbsync.Plugin
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init initializes example plugin.
func (plugin *ExamplePlugin) Init() error {
	// Receive linux interfaces mapping.
	if plugin.Linux != nil {
		plugin.linuxIfIdxLocal = plugin.Linux.GetLinuxIfIndexes()
	} else {
		return fmt.Errorf("linux plugin not initialized")
	}

	// Run consumer.
	go plugin.consume()

	// Cache the agent1/agent2 name-to-idx mapping to separate mapping within plugin example.
	plugin.linuxIfIdxAgent1 = linux_if.Cache(plugin.Agent1)
	plugin.linuxIfIdxAgent2 = linux_if.Cache(plugin.Agent2)

	logrus.DefaultLogger().Info("Initialization of the example plugin has completed")

	return nil
}

// AfterInit - call Cache()
func (plugin *ExamplePlugin) AfterInit() error {
	// Manually start resync (simulate vpp-agent default behaviour)
	resync.DefaultPlugin.DoResync()

	// Publish test data.
	plugin.publish()

	return nil
}

// Close cleans up the resources.
func (plugin *ExamplePlugin) Close() error {
	plugin.wg.Wait()

	return safeclose.Close(plugin.Agent1, plugin.Agent2, plugin.Agent1, plugin.Agent2,
		plugin.linuxIfIdxLocal, plugin.linuxIfIdxAgent1, plugin.linuxIfIdxAgent2)
}

// publish propagates example configuration to etcd.
func (plugin *ExamplePlugin) publish() error {
	logrus.DefaultLogger().Infof("Putting interfaces to ETCD")

	// VETH pair in default namespace
	vethDef := &veth11DefaultNs
	vethDefPeer := &veth12DefaultNs

	// Publish VETH pair to agent1.
	err := plugin.Agent1.Put(linux_intf.InterfaceKey(vethDef.Name), vethDef)
	err = plugin.Agent1.Put(linux_intf.InterfaceKey(vethDefPeer.Name), vethDefPeer)

	// VETH pair in custom namespace
	vethNs1 := &veth21Ns1
	vethNs2Peer := &veth22Ns2

	// Publish VETH pair to agent2.
	err = plugin.Agent2.Put(linux_intf.InterfaceKey(vethNs1.Name), vethDef)
	err = plugin.Agent2.Put(linux_intf.InterfaceKey(vethNs2Peer.Name), vethNs2Peer)

	if err != nil {
		logrus.DefaultLogger().Errorf("Failed to apply initial Linux&VPP configuration: %v", err)
		return err
	}
	logrus.DefaultLogger().Info("Successfully applied initial Linux&VPP configuration")

	return err
}

// Use the NameToIndexMapping to watch changes.
func (plugin *ExamplePlugin) consume() (err error) {
	plugin.Log.Info("Watching started")
	// Init chan to sent watch updates.
	linuxIfIdxChan := make(chan linux_if.LinuxIfIndexDto)
	// Register all agents (incl. local) to watch linux name-to-idx mapping changes.
	plugin.linuxIfIdxLocal.WatchNameToIdx(PluginName, linuxIfIdxChan)
	plugin.linuxIfIdxAgent1.WatchNameToIdx(PluginName, linuxIfIdxChan)
	plugin.linuxIfIdxAgent2.WatchNameToIdx(PluginName, linuxIfIdxChan)

	counter := 0

	watching := true
	for watching {
		select {
		case ifaceIdxEvent := <-linuxIfIdxChan:
			plugin.Log.Info("Event received: VETH interface ", ifaceIdxEvent.Name,
				" of ", ifaceIdxEvent.RegistryTitle)
			counter++
		}
		// Example is expecting 3 events.
		if counter == 4 {
			watching = false
		}
	}

	// Do a lookup whether all mappings were registered.
	success := plugin.lookup()
	if !success {
		return fmt.Errorf("idx_veth_cache example failed; one or more VETH interfaces are missing")
	}

	// End the example.
	plugin.Log.Infof("idx-iface-cache example finished, sending shutdown ...")
	close(plugin.exampleFinished)

	return nil
}

// Use the NameToIndexMapping to lookup changes.
func (plugin *ExamplePlugin) lookup() bool {
	var (
		loopback bool
		veth11   bool
		veth12   bool
		veth21   bool
		veth22   bool
	)

	// Look for loopback interface.
	if _, _, loopback = plugin.linuxIfIdxLocal.LookupIdx("lo"); loopback {
		logrus.DefaultLogger().Info("Interface found: loopback")
	} else {
		logrus.DefaultLogger().Warn("Interface not found: loopback")
	}
	// Look for VETH 11 default namespace interface on agent1.
	for i := 0; i <= 10; i++ {
		if _, _, veth11 = plugin.linuxIfIdxAgent1.LookupIdx(veth11DefaultNs.Name); veth11 {
			logrus.DefaultLogger().Info("Interface found on agent1: veth11Def")
			break
		} else if i == 3 {
			logrus.DefaultLogger().Warn("Interface not found on agent1: veth11Def")
		} else {
			// Try several times in case cache is not updated yet.
			time.Sleep(1 * time.Second)
			continue
		}
	}
	// Look for VETH 12 default namespace interface on agent1.
	for i := 0; i <= 3; i++ {
		if _, _, veth12 = plugin.linuxIfIdxAgent1.LookupIdx(veth12DefaultNs.Name); veth12 {
			logrus.DefaultLogger().Info("Interface found on agent1: veth12Def")
			break
		} else if i == 3 {
			logrus.DefaultLogger().Warn("Interface not found on agent1: veth12Def")
		} else {
			// Try several times in case cache is not updated yet.
			time.Sleep(1 * time.Second)
			continue
		}
	}
	// Look for VETH 21 ns1 namespace interface on agent2.
	for i := 0; i <= 3; i++ {
		if _, _, veth21 = plugin.linuxIfIdxAgent2.LookupIdx(veth21Ns1.Name); veth21 {
			logrus.DefaultLogger().Info("Interface found on agent2: veth21ns1")
			break
		} else if i == 3 {
			logrus.DefaultLogger().Warn("Interface not found on agent2 : veth21ns1")
		} else {
			// Try several times in case cache is not updated yet.
			time.Sleep(1 * time.Second)
			continue
		}
	}
	// Look for VETH 22 ns2 namespace interface on agent2.
	for i := 0; i <= 3; i++ {
		if _, _, veth22 = plugin.linuxIfIdxAgent2.LookupIdx(veth22Ns2.Name); veth22 {
			logrus.DefaultLogger().Info("Interface found on agent2: veth22ns2")
			break
		} else if i == 3 {
			logrus.DefaultLogger().Warn("Interface not found on agent2: veth22ns2")
		} else {
			// Try several times in case cache is not updated yet.
			time.Sleep(1 * time.Second)
			continue
		}
	}

	if loopback && veth11 && veth12 && veth21 && veth22 {
		return true
	}
	return false
}

// Interface data
var (
	// veth11DefaultNs is one member of the veth11-veth12DefaultNs VETH pair, put into the default namespace
	veth11DefaultNs = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth1",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth12DefaultNs",
		},
		IpAddresses: []string{"10.0.0.1/24"},
	}
	// veth12DefaultNs is one member of the veth11-veth12DefaultNs VETH pair, put into the default namespace
	veth12DefaultNs = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth12DefaultNs",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth11",
		},
	}
	// veth11DefaultNs is one member of the veth21-veth22 VETH pair, put into the ns1.
	veth21Ns1 = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth11",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth12DefaultNs",
		},
		IpAddresses: []string{"10.0.0.1/24"},
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
			Name: "ns1",
		},
	}
	// veth22Ns2 is one member of the veth21-veth22 VETH pair, put into the namespace "ns2".
	veth22Ns2 = linux_intf.LinuxInterfaces_Interface{
		Name:    "veth21",
		Type:    linux_intf.LinuxInterfaces_VETH,
		Enabled: true,
		Veth: &linux_intf.LinuxInterfaces_Interface_Veth{
			PeerIfName: "veth22",
		},
		IpAddresses: []string{"10.0.0.2/24"},
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
			Name: "ns2",
		},
	}
)
