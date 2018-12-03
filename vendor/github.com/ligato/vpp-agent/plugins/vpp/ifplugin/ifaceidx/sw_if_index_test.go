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

package ifaceidx_test

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	. "github.com/onsi/gomega"
)

func testInitialization(t *testing.T, toInterfaces map[string][]string) (idxvpp.NameToIdxRW, ifaceidx.SwIfIndexRW, []*intf.Interfaces_Interface) {
	RegisterTestingT(t)

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "sw_if_index_test", ifaceidx.IndexMetadata)
	index := ifaceidx.NewSwIfIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	// data preparation
	var interfaces []*intf.Interfaces_Interface

	for name, ipadrrs := range toInterfaces {
		interfaces = append(interfaces, &intf.Interfaces_Interface{
			Name:        name,
			IpAddresses: ipadrrs,
		})
	}

	return index.GetMapping(), index, interfaces
}

// TestIndexMetadata tests whether func IndexMetadata return map filled with correct values
func TestIndexMetadata(t *testing.T) {
	// Constants
	const (
		ifName0           = "if0"
		ipAddr0           = "192.168.0.1/24"
		ipAddr1           = "192.168.1.1/24"
		ifaceNameIndexKey = "ipAddrKey"
	)

	_, _, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	iface := interfaces[0]

	result := ifaceidx.IndexMetadata(nil)
	Expect(result).To(HaveLen(0))

	result = ifaceidx.IndexMetadata(iface)
	Expect(result).To(HaveLen(1))

	ipAddrs := result[ifaceNameIndexKey]
	Expect(ipAddrs).To(HaveLen(2))
	Expect(ipAddrs).To(ContainElement(ipAddr0))
	Expect(ipAddrs).To(ContainElement(ipAddr1))
}

// Tests registering and unregistering name to index
func TestRegisterAndUnregisterName(t *testing.T) {
	// Constants
	const (
		ifName0        = "if0"
		ipAddr0        = "192.168.0.1/24"
		ipAddr1        = "192.168.1.1/24"
		idx0    uint32 = 0
	)

	mapping, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	intif0 := interfaces[0]

	// Register if0
	index.RegisterName(intif0.Name, idx0, intif0)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(intif0.Name))

	// Unregister if0
	index.UnregisterName(intif0.Name)
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests index mapping clear
func TestClearInterfaces(t *testing.T) {
	mapping, index, _ := testInitialization(t, nil)

	// Register entries
	index.RegisterName("if1", 0, nil)
	index.RegisterName("if2", 1, nil)
	index.RegisterName("if3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests updating of metadata
func TestUpdateMetadata(t *testing.T) {
	// Constants
	const (
		ifName0        = "if0"
		ipAddr0        = "192.168.0.1/24"
		ipAddr1        = "192.168.1.1/24"
		ipAddr2        = "192.168.2.1/24"
		ipAddr3        = "192.168.3.1/24"
		idx0    uint32 = 0
	)

	mapping, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	// Prepare some data
	if0 := interfaces[0]

	ifUpdate1 := &intf.Interfaces_Interface{
		Name:        ifName0,
		IpAddresses: []string{ipAddr2},
	}

	ifUpdate2 := &intf.Interfaces_Interface{
		Name:        ifName0,
		IpAddresses: []string{ipAddr3},
	}

	// Update before registration (no entry created)
	success := index.UpdateMetadata(if0.Name, if0)
	Expect(success).To(BeFalse())

	_, metadata, found := mapping.LookupIdx(if0.Name)
	Expect(found).To(BeFalse())
	Expect(metadata).To(BeNil())

	// Register interface name
	index.RegisterName(if0.Name, idx0, if0)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(if0.Name))

	// Evaluate entry metadata
	_, metadata, found = mapping.LookupIdx(if0.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	data, ok := metadata.(*intf.Interfaces_Interface)
	Expect(ok).To(BeTrue())
	Expect(data.IpAddresses).To(HaveLen(2))

	ipaddrs := data.IpAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr0))
	Expect(ipaddrs).To(ContainElement(ipAddr1))

	// Update metadata (same name, different data)
	success = index.UpdateMetadata(ifUpdate1.Name, ifUpdate1)
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	_, metadata, found = index.LookupIdx(if0.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	data, ok = metadata.(*intf.Interfaces_Interface)
	Expect(ok).To(BeTrue())
	Expect(data.IpAddresses).To(HaveLen(1))

	ipaddrs = data.IpAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr2))

	// Update metadata again
	success = index.UpdateMetadata(ifUpdate2.Name, ifUpdate2)
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	_, metadata, found = mapping.LookupIdx(if0.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	data, ok = metadata.(*intf.Interfaces_Interface)
	Expect(ok).To(BeTrue())
	Expect(data.IpAddresses).To(HaveLen(1))

	ipaddrs = data.IpAddresses
	Expect(ipaddrs).To(ContainElement(ipAddr3))

	// Unregister
	index.UnregisterName(if0.Name)

	// Evaluate unregistration
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests lookup by index
func TestLookupByIndex(t *testing.T) {
	// Constants
	const (
		ifName0        = "if0"
		ipAddr0        = "192.168.0.1/24"
		ipAddr1        = "192.168.1.1/24"
		idx0    uint32 = 0
	)

	_, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	if0 := interfaces[0]
	index.RegisterName(if0.Name, idx0, if0)

	foundIdx, metadata, exist := index.LookupIdx(ifName0)
	Expect(exist).To(BeTrue())
	Expect(foundIdx).To(Equal(idx0))
	Expect(metadata).To(Equal(if0))
}

// Tests lookup by name
func TestLookupByName(t *testing.T) {
	// Constants
	const (
		ifName0        = "if0"
		ipAddr0        = "192.168.0.1/24"
		ipAddr1        = "192.168.1.1/24"
		idx0    uint32 = 0
	)

	_, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	if0 := interfaces[0]
	index.RegisterName(if0.Name, idx0, if0)

	foundName, metadata, exist := index.LookupName(idx0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(if0.Name))
	Expect(metadata).To(Equal(if0))
}

// Tests lookup by ip address
func TestLookupByIP(t *testing.T) {
	// Constants
	const (
		ifName0 = "if0"
		ifName1 = "if1"
		ifName2 = "if2"
		ipAddr0 = "192.168.0.1/24"
		ipAddr1 = "192.168.1.1/24"
		ipAddr2 = "192.168.2.1/24"
		ipAddr3 = "192.168.3.1/24"
	)

	// defines 3 interfaces
	_, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
		ifName1: {ipAddr2},
		ifName2: {ipAddr3},
	})

	// register all interfaces
	for i, iface := range interfaces {
		index.RegisterName(iface.Name, uint32(i), iface)
	}

	// try to lookup each interface by each ip adress
	ifaces := index.LookupNameByIP(ipAddr0)
	Expect(ifaces).To(ContainElement(ifName0))

	ifaces = index.LookupNameByIP(ipAddr1)
	Expect(ifaces).To(ContainElement(ifName0))

	ifaces = index.LookupNameByIP(ipAddr2)
	Expect(ifaces).To(ContainElement(ifName1))

	ifaces = index.LookupNameByIP(ipAddr3)
	Expect(ifaces).To(ContainElement(ifName2))

	// try empty lookup, should return nothing
	ifaces = index.LookupNameByIP("")
	Expect(ifaces).To(BeEmpty())
}

// Tests watch name to index
func TestWatchNameToIdx(t *testing.T) {
	// Constants
	const (
		testName        = "testName"
		ifName0         = "if0"
		ipAddr0         = "192.168.0.1/24"
		ipAddr1         = "192.168.1.1/24"
		idx0     uint32 = 0
	)

	_, index, interfaces := testInitialization(t, map[string][]string{
		ifName0: {ipAddr0, ipAddr1},
	})

	c := make(chan ifaceidx.SwIfIdxDto)
	index.WatchNameToIdx(testName, c)

	if0 := interfaces[0]
	index.RegisterName(if0.Name, idx0, if0)

	var dto ifaceidx.SwIfIdxDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(if0.Name))
	Expect(dto.Metadata).To(Equal(if0))
}
