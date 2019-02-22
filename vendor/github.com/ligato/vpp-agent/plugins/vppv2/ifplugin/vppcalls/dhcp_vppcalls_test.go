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

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/dhcp"
	. "github.com/onsi/gomega"
)

func TestSetInterfaceAsDHCPClient(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPClientConfigReply{})

	err := ifHandler.SetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*dhcp.DHCPClientConfig)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.Client.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.Client.Hostname).To(BeEquivalentTo([]byte("hostName")))
	Expect(vppMsg.Client.WantDHCPEvent).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestSetInterfaceAsDHCPClientError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPComplEvent{})

	err := ifHandler.SetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).ToNot(BeNil())
}

func TestSetInterfaceAsDHCPClientRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPClientConfigReply{
		Retval: 1,
	})

	err := ifHandler.SetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).ToNot(BeNil())
}

func TestUnsetInterfaceAsDHCPClient(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPClientConfigReply{})

	err := ifHandler.UnsetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*dhcp.DHCPClientConfig)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.Client.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.Client.Hostname).To(BeEquivalentTo([]byte("hostName")))
	Expect(vppMsg.Client.WantDHCPEvent).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(0))
}

func TestUnsetInterfaceAsDHCPClientError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPComplEvent{})

	err := ifHandler.UnsetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).ToNot(BeNil())
}

func TestUnsetInterfaceAsDHCPClientRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&dhcp.DHCPClientConfigReply{
		Retval: 1,
	})

	err := ifHandler.UnsetInterfaceAsDHCPClient(1, "hostName")

	Expect(err).ToNot(BeNil())
}
