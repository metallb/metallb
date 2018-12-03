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
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test adding of ARP entry to cached indexes
func TestAddArpCached(t *testing.T) {
	// Setup
	_, connection, plugin, _ := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	err := plugin.AddArp(&l3.ArpTable_ArpEntry{
		Interface:   "tap3",
		IpAddress:   "dead::01",
		PhysAddress: "59:6C:DE:AD:00:03",
		Static:      false,
	})
	Expect(err).ShouldNot(HaveOccurred())
	arps := plugin.GetArpCache().GetMapping().ListNames()
	Expect(arps).To(HaveLen(1))
	Expect(arps[0]).To(Equal("arp-iface-tap3-59:6C:DE:AD:00:03-dead::01"))
}

// Test adding of ARP entry to cached indexes
func TestAddArpAndResolve(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	err := plugin.AddArp(&l3.ArpTable_ArpEntry{
		Interface:   "tap3",
		IpAddress:   "dead::01",
		PhysAddress: "59:6C:DE:AD:00:03",
		Static:      false,
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(0))

	// add interface
	ifIndexes.GetMapping().RegisterName("tap3", 1, nil)
	// reslove interfaces
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err = plugin.ResolveCreatedInterface("tap3")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(1))
}

func TestAddArp(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	// add interface
	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	// add arp
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := plugin.AddArp(&l3.ArpTable_ArpEntry{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:DE:AD:00:01",
		Static:      true,
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
	arps := plugin.GetArpIndexes().GetMapping().ListNames()
	Expect(arps).To(HaveLen(1))
	Expect(arps[0]).To(Equal("arp-iface-tap1-59:6C:DE:AD:00:01-192.168.10.21"))
}

// Test ARP entry contains all required fields
func TestIsValidARP(t *testing.T) {
	// Setup
	_, connection, plugin, _ := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)
	// Test isValidARP
	err := plugin.AddArp(&l3.ArpTable_ArpEntry{})
	Expect(err).Should(HaveOccurred())
	err = plugin.AddArp(&l3.ArpTable_ArpEntry{Interface: "tap5"})
	Expect(err).Should(HaveOccurred())
	err = plugin.AddArp(&l3.ArpTable_ArpEntry{Interface: "tap6", IpAddress: "192.168.10.33"})
	Expect(err).Should(HaveOccurred())
}

// Test deleting of ARP entry for non existing intf
func TestDeleteArpNonExistingIntf(t *testing.T) {
	// Setup
	_, connection, plugin, _ := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	num_arps := len(plugin.GetArpIndexes().GetMapping().ListNames())
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(0))
	// delete arp for non existing intf
	err := plugin.DeleteArp(&l3.ArpTable_ArpEntry{
		Interface:   "tap4",
		IpAddress:   "dead::02",
		PhysAddress: "59:6C:DE:AD:00:04",
		Static:      false,
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(num_arps))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(1))
}

// Test deleting of cached ARP entry
func TestDeleteArpCached(t *testing.T) {
	// Setup
	_, connection, plugin, _ := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	arp_entry := &l3.ArpTable_ArpEntry{
		Interface:   "tap3",
		IpAddress:   "dead::01",
		PhysAddress: "59:6C:DE:AD:00:03",
		Static:      false,
	}

	plugin.GetArpCache().RegisterName("arp-iface-tap3-59:6C:DE:AD:00:03-dead::01", 1, arp_entry)
	num_arps := len(plugin.GetArpCache().GetMapping().ListNames())
	err := plugin.DeleteArp(arp_entry)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(num_arps - 1))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(0))
}

// Test deleting of ARP entry
func TestDeleteArp(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	// add interface
	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	// add arp
	arp_entry := &l3.ArpTable_ArpEntry{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:DE:AD:00:01",
		Static:      true,
	}
	plugin.GetArpIndexes().RegisterName("arp-iface-tap1-59:6C:DE:AD:00:01-192.168.10.21", 1, arp_entry)
	num_arps := len(plugin.GetArpIndexes().GetMapping().ListNames())

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := plugin.DeleteArp(&l3.ArpTable_ArpEntry{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:DE:AD:00:01",
		Static:      true,
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(num_arps - 1))
}

// Test changing of ARP entry
func TestChangeArp(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	// add interface
	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	ifIndexes.GetMapping().RegisterName("tap2", 2, nil)
	// add arp
	arp_entry_old := &l3.ArpTable_ArpEntry{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:DE:AD:00:01",
		Static:      true,
	}
	arp_entry_new := &l3.ArpTable_ArpEntry{
		Interface:   "tap2",
		IpAddress:   "192.168.10.22",
		PhysAddress: "59:6C:DE:AD:00:02",
		Static:      true,
	}
	plugin.GetArpIndexes().RegisterName("arp-iface-tap1-59:6C:DE:AD:00:01-192.168.10.21", 1, arp_entry_old)
	num_arps := len(plugin.GetArpIndexes().GetMapping().ListNames())

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := plugin.ChangeArp(arp_entry_new, arp_entry_old)
	Expect(err).To(BeNil())
	Expect(err).ShouldNot(HaveOccurred())
	arps := plugin.GetArpIndexes().GetMapping().ListNames()
	Expect(arps).To(HaveLen(num_arps))
	Expect(arps[0]).To(Equal("arp-iface-tap2-59:6C:DE:AD:00:02-192.168.10.22"))
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
}

// Test resolving of created ARPs
func TestArpResolveCreatedInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	arp_entry1 := &l3.ArpTable_ArpEntry{
		Interface:   "tap3",
		IpAddress:   "dead::01",
		PhysAddress: "59:6C:DE:AD:00:03",
		Static:      false,
	}
	arp_entry2 := &l3.ArpTable_ArpEntry{
		Interface:   "tap4",
		IpAddress:   "dead::02",
		PhysAddress: "59:6C:DE:AD:00:04",
		Static:      false,
	}

	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(0))

	plugin.GetArpCache().RegisterName("arp-iface-tap3-59:6C:DE:AD:00:03-dead::01", 1, arp_entry1)
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(0))

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	ifIndexes.GetMapping().RegisterName("tap3", 1, arp_entry1)
	err := plugin.ResolveCreatedInterface("tap3")
	Expect(err).ShouldNot(HaveOccurred())

	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(1))
	_, _, found := plugin.GetArpCache().LookupIdx("arp-iface-tap3-59:6C:DE:AD:00:03-dead::01")
	Expect(found).To(BeFalse())
	_, meta, found := plugin.GetArpIndexes().LookupIdx("arp-iface-tap3-59:6C:DE:AD:00:03-dead::01")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	Expect(meta.Interface).To(Equal("tap3"))

	plugin.GetArpIndexes().RegisterName("arp-iface-tap4-59:6C:DE:AD:00:04-dead::02", 2, arp_entry2)
	plugin.GetArpDeleted().RegisterName("arp-iface-tap4-59:6C:DE:AD:00:04-dead::02", 2, arp_entry2)
	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(1))

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	ifIndexes.GetMapping().RegisterName("tap4", 2, arp_entry2)
	err = plugin.ResolveCreatedInterface("tap4")
	Expect(err).ShouldNot(HaveOccurred())

	Expect(plugin.GetArpCache().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpDeleted().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetArpIndexes().GetMapping().ListNames()).To(HaveLen(1))

	_, _, found = plugin.GetArpCache().LookupIdx("arp-iface-tap4-59:6C:DE:AD:00:03-dead::02")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetArpIndexes().LookupIdx("arp-iface-tap4-59:6C:DE:AD:00:03-dead::02")
	Expect(found).To(BeFalse())
}

// Test resolving of created ARPs
func TestArpResolveDeletedInterface(t *testing.T) {
	// Setup
	_, connection, plugin, _ := arpTestSetup(t)
	defer arpTestTeardown(connection, plugin)

	err := plugin.ResolveDeletedInterface("tap4", 3)
	Expect(err).To(BeNil())
}

// ARP Test Setup
func arpTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *l3plugin.ArpConfigurator, ifaceidx.SwIfIndex) {
	RegisterTestingT(t)
	ctx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())

	plugin := &l3plugin.ArpConfigurator{}
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logging.ForPlugin("test-log"), "l3-plugin", nil))

	err = plugin.Init(logging.ForPlugin("test-log"), connection, ifIndexes)
	Expect(err).To(BeNil())

	return ctx, connection, plugin, ifIndexes
}

// Test Teardown
func arpTestTeardown(connection *core.Connection, plugin *l3plugin.ArpConfigurator) {
	connection.Disconnect()
	Expect(plugin.Close()).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}
