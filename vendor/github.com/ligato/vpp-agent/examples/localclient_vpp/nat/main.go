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
	"github.com/ligato/vpp-agent/clientv1/vpp/localclient"
	"github.com/ligato/vpp-agent/plugins/vpp"
	vpp_intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/namsral/flag"
)

var (
	timeout = flag.Int("timeout", 20, "Timeout between applying of global and DNAT configuration in seconds")
)

/* Confgiuration */

// Global NAT is a one-time configuration (single key in the etcd, but it can be modified or removed as ususally).
// Configured items are 'global' for the whole NAT44 environment. Data consists of:
// - NAT forwarding setup
// - Enabled interfaces (including output feature)
// - Enabled address pools

/* Result of test NAT global data */
/*
vpp# sh nat44 interfaces
NAT44 interfaces:
 memif1/3 in
 memif1/1 out
 memif1/2 output-feature out

vpp# sh nat44 addresses
NAT44 pool addresses:
192.168.0.1
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
175.124.0.1
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
175.124.0.2
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
175.124.0.3
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
10.10.0.1
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
10.10.0.2
  tenant VRF: 0
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
NAT44 twice-nat pool addresses:
vpp#
*/

// DNAT puts static mapping (with or without load balancer) or identity mapping entries to the VPP. Destination
// address can be translated to one or more local addresses. If more than one local address is used, load
// balancer is configured automatically.

/* Result of DNAT test data */
/*
vpp# sh nat44 static mappings
NAT44 static mappings:
udp vrf 0 external 192.168.0.1:8989  out2in-only
	local 172.124.0.2:6500 probability 40
	local 172.125.10.5:2300 probability 40
udp local 172.124.0.3:6501 external 192.47.21.1:8989 vrf 0  out2in-only
tcp local 10.10.0.1:2525 external 10.10.0.1:2525 vrf 0
vpp#
*/

/* Vpp-agent Init and Close*/

// Start Agent plugins selected for this example.
func main() {
	//Init close channel to stop the example.
	exampleFinished := make(chan struct{}, 1)
	// Prepare all the dependencies for example plugin
	watcher := datasync.KVProtoWatchers{
		local.Get(),
	}
	vppPlugin := vpp.NewPlugin(vpp.UseDeps(func(deps *vpp.Deps) {
		deps.Watcher = watcher
	}))

	// Inject dependencies to example plugin
	ep := &NatExamplePlugin{}
	ep.Deps.Log = logging.DefaultLogger
	ep.Deps.VPP = vppPlugin

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

/* NAT44 Example */

// NatExamplePlugin uses localclient to transport example global NAT and DNAT and af-packet
// configuration to NAT VPP plugin
type NatExamplePlugin struct {
	Deps

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Deps is example plugin dependencies.
type Deps struct {
	Log logging.Logger
	VPP *vpp.Plugin
}

// PluginName represents name of plugin.
const PluginName = "nat-example"

// Init initializes example plugin.
func (plugin *NatExamplePlugin) Init() error {
	// Logger
	plugin.Log = logrus.DefaultLogger()
	plugin.Log.SetLevel(logging.DebugLevel)
	plugin.Log.Info("Initializing NAT44 example")

	// Flags
	flag.Parse()
	plugin.Log.Infof("Timeout between configuring NAT global and DNAT set to %d", *timeout)

	// Apply initial VPP configuration.
	plugin.putGlobalConfig()

	// Schedule reconfiguration.
	var ctx context.Context
	ctx, plugin.cancel = context.WithCancel(context.Background())
	plugin.wg.Add(1)
	go plugin.putDNAT(ctx, *timeout)

	plugin.Log.Info("NAT example initialization done")
	return nil
}

// Close cleans up the resources.
func (plugin *NatExamplePlugin) Close() error {
	plugin.cancel()
	plugin.wg.Wait()

	logrus.DefaultLogger().Info("Closed NAT example plugin")
	return nil
}

// String returns plugin name
func (plugin *NatExamplePlugin) String() string {
	return PluginName
}

// Configure NAT44 Global config
func (plugin *NatExamplePlugin) putGlobalConfig() {
	plugin.Log.Infof("Applying NAT44 global configuration")
	err := localclient.DataResyncRequest(PluginName).
		Interface(interface1()).
		Interface(interface2()).
		Interface(interface3()).
		NAT44Global(globalNat()).
		Send().ReceiveReply()
	if err != nil {
		plugin.Log.Errorf("NAT44 global configuration failed: %v", err)
	} else {
		plugin.Log.Info("NAT44 global configuration successful")
	}
}

// Configure DNAT
func (plugin *NatExamplePlugin) putDNAT(ctx context.Context, timeout int) {
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		plugin.Log.Infof("Applying DNAT configuration")
		err := localclient.DataChangeRequest(PluginName).
			Put().
			NAT44DNat(dNat()).
			Send().ReceiveReply()
		if err != nil {
			plugin.Log.Errorf("DNAT configuration failed: %v", err)
		} else {
			plugin.Log.Info("DNAT configuration successful")
		}
	case <-ctx.Done():
		// Cancel the scheduled DNAT configuration.
		plugin.Log.Info("DNAT configuration canceled")
	}
	plugin.wg.Done()
}

/* Example Data */

func interface1() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:    "memif1",
		Type:    vpp_intf.InterfaceType_MEMORY_INTERFACE,
		Enabled: true,
		Mtu:     1478,
		IpAddresses: []string{
			"172.125.40.1/24",
		},
		Memif: &vpp_intf.Interfaces_Interface_Memif{
			Id:             1,
			Secret:         "secret1",
			Master:         false,
			SocketFilename: "/tmp/memif1.sock",
		},
	}
}

func interface2() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:    "memif2",
		Type:    vpp_intf.InterfaceType_MEMORY_INTERFACE,
		Enabled: true,
		Mtu:     1478,
		IpAddresses: []string{
			"192.47.21.1/24",
		},
		Memif: &vpp_intf.Interfaces_Interface_Memif{
			Id:             2,
			Secret:         "secret2",
			Master:         false,
			SocketFilename: "/tmp/memif1.sock",
		},
	}
}

func interface3() *vpp_intf.Interfaces_Interface {
	return &vpp_intf.Interfaces_Interface{
		Name:    "memif3",
		Type:    vpp_intf.InterfaceType_MEMORY_INTERFACE,
		Enabled: true,
		Mtu:     1478,
		IpAddresses: []string{
			"94.18.21.1/24",
		},
		Memif: &vpp_intf.Interfaces_Interface_Memif{
			Id:             3,
			Secret:         "secret3",
			Master:         false,
			SocketFilename: "/tmp/memif1.sock",
		},
	}
}

func globalNat() *nat.Nat44Global {
	return &nat.Nat44Global{
		Forwarding: false,
		NatInterfaces: []*nat.Nat44Global_NatInterface{
			{
				Name:          "memif1",
				IsInside:      false,
				OutputFeature: false,
			},
			{
				Name:          "memif2",
				IsInside:      false,
				OutputFeature: true,
			},
			{
				Name:          "memif3",
				IsInside:      true,
				OutputFeature: false,
			},
		},
		AddressPools: []*nat.Nat44Global_AddressPool{
			{
				VrfId:           0,
				FirstSrcAddress: "192.168.0.1",
				TwiceNat:        false,
			},
			{
				VrfId:           0,
				FirstSrcAddress: "175.124.0.1",
				LastSrcAddress:  "175.124.0.3",
				TwiceNat:        false,
			},
			{
				VrfId:           0,
				FirstSrcAddress: "10.10.0.1",
				LastSrcAddress:  "10.10.0.2",
				TwiceNat:        false,
			},
		},
	}
}

func dNat() *nat.Nat44DNat_DNatConfig {
	return &nat.Nat44DNat_DNatConfig{
		Label: "dnat1",
		StMappings: []*nat.Nat44DNat_DNatConfig_StaticMapping{
			{
				// DNAT static mapping with load balancer (multiple local addresses)
				ExternalInterface: "memif1",
				ExternalIp:        "192.168.0.1",
				ExternalPort:      8989,
				LocalIps: []*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP{
					{
						VrfId:       0,
						LocalIp:     "172.124.0.2",
						LocalPort:   6500,
						Probability: 40,
					},
					{
						VrfId:       0,
						LocalIp:     "172.125.10.5",
						LocalPort:   2300,
						Probability: 40,
					},
				},
				Protocol: 1,
				TwiceNat: nat.TwiceNatMode_ENABLED,
			},
			{
				// DNAT static mapping without load balancer (single local address)
				ExternalInterface: "memif2",
				ExternalIp:        "192.168.0.2",
				ExternalPort:      8989,
				LocalIps: []*nat.Nat44DNat_DNatConfig_StaticMapping_LocalIP{
					{
						VrfId:       0,
						LocalIp:     "172.124.0.3",
						LocalPort:   6501,
						Probability: 50,
					},
				},
				Protocol: 1,
				TwiceNat: nat.TwiceNatMode_ENABLED,
			},
		},
		IdMappings: []*nat.Nat44DNat_DNatConfig_IdentityMapping{
			{
				VrfId:     0,
				IpAddress: "10.10.0.1",
				Port:      2525,
				Protocol:  0,
			},
		},
	}
}
