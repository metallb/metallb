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
	l3 "github.com/ligato/vpp-agent/api/models/vpp/l3"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var arpEntries = []*l3.ARPEntry{
	{
		Interface:   "if1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:45:59:8E:BD",
		Static:      true,
	},
	{
		Interface:   "if1",
		IpAddress:   "192.168.10.22",
		PhysAddress: "6C:45:59:59:8E:BD",
		Static:      false,
	},
	{
		Interface:   "if1",
		IpAddress:   "dead::1",
		PhysAddress: "8E:BD:6C:45:59:59",
		Static:      false,
	},
}

// Test adding of ARP
func TestAddArp(t *testing.T) {
	ctx, ifIndexes, arpHandler := arpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := arpHandler.VppAddArp(arpEntries[0])
	Expect(err).To(Succeed())
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err = arpHandler.VppAddArp(arpEntries[1])
	Expect(err).To(Succeed())
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err = arpHandler.VppAddArp(arpEntries[2])
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{Retval: 1})
	err = arpHandler.VppAddArp(arpEntries[0])
	Expect(err).NotTo(BeNil())
}

// Test deleting of ARP
func TestDelArp(t *testing.T) {
	ctx, ifIndexes, arpHandler := arpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ifIndexes.Put("if1", &ifaceidx.IfaceMetadata{SwIfIndex: 1})

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := arpHandler.VppDelArp(arpEntries[0])
	Expect(err).To(Succeed())
}

func arpTestSetup(t *testing.T) (*vppcallmock.TestCtx, ifaceidx.IfaceMetadataIndexRW, vppcalls.ArpVppAPI) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifIndexes := ifaceidx.NewIfaceIndex(logrus.NewLogger("test"), "test")
	arpHandler := vppcalls.NewArpVppHandler(ctx.MockChannel, ifIndexes, log)
	return ctx, ifIndexes, arpHandler
}
