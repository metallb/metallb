//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package vppcalls_test

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	bin_api "github.com/ligato/vpp-agent/plugins/vpp/binapi/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/tests/vppcallmock"

	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	. "github.com/onsi/gomega"
)

func TestNat44InterfaceDump(t *testing.T) {
	ctx, natHandler, swIfIndexes := natTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bin_api.Nat44InterfaceDetails{
		SwIfIndex: 1,
		IsInside:  0,
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	swIfIndexes.RegisterName("if0", 1, nil)

	ifaces, err := natHandler.Nat44InterfaceDump()
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(1))
	Expect(ifaces[0].IsInside).To(BeFalse())
}

func TestNat44InterfaceDump2(t *testing.T) {
	ctx, natHandler, swIfIndexes := natTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bin_api.Nat44InterfaceDetails{
		SwIfIndex: 1,
		IsInside:  1,
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	swIfIndexes.RegisterName("if0", 1, nil)

	ifaces, err := natHandler.Nat44InterfaceDump()
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(1))
	Expect(ifaces[0].IsInside).To(BeTrue())
}

func TestNat44InterfaceDump3(t *testing.T) {
	ctx, natHandler, swIfIndexes := natTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&bin_api.Nat44InterfaceDetails{
		SwIfIndex: 1,
		IsInside:  2,
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	swIfIndexes.RegisterName("if0", 1, nil)

	ifaces, err := natHandler.Nat44InterfaceDump()
	Expect(err).To(Succeed())
	Expect(ifaces).To(HaveLen(2))
	Expect(ifaces[0].IsInside).To(BeFalse())
	Expect(ifaces[1].IsInside).To(BeTrue())
}

func natTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.NatVppAPI, ifaceidx.SwIfIndexRW) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	swIfIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "test-sw_if_indexes", ifaceidx.IndexMetadata))
	natHandler := vppcalls.NewNatVppHandler(ctx.MockChannel, ctx.MockChannel, swIfIndexes, log,)
	return ctx, natHandler, swIfIndexes
}
