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

package l3idx_test

import (
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin/l3idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
	. "github.com/onsi/gomega"
	"testing"
)

func l3routeIndexTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, l3idx.RouteIndexRW) {
	RegisterTestingT(t)

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "index_test", nil)
	index := l3idx.NewRouteIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	return index.GetMapping(), index
}

var routes = []l3.StaticRoutes_Route{
	{
		VrfId:             0,
		DstIpAddr:         "10.1.1.3/32",
		NextHopAddr:       "192.168.1.13",
		Weight:            6,
		OutgoingInterface: "tap1",
	},
	{
		VrfId:             1,
		DstIpAddr:         "dead:01::01/64",
		NextHopAddr:       "192.168.1.13",
		Weight:            6,
		OutgoingInterface: "tap2",
	},
}

func TestRouteRegisterAndUnregisterName(t *testing.T) {
	mapping, l3index := l3routeIndexTestInitialization(t)

	// Register entry
	l3index.RegisterName("l3", 0, &routes[0])
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement("l3"))

	// Unregister entry
	l3index.UnregisterName("l3")
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

func TestRouteLookupIndex(t *testing.T) {
	_, l3index := l3routeIndexTestInitialization(t)

	l3index.RegisterName("l3", 0, &routes[0])

	foundName, route, exist := l3index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal("l3"))
	Expect(route.OutgoingInterface).To(Equal("tap1"))
	_, _, exist = l3index.LookupName(1)
	Expect(exist).To(BeFalse())
}

func TestRouteLookupName(t *testing.T) {
	_, l3index := l3routeIndexTestInitialization(t)

	l3index.RegisterName("l3", 1, &routes[1])

	foundName, route, exist := l3index.LookupIdx("l3")
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(uint32(1)))
	Expect(route.OutgoingInterface).To(Equal("tap2"))
	_, _, exist = l3index.LookupIdx("l3a")
	Expect(exist).To(BeFalse())
}

func TestRouteLookupRouteAndIDByOutgoingIfc(t *testing.T) {
	_, l3index := l3routeIndexTestInitialization(t)

	l3index.RegisterName("l3", 1, &routes[0])
	routes := l3index.LookupRouteAndIDByOutgoingIfc("tap1")
	Expect(routes).To(Not(BeNil()))
	Expect(routes[0].Route.OutgoingInterface).To(Equal("tap1"))
	routes = l3index.LookupRouteAndIDByOutgoingIfc("tap2")
	Expect(routes).To(BeNil())
}
