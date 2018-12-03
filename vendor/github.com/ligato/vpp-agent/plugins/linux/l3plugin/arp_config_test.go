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

package l3plugin_test

import (
	"fmt"
	"testing"

	"github.com/ligato/vpp-agent/plugins/linux/l3plugin/l3idx"

	"net"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/l3plugin"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
	"github.com/ligato/vpp-agent/tests/linuxmock"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

/* Linux ARP configurator init and close */

// Test init function
func TestLinuxArpConfiguratorInit(t *testing.T) {
	plugin, _, _, _ := arpTestSetup(t)
	defer arpTestTeardown(plugin)
	// Base fields
	Expect(plugin).ToNot(BeNil())
	// Mappings & cache
	Expect(plugin.GetArpIndexes()).ToNot(BeNil())
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpInterfaceCache()).ToNot(BeNil())
	Expect(plugin.GetArpInterfaceCache()).To(HaveLen(0))
}

/* Linux ARP configurator test cases */

// Configure ARP entry
func TestLinuxConfiguratorAddARP(t *testing.T) {
	plugin, _, _, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1", "00:00:00:00:00:01", 2, nil, nil)
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, meta, found := plugin.GetArpIndexes().LookupIdx(l3plugin.ArpIdentifier(getArpID(1, "10.0.0.1", "00:00:00:00:00:01")))
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(plugin.GetArpInterfaceCache()).ToNot(HaveKeyWithValue("arp1", BeNil()))
}

// Configure ARP entry with missing interface
func TestLinuxConfiguratorAddARPMissingInterface(t *testing.T) {
	plugin, _, _, _ := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1", "00:00:00:00:00:01",
		2, nil, nil)
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).ShouldNot(HaveOccurred())
	_, _, found := plugin.GetArpIndexes().LookupIdx(l3plugin.ArpIdentifier(getArpID(1, "10.0.0.1", "00:00:00:00:00:01")))
	Expect(found).To(BeFalse())
	Expect(plugin.GetArpInterfaceCache()).To(HaveKeyWithValue("arp1", Not(BeNil())))
}

// Configure ARP entry with parse IP error
func TestLinuxConfiguratorAddARPParseIPErr(t *testing.T) {
	plugin, _, _, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1/24", "00:00:00:00:00:01", 2, nil, nil)
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).Should(HaveOccurred())
	_, _, found := plugin.GetArpIndexes().LookupIdx(l3plugin.ArpIdentifier(getArpID(1, "10.0.0.1", "00:00:00:00:00:01")))
	Expect(found).To(BeFalse())
	Expect(plugin.GetArpInterfaceCache()).ToNot(HaveKeyWithValue("arp1", BeNil()))
}

// Configure ARP entry with parse MAC error
func TestLinuxConfiguratorAddARPParseMacErr(t *testing.T) {
	plugin, _, _, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1", "faulty-mac", 2, nil, nil)
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).Should(HaveOccurred())
}

// Configure ARP entry with error while switching namespace
func TestLinuxConfiguratorAddARPSwitchNamespaceError(t *testing.T) {
	plugin, _, nsMock, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	nsMock.When("SwitchNamespace").ThenReturn(fmt.Errorf("switch-ns-err"))

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1", "00:00:00:00:00:01", 2,
		&l3.LinuxStaticArpEntries_ArpEntry_IpFamily{
			Family: l3.LinuxStaticArpEntries_ArpEntry_IpFamily_IPV4,
		}, &l3.LinuxStaticArpEntries_ArpEntry_NudState{
			Type: l3.LinuxStaticArpEntries_ArpEntry_NudState_PERMANENT,
		})
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).Should(HaveOccurred())
}

// Configure ARP entry with error while adding entry
func TestLinuxConfiguratorAddARPError(t *testing.T) {
	plugin, l3Mock, _, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(plugin)

	l3Mock.When("AddArpEntry").ThenReturn(fmt.Errorf("add-arp-entry-error"))

	// Register interface
	ifIndexes.RegisterName("if1", 1, getInterfaceData("if1", 1))
	// Test ARP create
	data := getTestARP("arp1", "if1", "10.0.0.1", "00:00:00:00:00:01", 2,
		&l3.LinuxStaticArpEntries_ArpEntry_IpFamily{
			Family: l3.LinuxStaticArpEntries_ArpEntry_IpFamily_IPV6,
		}, &l3.LinuxStaticArpEntries_ArpEntry_NudState{
			Type: l3.LinuxStaticArpEntries_ArpEntry_NudState_NOARP,
		})
	err := plugin.ConfigureLinuxStaticArpEntry(data)
	Expect(err).Should(HaveOccurred())
}

/* ARP Test Setup */

func arpTestSetup(t *testing.T) (*l3plugin.LinuxArpConfigurator, *linuxmock.L3NetlinkHandlerMock, *linuxmock.NamespacePluginMock, ifaceidx.LinuxIfIndexRW) {
	RegisterTestingT(t)

	// Loggers
	pluginLog := logging.ForPlugin("linux-arp-log")
	pluginLog.SetLevel(logging.DebugLevel)
	nsHandleLog := logging.ForPlugin("ns-handle-log")
	nsHandleLog.SetLevel(logging.DebugLevel)
	// Linux interface indexes
	ifIndexes := ifaceidx.NewLinuxIfIndex(nametoidx.NewNameToIdx(pluginLog, "if", nil))
	arpIndexes := l3idx.NewLinuxARPIndex(nametoidx.NewNameToIdx(pluginLog, "arp", nil))
	// Configurator
	plugin := &l3plugin.LinuxArpConfigurator{}
	linuxMock := linuxmock.NewL3NetlinkHandlerMock()
	nsMock := linuxmock.NewNamespacePluginMock()
	err := plugin.Init(pluginLog, linuxMock, nsMock, arpIndexes, ifIndexes)
	Expect(err).To(BeNil())

	return plugin, linuxMock, nsMock, ifIndexes
}

func arpTestTeardown(plugin *l3plugin.LinuxArpConfigurator) {
	err := plugin.Close()
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

func getArpID(ifIdx uint32, ip, mac string) *netlink.Neigh {
	return &netlink.Neigh{
		LinkIndex: int(ifIdx),
		IP:        net.ParseIP(ip),
		HardwareAddr: func(mac string) net.HardwareAddr {
			hw, err := net.ParseMAC(mac)
			if err != nil {
				panic(err)
			}
			return hw
		}(mac),
	}
}

/* Linux ARP Test Data */

func getTestARP(arpName, ifName, ip, mac string, namespaceType l3.LinuxStaticArpEntries_ArpEntry_Namespace_NamespaceType,
	family *l3.LinuxStaticArpEntries_ArpEntry_IpFamily, nudState *l3.LinuxStaticArpEntries_ArpEntry_NudState) *l3.LinuxStaticArpEntries_ArpEntry {
	return &l3.LinuxStaticArpEntries_ArpEntry{
		Name: arpName,
		Namespace: func(namespaceType l3.LinuxStaticArpEntries_ArpEntry_Namespace_NamespaceType) *l3.LinuxStaticArpEntries_ArpEntry_Namespace {
			if namespaceType < 4 {
				return &l3.LinuxStaticArpEntries_ArpEntry_Namespace{
					Type:         namespaceType,
					Microservice: arpName + "-ms",
				}
			}
			return nil
		}(namespaceType),
		Interface: ifName,
		IpAddr:    ip,
		HwAddress: mac,
		IpFamily:  family,
		State:     nudState,
	}
}

func getInterfaceData(ifName string, idx uint32) *ifaceidx.IndexedLinuxInterface {
	return &ifaceidx.IndexedLinuxInterface{
		Index: idx,
		Data: &interfaces.LinuxInterfaces_Interface{
			Name: ifName,
		},
	}
}
