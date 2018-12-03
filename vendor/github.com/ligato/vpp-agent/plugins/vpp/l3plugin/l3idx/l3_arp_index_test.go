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

func l3arpIndexTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, l3idx.ARPIndexRW) {
	RegisterTestingT(t)

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "index_test", nil)
	index := l3idx.NewARPIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	return index.GetMapping(), index
}

var arpEntries = []l3.ArpTable_ArpEntry{
	{
		Interface:   "tap1",
		IpAddress:   "192.168.10.21",
		PhysAddress: "59:6C:DE:AD:00:01",
		Static:      true,
	},
	{
		Interface:   "tap2",
		IpAddress:   "192.168.10.22",
		PhysAddress: "59:6C:DE:AD:00:02",
		Static:      true,
	},
	{
		Interface:   "tap3",
		IpAddress:   "dead::01",
		PhysAddress: "59:6C:DE:AD:00:03",
		Static:      false,
	},
}

func TestArpRegisterAndUnregisterName(t *testing.T) {
	mapping, l3index := l3arpIndexTestInitialization(t)

	// Register entry
	l3index.RegisterName("l3", 0, &arpEntries[0])
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement("l3"))

	// Unregister entry
	l3index.UnregisterName("l3")
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

func TestArpLookupIndex(t *testing.T) {
	_, l3index := l3arpIndexTestInitialization(t)

	l3index.RegisterName("l3", 0, &arpEntries[0])

	foundName, arp, exist := l3index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal("l3"))
	Expect(arp.Interface).To(Equal("tap1"))
	_, _, exist = l3index.LookupName(1)
	Expect(exist).To(BeFalse())
}

func TestArpLookupName(t *testing.T) {
	_, l3index := l3arpIndexTestInitialization(t)

	l3index.RegisterName("l3", 1, &arpEntries[2])

	foundName, arp, exist := l3index.LookupIdx("l3")
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(uint32(1)))
	Expect(arp.Interface).To(Equal("tap3"))
	_, _, exist = l3index.LookupIdx("l3a")
	Expect(exist).To(BeFalse())
}

func TestArpLookupNamesByInterface(t *testing.T) {
	_, l3index := l3arpIndexTestInitialization(t)

	l3index.RegisterName("l3", 1, &arpEntries[0])
	arp := l3index.LookupNamesByInterface("tap1")
	Expect(arp).To(Not(BeNil()))
	arp = l3index.LookupNamesByInterface("tap2")
	Expect(arp).To(BeNil())
}

func TestArpWatchNameToIdx(t *testing.T) {
	_, l3index := l3arpIndexTestInitialization(t)

	c := make(chan l3idx.ARPIndexDto)
	l3index.WatchNameToIdx("testName", c)

	l3index.RegisterName("l3", 3, &arpEntries[1])

	var dto l3idx.ARPIndexDto
	Eventually(c).Should(Receive(&dto))
	Expect(dto.Name).To(Equal("l3"))
	Expect(dto.NameToIdxDtoWithoutMeta.Idx).To(Equal(uint32(3)))
}
