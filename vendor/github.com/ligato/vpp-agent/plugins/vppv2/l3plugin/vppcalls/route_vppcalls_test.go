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
	"github.com/ligato/vpp-agent/plugins/vpp/binapi/vpe"
	"github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/ifaceidx"
	ifvppcalls "github.com/ligato/vpp-agent/plugins/vppv2/ifplugin/vppcalls"
	"github.com/ligato/vpp-agent/plugins/vppv2/l3plugin/vppcalls"
	"github.com/ligato/vpp-agent/tests/vppcallmock"
	. "github.com/onsi/gomega"
)

var routes = []*l3.Route{
	{
		VrfId:       1,
		DstNetwork:  "192.168.10.21/24",
		NextHopAddr: "192.168.30.1",
	},
	{
		VrfId:       2,
		DstNetwork:  "10.0.0.1/24",
		NextHopAddr: "192.168.30.1",
	},
}

// Test adding routes
func TestAddRoute(t *testing.T) {
	ctx, _, rtHandler := routeTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ip.IPFibDetails{})
	ctx.MockVpp.MockReply(&vpe.ControlPingReply{})
	ctx.MockVpp.MockReply(&ip.IPTableAddDelReply{})
	ctx.MockVpp.MockReply(&ip.IPAddDelRouteReply{})
	err := rtHandler.VppAddRoute(routes[0])
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.IPAddDelRouteReply{})
	err = rtHandler.VppAddRoute(routes[0])
	Expect(err).To(Not(BeNil()))
}

// Test deleting routes
func TestDeleteRoute(t *testing.T) {
	ctx, _, rtHandler := routeTestSetup(t)
	defer ctx.TeardownTestCtx()

	ctx.MockVpp.MockReply(&ip.IPAddDelRouteReply{})
	err := rtHandler.VppDelRoute(routes[0])
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.IPAddDelRouteReply{})
	err = rtHandler.VppDelRoute(routes[1])
	Expect(err).To(Succeed())

	ctx.MockVpp.MockReply(&ip.IPAddDelRouteReply{Retval: 1})
	err = rtHandler.VppDelRoute(routes[0])
	Expect(err).To(Not(BeNil()))
}

func routeTestSetup(t *testing.T) (*vppcallmock.TestCtx, ifvppcalls.IfVppAPI, vppcalls.RouteVppAPI) {
	ctx := vppcallmock.SetupTestCtx(t)
	log := logrus.NewLogger("test-log")
	ifHandler := ifvppcalls.NewIfVppHandler(ctx.MockChannel, log)
	ifIndexes := ifaceidx.NewIfaceIndex(logrus.NewLogger("test"), "test")
	rtHandler := vppcalls.NewRouteVppHandler(ctx.MockChannel, ifIndexes, log)
	return ctx, ifHandler, rtHandler
}
