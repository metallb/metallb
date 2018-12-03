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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/memif"
	ifModel "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	. "github.com/onsi/gomega"
)

func TestAddMasterMemifInterface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	swIfIdx, err := ifHandler.AddMemifInterface("memif", &ifModel.Interfaces_Interface_Memif{
		Id:     1,
		Mode:   ifModel.Interfaces_Interface_Memif_IP,
		Secret: "secret",
		Master: true,
	}, 5)

	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*memif.MemifCreate)
		if ok {
			Expect(vppMsg.ID).To(BeEquivalentTo(1))
			Expect(vppMsg.Mode).To(BeEquivalentTo(1))
			Expect(vppMsg.Role).To(BeEquivalentTo(0))
			Expect(vppMsg.SocketID).To(BeEquivalentTo(5))
			Expect(vppMsg.RxQueues).To(BeEquivalentTo(1))
			Expect(vppMsg.TxQueues).To(BeEquivalentTo(1))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestAddMasterMemifInterfaceAsSlave(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifCreateReply{
		SwIfIndex: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	swIfIdx, err := ifHandler.AddMemifInterface("memif", &ifModel.Interfaces_Interface_Memif{
		Id:     1,
		Mode:   ifModel.Interfaces_Interface_Memif_IP,
		Secret: "secret",
		Master: false,
	}, 5)

	Expect(err).To(BeNil())
	Expect(swIfIdx).To(BeEquivalentTo(1))
	var msgCheck bool
	for _, msg := range ctx.MockChannel.Msgs {
		vppMsg, ok := msg.(*memif.MemifCreate)
		if ok {
			Expect(vppMsg.Role).To(BeEquivalentTo(1))
			msgCheck = true
		}
	}
	Expect(msgCheck).To(BeTrue())
}

func TestAddMasterMemifInterfaceError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifCreate{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	_, err := ifHandler.AddMemifInterface("memif", &ifModel.Interfaces_Interface_Memif{
		Id:     1,
		Mode:   ifModel.Interfaces_Interface_Memif_IP,
		Secret: "secret",
		Master: false,
	}, 5)

	Expect(err).ToNot(BeNil())
}

func TestAddMasterMemifInterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifCreateReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	_, err := ifHandler.AddMemifInterface("memif", &ifModel.Interfaces_Interface_Memif{
		Id:     1,
		Mode:   ifModel.Interfaces_Interface_Memif_IP,
		Secret: "secret",
		Master: false,
	}, 5)

	Expect(err).ToNot(BeNil())
}

func TestDeleteMemifInterface(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifDeleteReply{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteMemifInterface("memif", 1)

	Expect(err).To(BeNil())
}

func TestDeleteMemifInterfaceError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifDelete{})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteMemifInterface("memif", 1)

	Expect(err).ToNot(BeNil())
}

func TestDeleteMemifInterfaceRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifDeleteReply{
		Retval: 1,
	})
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceTagAddDelReply{})

	err := ifHandler.DeleteMemifInterface("memif", 1)

	Expect(err).ToNot(BeNil())
}

func TestRegisterMemifSocketFilename(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDelReply{})

	err := ifHandler.RegisterMemifSocketFilename([]byte("filename"), 1)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*memif.MemifSocketFilenameAddDel)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
	Expect(vppMsg.SocketID).To(BeEquivalentTo(1))
	Expect(vppMsg.SocketFilename).To(BeEquivalentTo([]byte("filename")))
}

func TestRegisterMemifSocketFilenameError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDel{})

	err := ifHandler.RegisterMemifSocketFilename([]byte("filename"), 1)

	Expect(err).ToNot(BeNil())
}

func TestRegisterMemifSocketFilenameRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&memif.MemifSocketFilenameAddDelReply{
		Retval: 1,
	})

	err := ifHandler.RegisterMemifSocketFilename([]byte("filename"), 1)

	Expect(err).ToNot(BeNil())
}
