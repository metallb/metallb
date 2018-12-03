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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tap"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/tapv2"
	ifModel "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	. "github.com/onsi/gomega"
)

func TestAddTapInterface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapConnectReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	swIfIdx, err := ifHandler.AddTapInterface("tapIf", &ifModel.Interfaces_Interface_Tap{
		Version:    1,
		HostIfName: "hostIf",
		Namespace:  "ns1",
		RxRingSize: 1,
		TxRingSize: 1,
	})
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*tap.TapConnect)
		if ok {
			Expect(vppMsg.UseRandomMac).To(BeEquivalentTo(1))
			Expect(vppMsg.TapName).To(BeEquivalentTo([]byte("hostIf")))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestAddTapInterfaceV2(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tapv2.TapCreateV2Reply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	swIfIdx, err := ifHandler.AddTapInterface("tapIf", &ifModel.Interfaces_Interface_Tap{
		Version:    2,
		HostIfName: "hostIf",
		Namespace:  "ns1",
		RxRingSize: 1,
		TxRingSize: 1,
	})
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*tapv2.TapCreateV2)
		if ok {
			Expect(vppMsg.UseRandomMac).To(BeEquivalentTo(1))
			Expect(vppMsg.HostIfName).To(BeEquivalentTo([]byte("hostIf")))
			Expect(vppMsg.HostNamespace).To(BeEquivalentTo([]byte("ns1")))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestAddTapInterfaceNoInput(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapConnectReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	_, err := ifHandler.AddTapInterface("tapIf", nil)
	Expect(err).ToNot(BeNil())
}

func TestAddTapInterfaceError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapConnect{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	_, err := ifHandler.AddTapInterface("tapIf", &ifModel.Interfaces_Interface_Tap{
		Version:    1,
		HostIfName: "hostIf",
		Namespace:  "ns1",
		RxRingSize: 1,
		TxRingSize: 1,
	})
	Expect(err).ToNot(BeNil())
}

func TestAddTapInterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapConnectReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	_, err := ifHandler.AddTapInterface("tapIf", &ifModel.Interfaces_Interface_Tap{
		Version:    1,
		HostIfName: "hostIf",
		Namespace:  "ns1",
		RxRingSize: 1,
		TxRingSize: 1,
	})
	Expect(err).ToNot(BeNil())
}

func TestDeleteTapInterface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteTapInterface("tapIf", 1, 1)
	Expect(err).To(BeNil())
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*tap.TapDelete)
		if ok {
			Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestDeleteTapInterfaceV2(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tapv2.TapDeleteV2Reply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteTapInterface("tapIf", 1, 2)
	Expect(err).To(BeNil())
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*tapv2.TapDeleteV2)
		if ok {
			Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestDeleteTapInterfaceError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapDelete{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteTapInterface("tapIf", 1, 1)
	Expect(err).ToNot(BeNil())
}

func TestDeleteTapInterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&tap.TapDeleteReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteTapInterface("tapIf", 1, 1)
	Expect(err).ToNot(BeNil())
}
