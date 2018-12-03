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

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/stn"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

func TestAddStnRule(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRuleReply{})

	_, ip, _ := net.ParseCIDR("10.0.0.1/24")
	err := stnHandler.AddStnRule(1, &ip.IP)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*stn.StnAddDelRule)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IPAddress).To(BeEquivalentTo(net.ParseIP("10.0.0.0").To4())) // net IP
	Expect(vppMsg.IsIP4).To(BeEquivalentTo(1))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestAddStnRuleIPv6(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRuleReply{})

	_, ip, _ := net.ParseCIDR("2001:db8:0:1:1:1:1:1/128")
	err := stnHandler.AddStnRule(1, &ip.IP)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*stn.StnAddDelRule)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.SwIfIndex).To(BeEquivalentTo(1))
	Expect(vppMsg.IPAddress).To(BeEquivalentTo(net.ParseIP("2001:db8:0:1:1:1:1:1").To16()))
	Expect(vppMsg.IsIP4).To(BeEquivalentTo(0))
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(1))
}

func TestAddStnRuleInvalidIP(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRuleReply{})

	var ip net.IP = []byte("invalid-ip")
	err := stnHandler.AddStnRule(1, &ip)

	Expect(err).ToNot(BeNil())
}

func TestAddStnRuleError(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRule{})

	_, ip, _ := net.ParseCIDR("10.0.0.1/24")
	err := stnHandler.AddStnRule(1, &ip.IP)

	Expect(err).ToNot(BeNil())
}

func TestAddStnRuleRetval(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRuleReply{
		Retval: 1,
	})

	_, ip, _ := net.ParseCIDR("10.0.0.1/24")
	err := stnHandler.AddStnRule(1, &ip.IP)

	Expect(err).ToNot(BeNil())
}

func TestDelStnRule(t *testing.T) {
	ctx, stnHandler := stnTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&stn.StnAddDelRuleReply{})

	_, ip, _ := net.ParseCIDR("10.0.0.1/24")
	err := stnHandler.DelStnRule(1, &ip.IP)

	Expect(err).To(BeNil())
	vppMsg, ok := ctx.MockChannel.Msg.(*stn.StnAddDelRule)
	Expect(ok).To(BeTrue())
	Expect(vppMsg.IsAdd).To(BeEquivalentTo(0))
}

func stnTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.StnVppAPI) {
	ctx := vppcallmock.SetupTestCtx(t)
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "stn-if-idx", nil))
	stnHandler := vppcalls.NewStnVppHandler(ctx.MockChannel, ifIndexes, logrus.DefaultLogger())
	return ctx, stnHandler
}
