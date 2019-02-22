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

package vppcalls_test

import (
	"testing"

	govppapi "git.fd.io/govpp.git/api"
	l2nb "github.com/ligato/vpp-agent/api/models/vpp/l2"
	"github.com/ligato/vpp-agent/pkg/idxvpp2"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/l2plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var testDataInMessagesBDs = []govppapi.Message{
	&l2ba.BridgeDomainDetails{
		BdID:  4,
		Flood: 1, UuFlood: 1, Forward: 1, Learn: 1, ArpTerm: 1, MacAge: 140,
		SwIfDetails: []l2ba.BridgeDomainSwIf{
			{SwIfIndex: 5},
			{SwIfIndex: 7},
		},
	},
	&l2ba.BridgeDomainDetails{
		BdID:  5,
		Flood: 0, UuFlood: 0, Forward: 0, Learn: 0, ArpTerm: 0, MacAge: 141,
		SwIfDetails: []l2ba.BridgeDomainSwIf{
			{SwIfIndex: 5},
			{SwIfIndex: 8},
		},
	},
}

var testDataOutMessage = []*vppcalls.BridgeDomainDetails{
	{
		Bd: &l2nb.BridgeDomain{
			Flood:               true,
			UnknownUnicastFlood: true,
			Forward:             true,
			Learn:               true,
			ArpTermination:      true,
			MacAge:              140,
			Interfaces: []*l2nb.BridgeDomain_Interface{
				{
					Name: "if1",
				},
				{
					Name: "if2",
				},
			},
		},
		Meta: &vppcalls.BridgeDomainMeta{
			BdID: 4,
		},
	}, {
		Bd: &l2nb.BridgeDomain{
			Flood:               false,
			UnknownUnicastFlood: false,
			Forward:             false,
			Learn:               false,
			ArpTermination:      false,
			MacAge:              141,
			Interfaces: []*l2nb.BridgeDomain_Interface{
				{
					Name: "if1",
				},
				{
					Name: "if3",
				},
			},
			ArpTerminationTable: []*l2nb.BridgeDomain_ArpTerminationEntry{
				{
					IpAddress:   "192.168.0.1",
					PhysAddress: "aa:aa:aa:aa:aa:aa",
				},
			},
		},
		Meta: &vppcalls.BridgeDomainMeta{
			BdID: 5,
		},
	},
}

// TestDumpBridgeDomains tests DumpBridgeDomains method
func TestDumpBridgeDomains(t *testing.T) {
	ctx, bdHandler, ifIndexes := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 5})
	ifIndexes.Put("if2", &ifaceidx.IfaceMetadata{SwIfIndex: 7})

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name:    (&l2ba.BdIPMacDump{}).GetMessageName(),
			Ping:    true,
			Message: &l2ba.BdIPMacDetails{},
		},
		{
			Name:    (&l2ba.BridgeDomainDump{}).GetMessageName(),
			Ping:    true,
			Message: testDataInMessagesBDs[0],
		},
	})

	bridgeDomains, err := bdHandler.DumpBridgeDomains()

	Expect(err).To(BeNil())
	Expect(bridgeDomains).To(HaveLen(1))
	Expect(bridgeDomains[0]).To(Equal(testDataOutMessage[0]))

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	_, err = bdHandler.DumpBridgeDomains()
	Expect(err).Should(HaveOccurred())
}

// TestDumpBridgeDomains tests DumpBridgeDomains method
func TestDumpBridgeDomainsWithARP(t *testing.T) {
	ctx, bdHandler, ifIndexes := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 5})
	ifIndexes.Put("if3", &ifaceidx.IfaceMetadata{SwIfIndex: 8})

	ctx.MockReplies([]*vppcallmock.HandleReplies{
		{
			Name: (&l2ba.BdIPMacDump{}).GetMessageName(),
			Ping: true,
			Message: &l2ba.BdIPMacDetails{
				BdID:       5,
				IsIPv6:     0,
				IPAddress:  []byte{192, 168, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				MacAddress: []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA},
			},
		},
		{
			Name:    (&l2ba.BridgeDomainDump{}).GetMessageName(),
			Ping:    true,
			Message: testDataInMessagesBDs[1],
		},
	})

	bridgeDomains, err := bdHandler.DumpBridgeDomains()

	Expect(err).To(BeNil())
	Expect(bridgeDomains).To(HaveLen(1))
	Expect(bridgeDomains[0]).To(Equal(testDataOutMessage[1]))

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	_, err = bdHandler.DumpBridgeDomains()
	Expect(err).Should(HaveOccurred())
}

var testDataInMessagesFIBs = []govppapi.Message{
	&l2ba.L2FibTableDetails{
		BdID:   10,
		Mac:    []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xAA, 0xAA},
		BviMac: 0, SwIfIndex: ^uint32(0), FilterMac: 1, StaticMac: 0,
	},
	&l2ba.L2FibTableDetails{
		BdID:   20,
		Mac:    []byte{0xBB, 0xBB, 0xBB, 0xBB, 0xBB, 0xBB},
		BviMac: 1, SwIfIndex: 1, FilterMac: 0, StaticMac: 1,
	},
}

var testDataOutFIBs = []*vppcalls.FibTableDetails{
	{
		Fib: &l2nb.FIBEntry{
			PhysAddress:             "aa:aa:aa:aa:aa:aa",
			BridgeDomain:            "bd1",
			Action:                  l2nb.FIBEntry_DROP,
			StaticConfig:            false,
			BridgedVirtualInterface: false,
			OutgoingInterface:       "",
		},
		Meta: &vppcalls.FibMeta{
			BdID:  10,
			IfIdx: ^uint32(0),
		},
	},
	{
		Fib: &l2nb.FIBEntry{
			PhysAddress:             "bb:bb:bb:bb:bb:bb",
			BridgeDomain:            "bd2",
			Action:                  l2nb.FIBEntry_FORWARD,
			StaticConfig:            true,
			BridgedVirtualInterface: true,
			OutgoingInterface:       "if1",
		},
		Meta: &vppcalls.FibMeta{
			BdID:  20,
			IfIdx: 1,
		},
	},
}

// Scenario:
// - 2 FIB entries in VPP
// TestDumpFIBTableEntries tests DumpFIBTableEntries method
func TestDumpFIBTableEntries(t *testing.T) {
	ctx, fibHandler, ifIndexes, bdIndexes := fibTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})
	bdIndexes.Put("bd1", &idxvpp2.OnlyIndex{Index: 10})
	bdIndexes.Put("bd2", &idxvpp2.OnlyIndex{Index: 20})

	ctx.MockVpp.MockReply(testDataInMessagesFIBs...)
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	fibTable, err := fibHandler.DumpL2FIBs()
	Expect(err).To(BeNil())
	Expect(fibTable).To(HaveLen(2))
	Expect(fibTable["aa:aa:aa:aa:aa:aa"]).To(Equal(testDataOutFIBs[0]))
	Expect(fibTable["bb:bb:bb:bb:bb:bb"]).To(Equal(testDataOutFIBs[1]))

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	_, err = fibHandler.DumpL2FIBs()
	Expect(err).Should(HaveOccurred())
}

var testDataInXConnect = []govppapi.Message{
	&l2ba.L2XconnectDetails{
		RxSwIfIndex: 1,
		TxSwIfIndex: 2,
	},
	&l2ba.L2XconnectDetails{
		RxSwIfIndex: 3,
		TxSwIfIndex: 4,
	},
}

var testDataOutXconnect = []*vppcalls.XConnectDetails{
	{
		Xc: &l2nb.XConnectPair{
			ReceiveInterface:  "if1",
			TransmitInterface: "if2",
		},
		Meta: &vppcalls.XcMeta{
			ReceiveInterfaceSwIfIdx:  1,
			TransmitInterfaceSwIfIdx: 2,
		},
	},
	{
		Xc: &l2nb.XConnectPair{
			ReceiveInterface:  "if3",
			TransmitInterface: "if4",
		},
		Meta: &vppcalls.XcMeta{
			ReceiveInterfaceSwIfIdx:  3,
			TransmitInterfaceSwIfIdx: 4,
		},
	},
}

// Scenario:
// - 2 Xconnect entries in VPP
// TestDumpXConnectPairs tests DumpXConnectPairs method
func TestDumpXConnectPairs(t *testing.T) {
	ctx, xcHandler, ifIndex := xcTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndex.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})
	ifIndex.Put("if2", &ifaceidx.IfaceMetadata{SwIfIndex: 2})
	ifIndex.Put("if3", &ifaceidx.IfaceMetadata{SwIfIndex: 3})
	ifIndex.Put("if4", &ifaceidx.IfaceMetadata{SwIfIndex: 4})

	ctx.MockVpp.MockReply(testDataInXConnect...)
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	xConnectPairs, err := xcHandler.DumpXConnectPairs()

	Expect(err).To(BeNil())
	Expect(xConnectPairs).To(HaveLen(2))
	Expect(xConnectPairs[1]).To(Equal(testDataOutXconnect[0]))
	Expect(xConnectPairs[3]).To(Equal(testDataOutXconnect[1]))

	ctx.MockVpp.MockReply(&l2ba.BridgeDomainAddDelReply{})
	_, err = xcHandler.DumpXConnectPairs()
	Expect(err).Should(HaveOccurred())
}
