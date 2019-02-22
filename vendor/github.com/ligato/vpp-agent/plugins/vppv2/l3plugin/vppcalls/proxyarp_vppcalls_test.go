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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test enable/disable proxy arp
func TestProxyArp(t *testing.T) {
	ctx, ifIndexes, _, pArpHandler := pArpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err := pArpHandler.EnableProxyArpInterface("if1")
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{})
	err = pArpHandler.DisableProxyArpInterface("if1")
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.ProxyArpIntfcEnableDisableReply{Retval: 1})
	err = pArpHandler.DisableProxyArpInterface("if1")
	Expect(err).NotTo(BeNil())
}

// Test add/delete ip range for proxy arp
func TestProxyArpRange(t *testing.T) {
	ctx, _, _, pArpHandler := pArpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err := pArpHandler.AddProxyArpRange([]byte{192, 168, 10, 20}, []byte{192, 168, 10, 30})
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{})
	err = pArpHandler.DeleteProxyArpRange([]byte{192, 168, 10, 23}, []byte{192, 168, 10, 27})
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.ProxyArpAddDelReply{Retval: 1})
	err = pArpHandler.AddProxyArpRange([]byte{192, 168, 10, 23}, []byte{192, 168, 10, 27})
	Expect(err).To(Not(BeNil()))
}

func pArpTestSetup(t *testing.T) (*vppcallmock.TestCtx, ifaceidx.IfaceMetadataIndexRW, vppcalls.ArpVppAPI, vppcalls.ProxyArpVppAPI) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifIndexes := ifaceidx.NewIfaceIndex(logrus.NewLogger("test"), "test")
	arpHandler := vppcalls.NewArpVppHandler(ctx.MockChannel, ifIndexes, log)
	pArpHandler := vppcalls.NewProxyArpVppHandler(ctx.MockChannel, ifIndexes, log)
	return ctx, ifIndexes, arpHandler, pArpHandler
}
