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

// Test adding of ARP proxy entry
func TestAddInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	ifIndexes.GetMapping().RegisterName("tap2", 2, nil)

	err := plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf1",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: ""}},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(0))
	err = plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))

	err = plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf4",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap1"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(2))
}

// Test deleting of ARP proxy entry
func TestDeleteInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	ifIndexes.GetMapping().RegisterName("tap2", 2, nil)

	plugin.GetArpIfIndexes().RegisterName("proxyArpIf1", 1, nil)
	plugin.GetArpIfIndexes().RegisterName("proxyArpIf2", 2, nil)
	plugin.GetArpIfIndexes().RegisterName("proxyArpIf3", 3, nil)
	plugin.GetArpIfIndexes().RegisterName("proxyArpIf4", 4, nil)
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(4))

	err := plugin.DeleteInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf1",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(3))

	err = plugin.DeleteInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	})
	Expect(err).Should(HaveOccurred())
	Expect(err).To(Not(BeNil()))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(3))

	err = plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))

	err = plugin.DeleteInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(2))
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = plugin.DeleteInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf4",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap1"}},
	})
	Expect(err).To(BeNil())
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
}

// Test deleting of ARP proxy entry
func TestModifyInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)
	ifIndexes.GetMapping().RegisterName("tap2", 2, nil)

	err := plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))

	// old int deleted from cache
	err = plugin.ModifyInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	}, &l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))

	err = plugin.ModifyInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	}, &l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf4",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap1"}},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))

	// new int added to cache
	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = plugin.ModifyInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	}, &l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()[0]).To(Equal("tap3"))

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = plugin.ModifyInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf3",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap2"}},
	}, &l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf4",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap1"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
}

// Test adding of ARP proxy range
func TestAddRange(t *testing.T) {
	// Setup
	ctx, connection, plugin, _ := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(0))
	err := plugin.AddRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIf1",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
			{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
		},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	// err cases
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = plugin.AddRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIf1",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
			{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
		},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	err = plugin.AddRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIfErr",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.20.0/24/32", LastIp: "124.168.30.10"},
			{FirstIp: "124.168.20.5", LastIp: "124.168.30.5/16/24"},
		},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))
}

// Test deleting of ARP proxy range
func TestDeleteRange(t *testing.T) {
	// Setup
	ctx, connection, plugin, _ := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	plugin.GetArpRngIndexes().RegisterName("proxyArpIf1", 1, nil)
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err := plugin.DeleteRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIf1",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
			{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
		},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(0))

	plugin.GetArpRngIndexes().RegisterName("proxyArpIf1", 1, nil)
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = plugin.DeleteRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIf1",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
			{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
		},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	err = plugin.DeleteRange(&l3.ProxyArpRanges_RangeList{
		Label: "proxyArpIfErr",
		Ranges: []*l3.ProxyArpRanges_RangeList_Range{
			{FirstIp: "124.168.20.0/24/32", LastIp: "124.168.30.10"},
			{FirstIp: "124.168.20.5", LastIp: "124.168.30.5/16/24"},
		},
	})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))
}

// Test modification of ARP proxy range
func TestModifyRange1(t *testing.T) {
	// Setup
	ctx, connection, plugin, _ := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	plugin.GetArpRngIndexes().RegisterName("proxyArpIf1", 1, nil)
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	err := plugin.ModifyRange(
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf2",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "172.154.100.0/24", LastIp: "172.154.200.0/24"},
			},
		},
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf1",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
			},
		})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = plugin.ModifyRange(
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf2",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "172.154.100.0/24", LastIp: "172.154.200.0/24"},
			},
		},
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf1",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
			},
		})
	Expect(err).Should(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpRngIndexes().ListNames()).To(HaveLen(1))

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = plugin.ModifyRange(
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf2",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "172.154.100.0/24", LastIp: "172.154.200.0/24"},
			},
		},
		&l3.ProxyArpRanges_RangeList{
			Label: "proxyArpIf1",
			Ranges: []*l3.ProxyArpRanges_RangeList_Range{
				{FirstIp: "124.168.10.5", LastIp: "124.168.10.10"},
				{FirstIp: "124.168.20.0/24", LastIp: "124.168.20.0/24"},
			},
		})
	Expect(err).ShouldNot(HaveOccurred())
}

// Test resolution of new registered interface for proxy ARP
func TestArpProxyResolveCreatedInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, _ := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	err := plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf2",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap3"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()[0]).To(Equal("tap3"))

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = plugin.ResolveCreatedInterface("tap3", 1)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
}

// Test resolution of new registered interface for proxy ARP
func TestArpProxyResolveDeletedInterface(t *testing.T) {
	// Setup
	ctx, connection, plugin, ifIndexes := proxyarpTestSetup(t)
	defer proxyarpTestTeardown(connection, plugin)

	ifIndexes.GetMapping().RegisterName("tap1", 1, nil)

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err := plugin.AddInterface(&l3.ProxyArpInterfaces_InterfaceList{
		Label:      "proxyArpIf4",
		Interfaces: []*l3.ProxyArpInterfaces_InterfaceList_Interface{{Name: "tap1"}},
	})
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(0))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = plugin.ResolveDeletedInterface("proxyArpIf4")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetArpIfCache()).To(HaveLen(1))
	Expect(plugin.GetArpIfCache()[0]).To(Equal("proxyArpIf4"))
	Expect(plugin.GetArpIfIndexes().ListNames()).To(HaveLen(1))
}

// Test Setup
func proxyarpTestSetup(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, *l3plugin.ProxyArpConfigurator, ifaceidx.SwIfIndex) {
	RegisterTestingT(t)
	ctx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(ctx.MockVpp)
	Expect(err).ShouldNot(HaveOccurred())

	plugin := &l3plugin.ProxyArpConfigurator{}
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logging.ForPlugin("test-log"), "l3-plugin", nil))

	err = plugin.Init(logging.ForPlugin("test-log"), connection, ifIndexes)
	Expect(err).To(BeNil())

	return ctx, connection, plugin, ifIndexes
}

// Test Teardown
func proxyarpTestTeardown(connection *core.Connection, plugin *l3plugin.ProxyArpConfigurator) {
	connection.Disconnect()
	Expect(plugin.Close()).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}
