// Copyright (c) 2017 Cisco and/or its affiliates.
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

package l2idx_test

import (
	"testing"

	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
	. "github.com/onsi/gomega"
)

const (
	// bridge domain name
	bdName0    = "bd0"
	bdName1    = "bd1"
	bdName2    = "bd2"
	ifaceAName = "interfaceA"
	ifaceBName = "interfaceB"
	ifaceCName = "interfaceC"
	ifaceDName = "interfaceD"

	idx0 uint32 = 0
	idx1 uint32 = 1
	idx2 uint32 = 2

	ifaceNameIndexKey = "ipAddrKey"
)

func testInitialization(t *testing.T, bdToIfaces map[string][]string) (idxvpp.NameToIdxRW, l2idx.BDIndexRW, []*l2.BridgeDomains_BridgeDomain) {
	RegisterTestingT(t)

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "bd_indexes_test", l2idx.IndexMetadata)
	bdIndex := l2idx.NewBDIndex(nameToIdx)
	names := nameToIdx.ListNames()
	Expect(names).To(BeEmpty())

	// data preparation
	var bridgeDomains []*l2.BridgeDomains_BridgeDomain
	for bdName, ifaces := range bdToIfaces {
		bridgeDomains = append(bridgeDomains, prepareBridgeDomainData(bdName, ifaces))
	}

	return bdIndex.GetMapping(), bdIndex, bridgeDomains
}

func prepareBridgeDomainData(bdName string, ifaces []string) *l2.BridgeDomains_BridgeDomain {
	var interfaces []*l2.BridgeDomains_BridgeDomain_Interfaces
	for _, iface := range ifaces {
		interfaces = append(interfaces, &l2.BridgeDomains_BridgeDomain_Interfaces{Name: iface})
	}
	return &l2.BridgeDomains_BridgeDomain{Interfaces: interfaces, Name: bdName}
}

/**
TestIndexMetadatat tests whether func IndexMetadata return map filled with correct values
*/
func TestIndexMetadata(t *testing.T) {
	RegisterTestingT(t)

	bridgeDomain := prepareBridgeDomainData(bdName0, []string{ifaceAName, ifaceBName})

	result := l2idx.IndexMetadata(nil)
	Expect(result).To(HaveLen(0))

	result = l2idx.IndexMetadata(l2idx.NewBDMetadata(bridgeDomain, nil))
	Expect(result).To(HaveLen(1))

	ifaceNames := result[ifaceNameIndexKey]
	Expect(ifaceNames).To(HaveLen(2))
	Expect(ifaceNames).To(ContainElement(ifaceAName))
	Expect(ifaceNames).To(ContainElement(ifaceBName))
}

/**
TestRegisterAndUnregisterName tests methods:
* RegisterName()
* UnregisterName()
*/
func TestRegisterAndUnregisterName(t *testing.T) {
	RegisterTestingT(t)

	nameToIdx, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
	})

	bdIndex.RegisterName(bridgeDomains[0].Name, idx0, l2idx.NewBDMetadata(bridgeDomains[0], nil))
	names := nameToIdx.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(bridgeDomains[0].Name))

	bdIndex.UnregisterName(bridgeDomains[0].Name)
	names = nameToIdx.ListNames()
	Expect(names).To(BeEmpty())
}

/**
TestUpdateMetadata tests methods:
* UpdateMetadata()
*/
func TestUpdateMetadata(t *testing.T) {
	RegisterTestingT(t)

	nameToIdx, bdIndex, _ := testInitialization(t, nil)
	bd := prepareBridgeDomainData(bdName0, []string{ifaceAName, ifaceBName})
	bdUpdt1 := prepareBridgeDomainData(bdName0, []string{ifaceCName})
	bdUpdt2 := prepareBridgeDomainData(bdName0, []string{ifaceDName})

	// Update before registration (no entry created)
	success := bdIndex.UpdateMetadata(bd.Name, l2idx.NewBDMetadata(bd, []string{ifaceAName, ifaceBName}))
	Expect(success).To(BeFalse())
	_, metadata, found := nameToIdx.LookupIdx(bd.Name)
	Expect(found).To(BeFalse())
	Expect(metadata).To(BeNil())

	// Register bridge domain
	bdIndex.RegisterName(bd.Name, idx0, l2idx.NewBDMetadata(bd, []string{ifaceAName, ifaceBName}))
	var names []string
	names = nameToIdx.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(bd.Name))

	// Evaluate entry metadata
	_, metadata, found = nameToIdx.LookupIdx(bd.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	bdData, ok := metadata.(*l2idx.BdMetadata)
	Expect(ok).To(BeTrue())
	Expect(bdData).ToNot(BeNil())
	Expect(bdData.BridgeDomain).ToNot(BeNil())
	Expect(bdData.BridgeDomain.Interfaces).To(HaveLen(2))
	Expect(bdData.ConfiguredInterfaces).To(HaveLen(2))

	var ifNames []string
	for _, ifData := range bdData.BridgeDomain.Interfaces {
		ifNames = append(ifNames, ifData.Name)
	}
	Expect(ifNames).To(ContainElement(ifaceAName))
	Expect(ifNames).To(ContainElement(ifaceBName))

	var configured []string
	for _, confIf := range bdData.ConfiguredInterfaces {
		configured = append(configured, confIf)
	}
	Expect(configured).To(ContainElement(ifaceAName))
	Expect(configured).To(ContainElement(ifaceBName))

	// Update metadata (same name, different data)
	success = bdIndex.UpdateMetadata(bdUpdt1.Name, l2idx.NewBDMetadata(bdUpdt1, []string{ifaceCName}))
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	_, metadata, found = nameToIdx.LookupIdx(bd.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	bdData, ok = metadata.(*l2idx.BdMetadata)
	Expect(ok).To(BeTrue())
	Expect(bdData).ToNot(BeNil())
	Expect(bdData.BridgeDomain).ToNot(BeNil())
	Expect(bdData.BridgeDomain.Interfaces).To(HaveLen(1))
	Expect(bdData.ConfiguredInterfaces).To(HaveLen(1))

	ifNames = []string{}
	for _, ifData := range bdData.BridgeDomain.Interfaces {
		ifNames = append(ifNames, ifData.Name)
	}
	Expect(ifNames).To(ContainElement(ifaceCName))

	configured = []string{}
	for _, confIf := range bdData.ConfiguredInterfaces {
		configured = append(configured, confIf)
	}
	Expect(configured).To(ContainElement(ifaceCName))

	// Update metadata again
	success = bdIndex.UpdateMetadata(bdUpdt2.Name, l2idx.NewBDMetadata(bdUpdt2, []string{ifaceDName}))
	Expect(success).To(BeTrue())

	// Evaluate updated metadata
	_, metadata, found = nameToIdx.LookupIdx(bd.Name)
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())

	bdData, ok = metadata.(*l2idx.BdMetadata)
	Expect(ok).To(BeTrue())
	Expect(bdData).ToNot(BeNil())
	Expect(bdData.BridgeDomain).ToNot(BeNil())
	Expect(bdData.BridgeDomain.Interfaces).To(HaveLen(1))

	ifNames = []string{}
	for _, ifData := range bdData.BridgeDomain.Interfaces {
		ifNames = append(ifNames, ifData.Name)
	}
	Expect(ifNames).To(ContainElement(ifaceDName))

	configured = []string{}
	for _, confIf := range bdData.ConfiguredInterfaces {
		configured = append(configured, confIf)
	}
	Expect(configured).To(ContainElement(ifaceDName))

	// Unregister
	bdIndex.UnregisterName(bd.Name)

	// Evaluate unregistration
	names = nameToIdx.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests index mapping clear
func TestClearBD(t *testing.T) {
	mapping, index, _ := testInitialization(t, nil)

	// Register entries
	index.RegisterName("bd1", 0, nil)
	index.RegisterName("bd2", 1, nil)
	index.RegisterName("bd3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

/**
TestLookupIndex tests method:
* LookupIndex
*/
func TestLookupIndex(t *testing.T) {
	RegisterTestingT(t)

	_, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
	})

	bdIndex.RegisterName(bridgeDomains[0].Name, idx0, l2idx.NewBDMetadata(bridgeDomains[0], []string{ifaceAName, ifaceBName}))

	foundIdx, metadata, exist := bdIndex.LookupIdx(bdName0)
	Expect(exist).To(BeTrue())
	Expect(foundIdx).To(Equal(idx0))
	Expect(metadata).ToNot(BeNil())
	Expect(metadata.BridgeDomain).To(Equal(bridgeDomains[0]))
	Expect(metadata.ConfiguredInterfaces).To(HaveLen(2))
}

/**
TestLookupIndex tests method:
* LookupIndex
*/
func TestLookupName(t *testing.T) {
	RegisterTestingT(t)

	_, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
	})

	bdIndex.RegisterName(bridgeDomains[0].Name, idx0, l2idx.NewBDMetadata(bridgeDomains[0], []string{ifaceAName, ifaceBName}))

	foundName, metadata, exist := bdIndex.LookupName(idx0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(bridgeDomains[0].Name))
	Expect(metadata).ToNot(BeNil())
	Expect(metadata.BridgeDomain).To(Equal(bridgeDomains[0]))
	Expect(metadata.ConfiguredInterfaces).To(HaveLen(2))
}

/**
TestLookupNameByIfaceName tests method:
* LookupNameByIfaceName
*/
func TestLookupByIfaceName(t *testing.T) {
	RegisterTestingT(t)

	// Define 3 bridge domains
	_, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
		bdName1: {ifaceCName},
		bdName2: {ifaceDName},
	})

	// Assign correct index to every bridge domain
	for _, bridgeDomain := range bridgeDomains {
		if bridgeDomain.Name == bdName0 {
			bdIndex.RegisterName(bridgeDomain.Name, idx0, l2idx.NewBDMetadata(bridgeDomain, []string{ifaceAName, ifaceBName}))
		} else if bridgeDomain.Name == bdName1 {
			bdIndex.RegisterName(bridgeDomain.Name, idx1, l2idx.NewBDMetadata(bridgeDomain, []string{ifaceCName}))
		} else {
			bdIndex.RegisterName(bridgeDomain.Name, idx2, l2idx.NewBDMetadata(bridgeDomain, []string{ifaceDName}))
		}
	}

	// Return all bridge domains to which ifaceAName belongs
	bdIdx, _, _, exists := bdIndex.LookupBdForInterface(ifaceAName)
	Expect(exists).To(BeTrue())
	Expect(bdIdx).To(BeEquivalentTo(0))

	bdIdx, _, _, exists = bdIndex.LookupBdForInterface(ifaceBName)
	Expect(exists).To(BeTrue())
	Expect(bdIdx).To(BeEquivalentTo(0))

	bdIdx, _, _, exists = bdIndex.LookupBdForInterface(ifaceCName)
	Expect(exists).To(BeTrue())
	Expect(bdIdx).To(BeEquivalentTo(1))

	bdIdx, _, _, exists = bdIndex.LookupBdForInterface(ifaceDName)
	Expect(exists).To(BeTrue())
	Expect(bdIdx).To(BeEquivalentTo(2))

	_, _, _, exists = bdIndex.LookupBdForInterface("")
	Expect(exists).To(BeFalse())
}

/**
LookupConfiguredIfsForBd tests method:
* LookupConfiguredIfsForBd
*/
func TestLookupConfiguredIfsForBd(t *testing.T) {
	RegisterTestingT(t)

	// Define 3 bridge domains
	_, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
		bdName1: {ifaceCName},
		bdName2: {ifaceDName},
	})

	// Assign correct index to every bridge domain
	for _, bridgeDomain := range bridgeDomains {
		if bridgeDomain.Name == bdName0 {
			bdIndex.RegisterName(bridgeDomain.Name, idx0, l2idx.NewBDMetadata(bridgeDomain, []string{ifaceAName, ifaceBName}))
		} else if bridgeDomain.Name == bdName1 {
			bdIndex.RegisterName(bridgeDomain.Name, idx1, l2idx.NewBDMetadata(bridgeDomain, []string{ifaceCName}))
		} else {
			bdIndex.RegisterName(bridgeDomain.Name, idx2, nil)
		}
	}

	// Return correct list of configured interfaces for every bridge domain
	_, bdMeta, exists := bdIndex.LookupIdx(bdName0)
	Expect(exists).To(BeTrue())
	Expect(bdMeta.ConfiguredInterfaces).To(HaveLen(2))
	Expect(bdMeta.ConfiguredInterfaces).To(ContainElement(ifaceAName))
	Expect(bdMeta.ConfiguredInterfaces).To(ContainElement(ifaceBName))

	_, bdMeta, exists = bdIndex.LookupIdx(bdName1)
	Expect(exists).To(BeTrue())
	Expect(bdMeta.ConfiguredInterfaces).To(HaveLen(1))
	Expect(bdMeta.ConfiguredInterfaces).To(ContainElement(ifaceCName))

	_, bdMeta, exists = bdIndex.LookupIdx(bdName2)
	Expect(exists).To(BeTrue())
	Expect(bdMeta).To(BeNil())

	_, bdMeta, exists = bdIndex.LookupIdx("")
	Expect(exists).To(BeFalse())
	Expect(bdMeta).To(BeNil())
}

func TestWatchNameToIdx(t *testing.T) {
	RegisterTestingT(t)

	_, bdIndex, bridgeDomains := testInitialization(t, map[string][]string{
		bdName0: {ifaceAName, ifaceBName},
	})

	c := make(chan l2idx.BdChangeDto)
	bdIndex.WatchNameToIdx("testName", c)

	bdIndex.RegisterName(bridgeDomains[0].Name, idx0, l2idx.NewBDMetadata(bridgeDomains[0], []string{ifaceAName, ifaceBName}))

	var dto l2idx.BdChangeDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(bridgeDomains[0].Name))
	Expect(dto.Metadata).ToNot(BeNil())
	Expect(dto.Metadata.BridgeDomain).To(Equal(bridgeDomains[0]))
	Expect(dto.Metadata.ConfiguredInterfaces).To(HaveLen(2))
}
