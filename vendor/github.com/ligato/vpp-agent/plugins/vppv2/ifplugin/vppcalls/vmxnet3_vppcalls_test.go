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

package vppcalls_test

import (
	"testing"

	ifModel "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vmxnet3"
	. "github.com/onsi/gomega"
)

func TestAddVmxNet3Interface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&vmxnet3.Vmxnet3CreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	swIfIdx, err := ifHandler.AddVmxNet3("vmxnet3-face/be/1c/4", &ifModel.VmxNet3Link{
		EnableElog: true,
		RxqSize:    2048,
		TxqSize:    512,
	})
	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*vmxnet3.Vmxnet3Create)
		if ok {
			Expect(vppMsg.PciAddr).To(BeEquivalentTo(2629761742))
			Expect(vppMsg.EnableElog).To(BeEquivalentTo(1))
			Expect(vppMsg.RxqSize).To(BeEquivalentTo(2048))
			Expect(vppMsg.TxqSize).To(BeEquivalentTo(512))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestAddVmxNet3InterfacePCIErr(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&vmxnet3.Vmxnet3CreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	// Name in incorrect format
	_, err := ifHandler.AddVmxNet3("vmxnet3-a/14/19", nil)
	Expect(err).ToNot(BeNil())
}

func TestAddVmxNet3InterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&vmxnet3.Vmxnet3CreateReply{
		Retval: 1,
	})

	_, err := ifHandler.AddVmxNet3("vmxnet3-a/14/19/1e", nil)
	Expect(err).ToNot(BeNil())
}

func TestDelVmxNet3Interface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&vmxnet3.Vmxnet3DeleteReply{
		Retval: 0,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteVmxNet3("vmxnet3-a/14/19/1e", 1)
	Expect(err).To(BeNil())
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*vmxnet3.Vmxnet3Delete)
		if ok {
			Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestDelVmxNet3InterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&vmxnet3.Vmxnet3DeleteReply{
		Retval: 1,
	})

	err := ifHandler.DeleteVmxNet3("vmxnet3-a/14/19/1e", 1)
	Expect(err).ToNot(BeNil())
}
