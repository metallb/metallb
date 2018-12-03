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
	"context"
	"log"
	"sync"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync/local"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/clientv1/linux/localclient"
	"github.com/ligato/vpp-agent/plugins/linux"
	linux_intf "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp"
	vpp_intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	vpp_l2 "github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/namsral/flag"
)

var (
	timeout = flag.Int("timeout", 20, "Timeout between applying of initial and modified configuration in seconds")
)

/* Confgiuration */

// Initial Data configures TAP interface on the vpp with other end in the default namespace. Linux-tap is then set with
// IP address. Also bridge domain is created for linux TAP interfaces
/**********************************************
 * Initial Data                               *
 *                                            *
 *  +--------------------------------------+  *
 *  |       +-- Bridge Domain --+          |  *
 *  |       |                   |          |  *
 *  |   +-------+               |          |  *
 *  |   | tap1  |             (tap2)       |  *
 *  |   +---+---+                          |  *
 *  |       |                              |  *
 *  +-------+------------------------------+  *
 *          |                                 *
 *  +-------+---------+                       *
 *  | linux-tap1      |                       *
 *  | IP: 10.0.0.2/24 |                       *
 *  +-----------------+                       *
 *                                            *
 *                                            *
 **********************************************/

// Modify sets IP address for tap1, moves linux host to namespace ns1 and configures second TAP interface with linux
// host in namespace ns2
/************************************************
 * Initial Data                                 *
 *                                              *
 *  +----------------------------------------+  *
 *  |       +-- Bridge Domain --+            |  *
 *  |       |                   |            |  *
 *  | +-----+--------+    *------+-------+   |  *
 *  | | tap1         |    | (tap2)       |   |  *
 *  | | 10.0.0.11/24 |    | 20.0.0.11/24 |   |  *
 *  | +-----+--------+    +------+-------+   |  *
 *  |       |                   |            |  *
 *  +-------+-------------------+------------+  *
 *          |                   |               *
 *  +-------+----------+   +-----+------------+ *
 *  | linux-tap1       |   | linux-tap2       | *
 *  | IP: 10.0.0.12/24 |   | IP: 20.0.0.12\24 | *
 *  | Namespace: ns1   |   | Namespace: ns2   | *
 *  +------------------+   +------------------+ *
 *                                              *
 *                                              *
 ************************************************/

/* Vpp-agent Init and Close*/

// Start Agent plugins selected for this example.
func main() {
	//Init close channel to stop the example.
	exampleFinished := make(chan struct{})
	// Prepare all the dependencies for example plugin
	watcher := datasync.KVProtoWatchers{
		local.Get(),
	}
	vppPlugin := vpp.NewPlugin(vpp.UseDeps(func(deps *vpp.Deps) {
		deps.Watcher = watcher
	}))
	linuxPlugin := linux.NewPlugin(linux.UseDeps(func(deps *linux.Deps) {
		deps.VPP = vppPlugin
		deps.Watcher = watcher
	}))
	vppPlugin.Deps.Linux = linuxPlugin

	var watchEventsMutex sync.Mutex
	vppPlugin.Deps.WatchEventsMutex = &watchEventsMutex
	linuxPlugin.Deps.WatchEventsMutex = &watchEventsMutex

	// Inject dependencies to example plugin
	ep := &TapExamplePlugin{
		Log: logging.DefaultLogger,
	}
	ep.Deps.VPP = vppPlugin
	ep.Deps.Linux = linuxPlugin

	// Start Agent
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal()
	}

	go closeExample("localhost example finished", exampleFinished)
}

// Stop the agent with desired info message.
func closeExample(message string, exampleFinished chan struct{}) {
	time.Sleep(time.Duration(*timeout+5) * time.Second)
	logrus.DefaultLogger().Info(message)
	close(exampleFinished)
}

/* TAP Example */

// TapExamplePlugin uses localclient to transport example tap and its linux end
// configuration to linuxplugin or VPP plugins
type TapExamplePlugin struct {
	Deps

	Log    logging.Logger
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Deps is example plugin dependencies. Keep order of fields.
type Deps struct {
	VPP   *vpp.Plugin
	Linux *linux.Plugin
}

// PluginName represents name of plugin.
const PluginName = "tap-example"

// Init initializes example plugin.
func (plugin *TapExamplePlugin) Init() error {
	// Logger
	plugin.Log = logrus.DefaultLogger()
	plugin.Log.SetLevel(logging.DebugLevel)
	plugin.Log.Info("Initializing Tap example")

	// Flags
	flag.Parse()
	plugin.Log.Infof("Timeout between create and modify set to %d", *timeout)

	// Apply initial Linux/VPP configuration.
	plugin.putInitialData()

	// Schedule reconfiguration.
	var ctx context.Context
	ctx, plugin.cancel = context.WithCancel(context.Background())
	plugin.wg.Add(1)
	go plugin.putModifiedData(ctx, *timeout)

	plugin.Log.Info("Tap example initialization done")
	return nil
}

// Close cleans up the resources.
func (plugin *TapExamplePlugin) Close() error {
	plugin.cancel()
	plugin.wg.Wait()

	plugin.Log.Info("Closed Tap plugin")
	return nil
}

// String returns plugin name
func (plugin *TapExamplePlugin) String() string {
	return PluginName
}

// Configure initial data
func (plugin *TapExamplePlugin) putInitialData() {
	plugin.Log.Infof("Applying initial configuration")
	err := localclient.DataResyncRequest(PluginName).
		VppInterface(initialTap1()).
		LinuxInterface(initialLinuxTap1()).
		BD(bridgeDomain()).
		Send().ReceiveReply()
	if err != nil {
		plugin.Log.Errorf("Initial configuration failed: %v", err)
	} else {
		plugin.Log.Info("Initial configuration successful")
	}
}

// Configure modified data
func (plugin *TapExamplePlugin) putModifiedData(ctx context.Context, timeout int) {
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		plugin.Log.Infof("Applying modified configuration")
		// Simulate configuration change after timeout
		err := localclient.DataChangeRequest(PluginName).
			Put().
			VppInterface(modifiedTap1()).
			VppInterface(tap2()).
			LinuxInterface(modifiedLinuxTap1()).
			LinuxInterface(linuxTap2()).
			Send().ReceiveReply()
		if err != nil {
			plugin.Log.Errorf("Modified configuration failed: %v", err)
		} else {
			plugin.Log.Info("Modified configuration successful")
		}
	case <-ctx.Done():
		// Cancel the scheduled re-configuration.
		plugin.Log.Info("Modification of configuration canceled")
	}
	plugin.wg.Done()
}

/* Example Data */

func initialTap1() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:    "tap1",
		Type:    vpp_intf.InterfaceType_TAP_INTERFACE,
		Enabled: true,
		Tap: &vpp_intf.Interfaces_Interface_Tap{
			HostIfName: "linux-tap1",
		},
	}
}

func modifiedTap1() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:        "tap1",
		Type:        vpp_intf.InterfaceType_TAP_INTERFACE,
		Enabled:     true,
		PhysAddress: "12:E4:0E:D5:BC:DC",
		IpAddresses: []string{
			"10.0.0.11/24",
		},
		Tap: &vpp_intf.Interfaces_Interface_Tap{
			HostIfName: "linux-tap1",
		},
	}
}

func tap2() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:        "tap2",
		Type:        vpp_intf.InterfaceType_TAP_INTERFACE,
		Enabled:     true,
		PhysAddress: "D5:BC:DC:12:E4:0E",
		IpAddresses: []string{
			"20.0.0.11/24",
		},
		Tap: &vpp_intf.Interfaces_Interface_Tap{
			HostIfName: "linux-tap2",
		},
	}
}

func initialLinuxTap1() *linux_intf.LinuxInterfaces_Interface {
	return &linux_intf.LinuxInterfaces_Interface{

		Name:        "linux-tap1",
		Type:        linux_intf.LinuxInterfaces_AUTO_TAP,
		Enabled:     true,
		PhysAddress: "BC:FE:E9:5E:07:04",
		Mtu:         1500,
		IpAddresses: []string{
			"10.0.0.12/24",
		},
	}
}

func modifiedLinuxTap1() *linux_intf.LinuxInterfaces_Interface {
	return &linux_intf.LinuxInterfaces_Interface{

		Name:        "linux-tap1",
		Type:        linux_intf.LinuxInterfaces_AUTO_TAP,
		Enabled:     true,
		PhysAddress: "BC:FE:E9:5E:07:04",
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Name: "ns1",
			Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
		},
		Mtu: 1500,
		IpAddresses: []string{
			"10.0.0.12/24",
		},
	}
}

func linuxTap2() *linux_intf.LinuxInterfaces_Interface {
	return &linux_intf.LinuxInterfaces_Interface{

		Name:        "linux-tap2",
		Type:        linux_intf.LinuxInterfaces_AUTO_TAP,
		Enabled:     true,
		PhysAddress: "5E:07:04:BC:FE:E9",
		Namespace: &linux_intf.LinuxInterfaces_Interface_Namespace{
			Name: "ns2",
			Type: linux_intf.LinuxInterfaces_Interface_Namespace_NAMED_NS,
		},
		Mtu: 1500,
		IpAddresses: []string{
			"20.0.0.12/24",
		},
	}
}

func bridgeDomain() *vpp_l2.BridgeDomains_BridgeDomain {
	return &vpp_l2.BridgeDomains_BridgeDomain{
		Name:                "br1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      false,
		MacAge:              0, /* means disable aging */
		Interfaces: []*vpp_l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "tap1",
				BridgedVirtualInterface: false,
			},
			{
				Name: "tap2",
				BridgedVirtualInterface: false,
			},
			{
				Name: "loop1",
				BridgedVirtualInterface: true,
			},
		},
	}
}
