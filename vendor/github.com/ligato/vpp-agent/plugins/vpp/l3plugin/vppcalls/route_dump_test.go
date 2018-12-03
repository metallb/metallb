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

package vppcalls

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/ip"
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

// Test dumping routes
func TestDumpStaticRoutes(t *testing.T) {
	ctx := vppcallmock.SetupTestCtx(t)
	ifIndexes := ifaceidx.NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), "rt-dump-if-idx", nil))
	l3handler := NewRouteVppHandler(ctx.MockChannel, ifIndexes, logrus.DefaultLogger())
	defer ctx.TeardownTestCtx()

	ifIndexes.RegisterName("if1", 2, nil)
	ifIndexes.RegisterName("if2", 3, nil)

	ctx.MockVpp.MockReply(&ip.IPFibDetails{
		Path: []ip.FibPath{{SwIfIndex: 3}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&ip.IP6FibDetails{
		Path: []ip.FibPath{{SwIfIndex: 2}},
	})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})

	rtDetails, err := l3handler.DumpStaticRoutes()
	Expect(err).To(Succeed())
	Expect(rtDetails).To(HaveLen(2))
	Expect(rtDetails[0].Route.OutgoingInterface).To(Equal("if2"))
	Expect(rtDetails[1].Route.OutgoingInterface).To(Equal("if1"))
}
