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
	"net"
	"testing"

	"github.com/ligato/vpp-agent/plugins/vpp/binapi/interfaces"
	. "github.com/onsi/gomega"
)

func TestAddInterfaceIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	Expect(err).To(BeNil())
	err = ifHandler.AddInterfaceIP(1, ipNet)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceAddDelAddress)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.Address).To(BeEquivalentTo(net.ParseIP("10.0.0.0").To4()))
	Expect(vppMsg.AddressLength).To(BeEquivalentTo(24))
	Expect(vppMsg.DelAll).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestAddInterfaceIPv6(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	_, ipNet, err := net.ParseCIDR("2001:db8:0:1:1:1:1:1/128")
	Expect(err).To(BeNil())
	err = ifHandler.AddInterfaceIP(1, ipNet)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceAddDelAddress)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(1))
	Expect(vppMsg.Address).To(BeEquivalentTo(net.ParseIP("2001:db8:0:1:1:1:1:1").To16()))
	Expect(vppMsg.AddressLength).To(BeEquivalentTo(128))
	Expect(vppMsg.DelAll).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestAddInterfaceInvalidIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	err := ifHandler.AddInterfaceIP(1, &net.IPNet{
		IP: []byte("invalid-ip"),
	})

	Expect(err).ToNot(BeNil())
}

func TestAddInterfaceIPError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddress{})

	err = ifHandler.AddInterfaceIP(1, ipNet)

	Expect(err).ToNot(BeNil())
}

func TestAddInterfaceIPRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{
		Retval: 1,
	})

	err = ifHandler.AddInterfaceIP(1, ipNet)

	Expect(err).ToNot(BeNil())
}

func TestDelInterfaceIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	Expect(err).To(BeNil())
	err = ifHandler.DelInterfaceIP(1, ipNet)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceAddDelAddress)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(0))
	Expect(vppMsg.Address).To(BeEquivalentTo(net.ParseIP("10.0.0.0").To4()))
	Expect(vppMsg.AddressLength).To(BeEquivalentTo(24))
	Expect(vppMsg.DelAll).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(0))
}

func TestDelInterfaceIPv6(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	_, ipNet, err := net.ParseCIDR("2001:db8:0:1:1:1:1:1/128")
	Expect(err).To(BeNil())
	err = ifHandler.DelInterfaceIP(1, ipNet)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceAddDelAddress)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsIPv6).To(BeEquivalentTo(1))
	Expect(vppMsg.Address).To(BeEquivalentTo(net.ParseIP("2001:db8:0:1:1:1:1:1").To16()))
	Expect(vppMsg.AddressLength).To(BeEquivalentTo(128))
	Expect(vppMsg.DelAll).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(0))
}

func TestDelInterfaceInvalidIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{})

	err := ifHandler.DelInterfaceIP(1, &net.IPNet{
		IP: []byte("invalid-ip"),
	})

	Expect(err).ToNot(BeNil())
}

func TestDelInterfaceIPError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddress{})

	err = ifHandler.DelInterfaceIP(1, ipNet)

	Expect(err).ToNot(BeNil())
}

func TestDelInterfaceIPRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	_, ipNet, err := net.ParseCIDR("10.0.0.1/24")
	ctx.MockVpp.MockReply(&interfaces.SwInterfaceAddDelAddressReply{
		Retval: 1,
	})

	err = ifHandler.DelInterfaceIP(1, ipNet)

	Expect(err).ToNot(BeNil())
}

func TestSetUnnumberedIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{})

	err := ifHandler.SetUnnumberedIP(1, 2)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceSetUnnumbered)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(2))
	Expect(vppMsg.UnnumberedSwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestSetUnnumberedIPError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumbered{})

	err := ifHandler.SetUnnumberedIP(1, 2)

	Expect(err).ToNot(BeNil())
}

func TestSetUnnumberedIPRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{
		Retval: 1,
	})

	err := ifHandler.SetUnnumberedIP(1, 2)

	Expect(err).ToNot(BeNil())
}

func TestUnsetUnnumberedIP(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{})

	err := ifHandler.UnsetUnnumberedIP(1)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*interfaces.SwInterfaceSetUnnumbered)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(0))
	Expect(vppMsg.UnnumberedSwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(0))
}

func TestUnsetUnnumberedIPError(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumbered{})

	err := ifHandler.UnsetUnnumberedIP(1)

	Expect(err).ToNot(BeNil())
}

func TestUnsetUnnumberedIPRetval(t *testing.T) {
	ctx, ifHandler := ifTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&interfaces.SwInterfaceSetUnnumberedReply{
		Retval: 1,
	})

	err := ifHandler.UnsetUnnumberedIP(1)

	Expect(err).ToNot(BeNil())
}
