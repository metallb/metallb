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

package l2plugin_test

import (
	"testing"

	"git.fd.io/govpp.git/core"

	"git.fd.io/govpp.git/adapter/mock"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l22 "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

func bdConfigTestInitialization(t *testing.T) (*vppcallmock.TestCtx, *core.Connection, ifaceidx.SwIfIndexRW, chan l2plugin.BridgeDomainStateMessage, *l2plugin.BDConfigurator) {
	RegisterTestingT(t)

	// Initialize notification channel
	notifChan := make(chan l2plugin.BridgeDomainStateMessage, 100)

	// Initialize sw if index
	nameToIdxSW := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "ifaceidx_test", ifaceidx.IndexMetadata)
	swIfIndex := ifaceidx.NewSwIfIndex(nameToIdxSW)
	names := nameToIdxSW.ListNames()

	// Check if names were empty
	Expect(names).To(BeEmpty())

	// Create connection
	mockCtx := &vppcallmock.TestCtx{
		MockVpp: mock.NewVppAdapter(),
	}
	connection, err := core.Connect(mockCtx.MockVpp)
	Expect(err).To(BeNil())

	// Create plugin logger
	pluginLogger := logging.ForPlugin("testname")

	// Test initialization
	bdConfiguratorPlugin := &l2plugin.BDConfigurator{}
	err = bdConfiguratorPlugin.Init(pluginLogger, connection, swIfIndex, notifChan)
	Expect(err).To(BeNil())

	return mockCtx, connection, swIfIndex, notifChan, bdConfiguratorPlugin
}

func bdConfigTeardown(conn *core.Connection, plugin *l2plugin.BDConfigurator) {
	conn.Disconnect()
	Expect(plugin.Close()).To(Succeed())
	logging.DefaultRegistry.ClearRegistry()
}

// Tests configuration of bridge domain
func TestBDConfigurator_ConfigureBridgeDomain(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BdIPMacAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	err := plugin.ConfigureBridgeDomain(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	})

	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.BridgeDomain.ArpTerminationTable).To(Not(BeEmpty()))

	table := meta.BridgeDomain.ArpTerminationTable[0]
	Expect(table.IpAddress).To(BeEquivalentTo("192.168.0.1"))
	Expect(table.PhysAddress).To(BeEquivalentTo("01:23:45:67:89:ab"))
}

// Tests modification of bridge domain (recreating it)
func TestBDConfigurator_ModifyBridgeDomainRecreate(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BdIPMacAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	plugin.GetBdIndexes().RegisterName("test", 2, nil)

	err := plugin.ModifyBridgeDomain(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               false,
		UnknownUnicastFlood: false,
		Forward:             false,
		Learn:               false,
		ArpTermination:      false,
		MacAge:              15,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	}, &l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	})

	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.BridgeDomain.Flood).To(BeFalse())
	Expect(meta.BridgeDomain.UnknownUnicastFlood).To(BeFalse())
	Expect(meta.BridgeDomain.Forward).To(BeFalse())
	Expect(meta.BridgeDomain.Learn).To(BeFalse())
	Expect(meta.BridgeDomain.ArpTermination).To(BeFalse())
	Expect(meta.BridgeDomain.MacAge).To(BeEquivalentTo(15))
}

// Tests modification of bridge domain (bridge domain not found)
func TestBDConfigurator_ModifyBridgeDomainNotFound(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BdIPMacAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	err := plugin.ModifyBridgeDomain(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if1",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.2",
				PhysAddress: "01:23:45:67:89:ac",
			},
		},
	}, &l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	})

	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.BridgeDomain.ArpTerminationTable).To(Not(BeEmpty()))

	table := meta.BridgeDomain.ArpTerminationTable[0]
	Expect(table.IpAddress).To(BeEquivalentTo("192.168.0.2"))
	Expect(table.PhysAddress).To(BeEquivalentTo("01:23:45:67:89:ac"))
}

// Tests modification of bridge domain (bridge domain already present)
func TestBDConfigurator_ModifyBridgeDomainFound(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BdIPMacAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BdIPMacAddDelReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	plugin.GetBdIndexes().RegisterName("test", 0, l2idx.NewBDMetadata(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	}, []string{"if0"}))

	err := plugin.ModifyBridgeDomain(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.2",
				PhysAddress: "01:23:45:67:89:ac",
			},
		},
	}, &l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: false,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	})

	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.BridgeDomain.ArpTerminationTable).To(Not(BeEmpty()))

	table := meta.BridgeDomain.ArpTerminationTable[0]
	Expect(table.IpAddress).To(BeEquivalentTo("192.168.0.2"))
	Expect(table.PhysAddress).To(BeEquivalentTo("01:23:45:67:89:ac"))
}

// Tests deletion of bridge domain
func TestBDConfigurator_DeleteBridgeDomain(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})

	plugin.GetBdIndexes().RegisterName("test", 0, l2idx.NewBDMetadata(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	}, []string{"if0"}))

	err := plugin.DeleteBridgeDomain(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	})

	Expect(err).To(BeNil())

	_, _, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeFalse())
}

// Tests resolving of created interface (not found)
func TestBDConfigurator_ResolveCreatedInterfaceNotFound(t *testing.T) {
	_, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	err := plugin.ResolveCreatedInterface("test", 0)
	Expect(err).To(BeNil())
}

// Tests resolving of created interface (present)
func TestBDConfigurator_ResolveCreatedInterfaceFound(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{
		BdID:         1,
		Flood:        1,
		UuFlood:      2,
		Forward:      3,
		Learn:        4,
		ArpTerm:      5,
		MacAge:       6,
		BdTag:        []byte("test"),
		BviSwIfIndex: 1,
		NSwIfs:       1,
		SwIfDetails: []l22.BridgeDomainSwIf{
			{
				SwIfIndex: 1,
				Context:   0,
				Shg:       20,
			},
		},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	plugin.GetBdIndexes().RegisterName("test", 0, l2idx.NewBDMetadata(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "test",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	}, []string{"test"}))

	err := plugin.ResolveCreatedInterface("test", 0)
	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(Not(BeEmpty()))
	Expect(meta.ConfiguredInterfaces[0]).To(Equal("test"))
}

// Tests checks that calling Resolve twice with the same interface registers it to the metadata only once
func TestBDConfigurator_ResolveCreatedInterfaceDuplicated(t *testing.T) {
	ctx, conn, ifIndexes, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	// Register bridge domain (as created)
	plugin.GetBdIndexes().RegisterName("bd1", 1, &l2idx.BdMetadata{
		BridgeDomain: &l2.BridgeDomains_BridgeDomain{
			Name: "bd1",
			Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
				{
					Name: "if1",
				},
			},
		},
	})

	// Register interface (as created)
	ifIndexes.RegisterName("if1", 1, nil) // Meta is not needed

	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})

	// 1)
	err := plugin.ResolveCreatedInterface("if1", 1)
	Expect(err).To(BeNil())
	_, meta, found := plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(HaveLen(1))

	// 2)
	err = plugin.ResolveCreatedInterface("if1", 1)
	Expect(err).To(BeNil())
	_, meta, found = plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(HaveLen(1))
}

// Tests resolving of deleted interface (not found)
func TestBDConfigurator_ResolveDeletedInterfaceNotFound(t *testing.T) {
	_, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	err := plugin.ResolveDeletedInterface("test")
	Expect(err).To(BeNil())
}

// Tests resolving of deleted interface (present)
func TestBDConfigurator_ResolveDeletedInterfaceFound(t *testing.T) {
	ctx, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{
		BdID:         1,
		Flood:        1,
		UuFlood:      2,
		Forward:      3,
		Learn:        4,
		ArpTerm:      5,
		MacAge:       6,
		BdTag:        []byte("test"),
		BviSwIfIndex: 1,
		NSwIfs:       1,
		SwIfDetails: []l22.BridgeDomainSwIf{
			{
				SwIfIndex: 1,
				Context:   0,
				Shg:       20,
			},
		},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	plugin.GetBdIndexes().RegisterName("test", 0, l2idx.NewBDMetadata(&l2.BridgeDomains_BridgeDomain{
		Name:                "test",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		ArpTermination:      true,
		MacAge:              20,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "test",
				BridgedVirtualInterface: true,
				SplitHorizonGroup:       1,
			},
		},
		ArpTerminationTable: []*l2.BridgeDomains_BridgeDomain_ArpTerminationEntry{
			{
				IpAddress:   "192.168.0.1",
				PhysAddress: "01:23:45:67:89:ab",
			},
		},
	}, []string{"test"}))

	err := plugin.ResolveDeletedInterface("test")
	Expect(err).To(BeNil())

	_, meta, found := plugin.GetBdIndexes().LookupIdx("test")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(BeEmpty())
}

// Tests configuration and modification of bridge domain with 4 interfaces
func TestBDConfigurator_FourInterfacesModify(t *testing.T) {
	ctx, conn, index, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	// Register interfaces to index
	index.RegisterName("if0", 0, &interfaces.Interfaces_Interface{
		Name:        "if0",
		IpAddresses: []string{"192.168.1.10"},
	})

	index.RegisterName("if1", 1, &interfaces.Interfaces_Interface{
		Name:        "if1",
		IpAddresses: []string{"192.168.1.11"},
	})

	index.RegisterName("if2", 2, &interfaces.Interfaces_Interface{
		Name:        "if2",
		IpAddresses: []string{"192.168.1.12"},
	})

	index.RegisterName("if3", 3, &interfaces.Interfaces_Interface{
		Name:        "if3",
		IpAddresses: []string{"192.168.1.13"},
	})

	index.RegisterName("if4", 4, &interfaces.Interfaces_Interface{
		Name:        "if4",
		IpAddresses: []string{"192.168.1.14"},
	})

	index.RegisterName("if5", 5, &interfaces.Interfaces_Interface{
		Name:        "if5",
		IpAddresses: []string{"192.168.1.15"},
	})

	// Mock replies to creation of bridge domain
	ctx.MockVpp.MockReply(&l22.BridgeDomainAddDelReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})

	// Create bridge domain
	bdData := &l2.BridgeDomains_BridgeDomain{
		Name:                "bd1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
			},
			{
				Name: "if1",
			},
			{
				Name: "if2",
			},
			{
				Name: "if3",
			},
		},
	}

	err := plugin.ConfigureBridgeDomain(bdData)
	Expect(err).To(BeNil())

	// Check for correct metadata after creation
	_, meta, found := plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(HaveLen(4))

	// Mock replies to modification of bridge domain
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})

	// Modify bridge domain
	oldBdData := bdData
	bdData = &l2.BridgeDomains_BridgeDomain{
		Name:                "bd1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if2",
			},
			{
				Name: "if3",
			},
			{
				Name: "if4",
				BridgedVirtualInterface: true,
			},
			{
				Name: "if5",
			},
		},
	}

	err = plugin.ModifyBridgeDomain(bdData, oldBdData)
	Expect(err).To(BeNil())

	// Check for correct metadata after modification
	_, meta, found = plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(HaveLen(4))

	// Mock replies to modification (BVI move) of bridge domain
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.BridgeDomainDetails{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l22.SwInterfaceSetL2BridgeReply{})

	// Modify bridge domain (BVI move interface)
	oldBdData = bdData
	bdData = &l2.BridgeDomains_BridgeDomain{
		Name:                "bd1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if2",
			},
			{
				Name: "if3",
			},
			{
				Name: "if4",
			},
			{
				Name: "if5",
				BridgedVirtualInterface: true,
			},
		},
	}

	err = plugin.ModifyBridgeDomain(bdData, oldBdData)
	Expect(err).To(BeNil())

	// Check for correct metadata after modification
	_, meta, found = plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeTrue())
	Expect(meta.ConfiguredInterfaces).To(HaveLen(4))
	Expect(meta.ConfiguredInterfaces[3]).To(Equal("if5"))
}

// Tests invalid bridge domain with 2 BVI interfaces
func TestBDConfigurator_TwoBVI(t *testing.T) {
	_, conn, _, _, plugin := bdConfigTestInitialization(t)
	defer bdConfigTeardown(conn, plugin)

	// Create incorrect domain
	bdData := &l2.BridgeDomains_BridgeDomain{
		Name:                "bd1",
		Flood:               true,
		UnknownUnicastFlood: true,
		Forward:             true,
		Learn:               true,
		Interfaces: []*l2.BridgeDomains_BridgeDomain_Interfaces{
			{
				Name: "if0",
				BridgedVirtualInterface: true,
			},
			{
				Name: "if1",
				BridgedVirtualInterface: true,
			},
		},
	}

	err := plugin.ConfigureBridgeDomain(bdData)
	Expect(err).ToNot(BeNil())

	// Check for missing index after failed creation
	_, _, found := plugin.GetBdIndexes().LookupIdx("bd1")
	Expect(found).To(BeFalse())
}
