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

package ifaceidx

import (
	"testing"

	"fmt"
	"github.com/ligato/cn-infra/logging/logrus"
	. "github.com/onsi/gomega"
)

// Constants
const (
	ifName0          = "if0"
	ifName1          = "if1"
	ifName2          = "if2"
	idx0      uint32 = 0
	idx1      uint32 = 1
	idx2      uint32 = 2
	ipAddr0          = "192.168.0.1/24"
	ipAddr1          = "192.168.1.1/24"
	ipAddr2          = "192.168.2.1/24"
	ipAddr3          = "192.168.3.1/24"
	watchName        = "watchName"
)

func testInitialization(t *testing.T) IfaceMetadataIndexRW {
	RegisterTestingT(t)
	return NewIfaceIndex(logrus.DefaultLogger(), "iface-meta-index")
}

// TestIndexMetadata tests whether func IndexMetadata return map filled with correct values
func TestIndexMetadata(t *testing.T) {
	const (
		ifName0 = "if0"
		ipAddr0 = "192.168.0.1/24"
		ipAddr1 = "192.168.1.1/24"
	)

	testInitialization(t)
	iface := &IfaceMetadata{IPAddresses: []string{ipAddr0, ipAddr1}}

	result := indexMetadata(nil)
	Expect(result).To(HaveLen(0))

	result = indexMetadata(iface)
	Expect(result).To(HaveLen(1))

	ipAddrs := result[ipAddressIndexKey]
	Expect(ipAddrs).To(HaveLen(2))
	Expect(ipAddrs).To(ContainElement(ipAddr0))
	Expect(ipAddrs).To(ContainElement(ipAddr1))
}

// Tests registering and unregistering name to index
func TestRegisterAndUnregisterName(t *testing.T) {
	index := testInitialization(t)
	iface := &IfaceMetadata{SwIfIndex: idx0, IPAddresses: []string{ipAddr0, ipAddr1}}

	// Register iface
	index.Put(ifName0, iface)
	names := index.ListAllInterfaces()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(ifName0))

	// Unregister iface
	index.Delete(ifName0)
	names = index.ListAllInterfaces()
	Expect(names).To(BeEmpty())
}

// Tests index mapping clear
func TestClearInterfaces(t *testing.T) {
	index := testInitialization(t)

	// Register entries
	index.Put("if1", &IfaceMetadata{SwIfIndex: 0})
	index.Put("if2", &IfaceMetadata{SwIfIndex: 1})
	index.Put("if3", &IfaceMetadata{SwIfIndex: 2})
	names := index.ListAllInterfaces()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = index.ListAllInterfaces()
	Expect(names).To(BeEmpty())
}

// Tests updating of metadata
func TestUpdateMetadata(t *testing.T) {
	index := testInitialization(t)
	iface := &IfaceMetadata{IPAddresses: []string{ipAddr0, ipAddr1}}

	ifUpdate1 := &IfaceMetadata{
		IPAddresses: []string{ipAddr2},
	}

	ifUpdate2 := &IfaceMetadata{
		IPAddresses: []string{ipAddr3},
	}

	// Update before registration (no entry created)
	success := index.Update(ifName0, iface)
	Expect(success).To(BeFalse())

	metadata, found := index.LookupByName(ifName0)
	Expect(found).To(BeFalse())
	Expect(metadata).To(BeNil())

	// Add interface
	index.Put(ifName0, iface)
	names := index.ListAllInterfaces()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(ifName0))

	// Evaluate entry metadata
	metadata, found = index.LookupByName(ifName0)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
	Expect(metadata.IPAddresses).To(HaveLen(2))

	ipaddrs := metadata.IPAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr0))
	Expect(ipaddrs).To(ContainElement(ipAddr1))

	// Update metadata (same name, different data)
	success = index.Update(ifName0, ifUpdate1)
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	metadata, found = index.LookupByName(ifName0)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
	Expect(metadata.IPAddresses).To(HaveLen(1))

	ipaddrs = metadata.IPAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr2))

	// Update metadata again
	success = index.Update(ifName0, ifUpdate2)
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	metadata, found = index.LookupByName(ifName0)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
	Expect(metadata.IPAddresses).To(HaveLen(1))

	ipaddrs = metadata.IPAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr3))

	// Remove interface
	index.Delete(ifName0)

	// Check removal
	names = index.ListAllInterfaces()
	Expect(names).To(BeEmpty())
}

// Tests lookup by name
func TestLookupByName(t *testing.T) {
	index := testInitialization(t)
	iface := &IfaceMetadata{SwIfIndex: idx0, IPAddresses: []string{ipAddr0, ipAddr1}}

	index.Put(ifName0, iface)

	metadata, exist := index.LookupByName(ifName0)
	Expect(exist).To(BeTrue())
	Expect(metadata.GetIndex()).To(Equal(idx0))
	Expect(metadata).To(Equal(iface))
}

// Tests lookup by index
func TestLookupByIndex(t *testing.T) {
	index := testInitialization(t)
	iface := &IfaceMetadata{SwIfIndex: idx0, IPAddresses: []string{ipAddr0, ipAddr1}}

	index.Put(ifName0, iface)

	foundName, metadata, exist := index.LookupBySwIfIndex(idx0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(ifName0))
	Expect(metadata).To(Equal(iface))
}

// Tests lookup by ip address
func TestLookupByIP(t *testing.T) {
	index := testInitialization(t)

	// defines 3 interfaces
	iface1 := &IfaceMetadata{SwIfIndex: idx0, IPAddresses: []string{ipAddr0, ipAddr1}}
	iface2 := &IfaceMetadata{SwIfIndex: idx1, IPAddresses: []string{ipAddr0, ipAddr2}}
	iface3 := &IfaceMetadata{SwIfIndex: idx2, IPAddresses: []string{ipAddr3}}

	// register all interfaces
	index.Put(ifName0, iface1)
	index.Put(ifName1, iface2)
	index.Put(ifName2, iface3)

	// try to lookup each interface by each ip adress
	ifaces := index.LookupByIP(ipAddr0)
	Expect(ifaces).To(ContainElement(ifName0))
	Expect(ifaces).To(ContainElement(ifName1))
	Expect(ifaces).To(HaveLen(2))

	ifaces = index.LookupByIP(ipAddr1)
	Expect(ifaces).To(ContainElement(ifName0))
	Expect(ifaces).To(HaveLen(1))

	ifaces = index.LookupByIP(ipAddr2)
	Expect(ifaces).To(ContainElement(ifName1))
	Expect(ifaces).To(HaveLen(1))

	ifaces = index.LookupByIP(ipAddr3)
	Expect(ifaces).To(ContainElement(ifName2))
	Expect(ifaces).To(HaveLen(1))

	// try empty lookup, should return nothing
	ifaces = index.LookupByIP("")
	Expect(ifaces).To(BeEmpty())
}

// Tests watch interfaces
func TestWatchNameToIdx(t *testing.T) {
	fmt.Println("TestWatchNameToIdx")
	index := testInitialization(t)
	iface := &IfaceMetadata{SwIfIndex: idx0, IPAddresses: []string{ipAddr0, ipAddr1}}

	c := make(chan IfaceMetadataDto, 10)
	index.WatchInterfaces(watchName, c)

	index.Put(ifName0, iface)

	var dto IfaceMetadataDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(ifName0))
	Expect(dto.Metadata.GetIndex()).To(Equal(idx0))
}
