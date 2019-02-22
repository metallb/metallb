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

	. "github.com/onsi/gomega"

	l2 "github.com/ligato/vpp-agent/api/models/vpp/l2"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
)

func TestAddInterfaceToBridgeDomain(t *testing.T) {
	ctx, bdHandler, ifaceIdx := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	err := bdHandler.AddInterfaceToBridgeDomain(1, &l2.BridgeDomain_Interface{
		Name: "if1",
		BridgedVirtualInterface: true,
		SplitHorizonGroup:       0,
	})

	Expect(err).To(BeNil())
	Expect(ctx.MockChannel.Msgs).To(HaveLen(1))
	msg := ctx.MockChannel.Msgs[0]
	Expect(msg).To(Equal(&l2ba.SwInterfaceSetL2Bridge{
		RxSwIfIndex: uint32(1),
		BdID:        1,
		Shg:         uint8(0),
		PortType:    l2ba.L2_API_PORT_TYPE_BVI,
		Enable:      1,
	}))
}

func TestAddMissingInterfaceToBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	// missing: ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	err := bdHandler.AddInterfaceToBridgeDomain(1, &l2.BridgeDomain_Interface{
		Name: "if1",
		BridgedVirtualInterface: true,
		SplitHorizonGroup:       0,
	})

	Expect(err).ToNot(BeNil())
	Expect(ctx.MockChannel.Msgs).To(BeEmpty())
}

func TestAddInterfaceToBridgeDomainWithError(t *testing.T) {
	ctx, bdHandler, ifaceIdx := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2Bridge{}) // wrong reply message type
	err := bdHandler.AddInterfaceToBridgeDomain(1, &l2.BridgeDomain_Interface{
		Name: "if1",
		BridgedVirtualInterface: true,
		SplitHorizonGroup:       0,
	})

	Expect(err).ToNot(BeNil())
	Expect(ctx.MockChannel.Msgs).To(HaveLen(1))
}

func TestAddInterfaceToBridgeDomainWithNonZeroRetval(t *testing.T) {
	ctx, bdHandler, ifaceIdx := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{
		Retval: 1,
	})
	err := bdHandler.AddInterfaceToBridgeDomain(1, &l2.BridgeDomain_Interface{
		Name: "if1",
		BridgedVirtualInterface: true,
		SplitHorizonGroup:       0,
	})

	Expect(err).ToNot(BeNil())
	Expect(ctx.MockChannel.Msgs).To(HaveLen(1))
}

func TestDeleteInterfaceFromBridgeDomain(t *testing.T) {
	ctx, bdHandler, ifaceIdx := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 10})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	err := bdHandler.DeleteInterfaceFromBridgeDomain(4, &l2.BridgeDomain_Interface{
		Name:              "if1",
		SplitHorizonGroup: 12,
	})

	Expect(err).To(BeNil())
	Expect(ctx.MockChannel.Msgs).To(HaveLen(1))
	msg := ctx.MockChannel.Msgs[0]
	Expect(msg).To(Equal(&l2ba.SwInterfaceSetL2Bridge{
		RxSwIfIndex: uint32(10),
		BdID:        4,
		Shg:         uint8(12),
		PortType:    l2ba.L2_API_PORT_TYPE_NORMAL,
		Enable:      0,
	}))
}

func TestDeleteMissingInterfaceFromBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	// missing: ifaceIdx.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 10})

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	err := bdHandler.DeleteInterfaceFromBridgeDomain(4, &l2.BridgeDomain_Interface{
		Name:              "if1",
		SplitHorizonGroup: 12,
	})

	Expect(err).ToNot(BeNil())
	Expect(ctx.MockChannel.Msgs).To(BeEmpty())
}
