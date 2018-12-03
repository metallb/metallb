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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var arpEntries = []vppcalls.ArpEntry{
	{
		Interface:  1,
		IPAddress:  []byte{192, 168, 10, 21},
		MacAddress: "59:6C:45:59:8E:BD",
		Static:     true,
	},
	{
		Interface:  1,
		IPAddress:  []byte{192, 168, 10, 22},
		MacAddress: "6C:45:59:59:8E:BD",
		Static:     false,
	},
	{
		Interface:  1,
		IPAddress:  []byte{0xde, 0xad, 0, 0, 0, 0, 0, 0, 0xde, 0xad, 0, 0, 0, 0, 0, 1},
		MacAddress: "8E:BD:6C:45:59:59",
		Static:     false,
	},
}

// Test adding of ARP
func TestAddArp(t *testing.T) {
	ctx, arpHandler := arpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := arpHandler.VppAddArp(&arpEntries[0])
	Expect(err).To(Succeed())
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err = arpHandler.VppAddArp(&arpEntries[1])
	Expect(err).To(Succeed())
	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err = arpHandler.VppAddArp(&arpEntries[2])
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{Retval: 1})
	err = arpHandler.VppAddArp(&arpEntries[0])
	Expect(err).To(Not(BeNil()))
}

// Test deleting of ARP
func TestDelArp(t *testing.T) {
	ctx, arpHandler := arpTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ip.IPNeighborAddDelReply{})
	err := arpHandler.VppDelArp(&arpEntries[0])
	Expect(err).To(Succeed())
}

func arpTestSetup(t *testing.T) (*vppcallmock.TestCtx, vppcalls.ArpVppAPI) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(log, "arp-if-idx", nil))
	arpHandler := vppcalls.NewArpVppHandler(ctx.MockChannel, ifIndexes, log)
	return ctx, arpHandler
}
