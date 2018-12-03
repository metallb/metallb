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

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	l2ba "github.com/ligato/vpp-agent/plugins/vpp/binapi/l2"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	. "github.com/onsi/gomega"
)

func TestSetInterfacesToBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose
	swIfIndexes.RegisterName("if2", 2, nil)
	swIfIndexes.RegisterName("if3", 3, nil)

	configured, err := bdHandler.SetInterfacesToBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
			BridgedVirtualInterface: true,
			SplitHorizonGroup:       0,
		},
		{
			Name: "if2",
			BridgedVirtualInterface: false,
			SplitHorizonGroup:       1,
		},
		{
			Name: "if3",
			BridgedVirtualInterface: false,
			SplitHorizonGroup:       2,
		},
	}, swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(3))
	for i, msg := range ctx.MockChannel.Msgs {
		portType := l2ba.L2_API_PORT_TYPE_NORMAL
		if i == 0 {
			portType = l2ba.L2_API_PORT_TYPE_BVI
		}
		Expect(msg).To(Equal(&l2ba.SwInterfaceSetL2Bridge{
			RxSwIfIndex: uint32(i + 1),
			BdID:        1,
			Shg:         uint8(i),
			PortType:    portType,
			Enable:      1,
		}))
	}
	Expect(configured).To(HaveLen(3))
}

func TestSetInterfacesToBridgeDomainNoInterfaceToSet(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))

	configured, err := bdHandler.SetInterfacesToBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{},
		swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(0))
	Expect(configured).To(BeNil())
}

func TestSetInterfacesToBridgeDomainMissingInterface(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose
	// Interface "if2" is not registered

	configured, err := bdHandler.SetInterfacesToBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
		{
			Name: "if2",
		},
	}, swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(1))
	Expect(configured).To(HaveLen(1))
}

func TestSetInterfacesToBridgeDomainError(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2Bridge{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose

	configured, err := bdHandler.SetInterfacesToBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
	}, swIfIndexes)

	Expect(err).ToNot(BeNil())
	Expect(configured).To(BeNil())
}

func TestSetInterfacesToBridgeDomainRetval(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{
		Retval: 1,
	})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose

	configured, err := bdHandler.SetInterfacesToBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
	}, swIfIndexes)

	Expect(err).ToNot(BeNil())
	Expect(configured).To(BeNil())
}

func TestUnsetInterfacesFromBridgeDomain(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose
	swIfIndexes.RegisterName("if2", 2, nil)
	swIfIndexes.RegisterName("if3", 3, nil)

	configured, err := bdHandler.UnsetInterfacesFromBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name:              "if1",
			SplitHorizonGroup: 0,
		},
		{
			Name:              "if2",
			SplitHorizonGroup: 1,
		},
		{
			Name:              "if3",
			SplitHorizonGroup: 2,
		},
	}, swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(3))
	for i, msg := range ctx.MockChannel.Msgs {
		Expect(msg).To(Equal(&l2ba.SwInterfaceSetL2Bridge{
			RxSwIfIndex: uint32(i + 1),
			BdID:        1,
			Shg:         uint8(i),
			Enable:      0,
		}))
	}
	Expect(configured).To(HaveLen(3))
}

func TestUnsetInterfacesFromBridgeDomainNoInterfaceToUnset(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))

	configured, err := bdHandler.UnsetInterfacesFromBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{},
		swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(0))
	Expect(configured).To(BeNil())
}

func TestUnsetInterfacesFromBridgeDomainMissingInterface(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})
	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose
	// Interface "if2" is not registered

	configured, err := bdHandler.UnsetInterfacesFromBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
		{
			Name: "if2",
		},
	}, swIfIndexes)

	Expect(err).To(BeNil())
	Expect(len(ctx.MockChannel.Msgs)).To(BeEquivalentTo(1))
	// Both must be marked as un-configured
	Expect(configured).To(HaveLen(2))
}

func TestUnsetInterfacesFromBridgeDomainError(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2Bridge{})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose

	configured, err := bdHandler.UnsetInterfacesFromBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
	}, swIfIndexes)

	Expect(err).ToNot(BeNil())
	Expect(configured).To(BeNil())
}

func TestUnsetInterfacesFromBridgeDomainRetval(t *testing.T) {
	ctx, bdHandler, _ := bdTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&l2ba.SwInterfaceSetL2BridgeReply{
		Retval: 1,
	})

	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd", nil))
	swIfIndexes.RegisterName("if1", 1, nil) // Metadata are not required for test purpose

	configured, err := bdHandler.UnsetInterfacesFromBridgeDomain("bd1", 1, []*l2.BridgeDomains_BridgeDomain_Interfaces{
		{
			Name: "if1",
		},
	}, swIfIndexes)

	Expect(err).ToNot(BeNil())
	Expect(configured).To(BeNil())
}
