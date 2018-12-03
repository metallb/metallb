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
	. "github.com/onsi/gomega"
)

type dhcpData struct {
	Name     string
	Settings *ifaceidx.DHCPSettings
}

func dhcpTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, ifaceidx.DhcpIndexRW, []*dhcpData) {
	RegisterTestingT(t)

	// create sample data
	toData := map[string]string{
		"dhcp0": "if0",
	}

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "dhcp_index_test", ifaceidx.IndexDHCPMetadata)
	index := ifaceidx.NewDHCPIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	// data preparation
	var data []*dhcpData

	for name, ifname := range toData {
		data = append(data, &dhcpData{
			Name: name,
			Settings: &ifaceidx.DHCPSettings{
				IfName: ifname,
			},
		})
	}

	return index.GetMapping(), index, data
}

// TestIndexMetadata tests whether func IndexMetadata return map filled with correct values
func TestDHCPIndexMetadata(t *testing.T) {
	_, _, data := dhcpTestInitialization(t)
	dhcp := data[0]

	result := ifaceidx.IndexDHCPMetadata(nil)
	Expect(result).To(HaveLen(0))

	// result of this call is always empty map
	result = ifaceidx.IndexDHCPMetadata(dhcp.Settings)
	Expect(result).To(HaveLen(0))
}

// Tests registering and unregistering name to index
func TestDHCPRegisterAndUnregisterName(t *testing.T) {
	mapping, index, data := dhcpTestInitialization(t)
	dhcp := data[0]

	// Register if0
	index.RegisterName(dhcp.Name, 0, dhcp.Settings)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(dhcp.Name))

	// Unregister if0
	index.UnregisterName(dhcp.Name)
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests index mapping clear
func TestClearDHCP(t *testing.T) {
	mapping, index, _ := dhcpTestInitialization(t)

	// Register entries
	index.RegisterName("dhcp1", 0, nil)
	index.RegisterName("dhcp2", 1, nil)
	index.RegisterName("dhcp3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests lookup by index
func TestDHCPLookupByIndex(t *testing.T) {
	_, index, data := dhcpTestInitialization(t)
	dhcp := data[0]
	index.RegisterName(dhcp.Name, 0, dhcp.Settings)

	foundIdx, metadata, exist := index.LookupIdx("dhcp0")
	Expect(exist).To(BeTrue())
	Expect(foundIdx).To(Equal(uint32(0)))
	Expect(metadata).To(Equal(dhcp.Settings))
}

// Tests lookup by name
func TestDHCPLookupByName(t *testing.T) {
	_, index, data := dhcpTestInitialization(t)
	dhcp := data[0]
	index.RegisterName(dhcp.Name, 0, dhcp.Settings)

	foundName, metadata, exist := index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(dhcp.Name))
	Expect(metadata).To(Equal(dhcp.Settings))
}

// Tests watch name to index
func TestDHCPWatchNameToIdx(t *testing.T) {
	_, index, data := dhcpTestInitialization(t)

	c := make(chan ifaceidx.DhcpIdxDto)
	index.WatchNameToIdx("testName", c)

	dhcp := data[0]
	index.RegisterName(dhcp.Name, 0, dhcp.Settings)

	var dto ifaceidx.DhcpIdxDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(dhcp.Name))
	Expect(dto.Metadata).To(Equal(dhcp.Settings))
}
