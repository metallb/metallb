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

type xcIndexData struct {
	Name string
	*l2.XConnectPairs_XConnectPair
}

func xcIndexTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, l2idx.XcIndexRW, []*xcIndexData) {
	RegisterTestingT(t)

	// create sample data
	toData := map[string]string{
		"xc0": "if0",
	}

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "index_test", nil)
	index := l2idx.NewXcIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	// data preparation
	var data []*xcIndexData

	for name, intface := range toData {
		data = append(data, &xcIndexData{
			Name: name,
			XConnectPairs_XConnectPair: &l2.XConnectPairs_XConnectPair{
				ReceiveInterface: intface,
			},
		})
	}

	return index.GetMapping(), index, data
}

// Tests registering and unregistering name to index
func TestXCRegisterAndUnregisterName(t *testing.T) {
	mapping, index, data := xcIndexTestInitialization(t)
	entry := data[0]

	// Register entry
	index.RegisterName(entry.Name, 0, entry.XConnectPairs_XConnectPair)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(entry.Name))

	// Unregister entry
	index.UnregisterName(entry.Name)
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests registering and updating metadata
func TestXCUpdateMetadata(t *testing.T) {
	mapping, index, data := xcIndexTestInitialization(t)
	entry := data[0]

	// Register entry
	index.RegisterName(entry.Name, 0, entry.XConnectPairs_XConnectPair)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(entry.Name))

	// Check first metadata
	name, metadata, exist := index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(metadata.ReceiveInterface).To(Equal("if0"))

	// Update metadata and check them
	index.UpdateMetadata(name, &l2.XConnectPairs_XConnectPair{
		ReceiveInterface: "if1",
	})

	name, metadata, exist = index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(metadata.ReceiveInterface).To(Equal("if1"))
}

// Tests index mapping clear
func TestXCClear(t *testing.T) {
	mapping, index, _ := testInitialization(t, nil)

	// Register entries
	index.RegisterName("xc1", 0, nil)
	index.RegisterName("xc2", 1, nil)
	index.RegisterName("xc3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests lookup by index
func TestXCLookupByIndex(t *testing.T) {
	_, index, data := xcIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.XConnectPairs_XConnectPair)

	foundIdx, metadata, exist := index.LookupIdx("xc0")
	Expect(exist).To(BeTrue())
	Expect(foundIdx).To(BeEquivalentTo(0))
	Expect(metadata).To(Equal(entry.XConnectPairs_XConnectPair))
}

// Tests lookup by name
func TestDHCPLookupByName(t *testing.T) {
	_, index, data := fibIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)

	foundName, metadata, exist := index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(entry.Name))
	Expect(metadata).To(Equal(entry.FibTable_FibEntry))
}

// Tests negative unregister by name
func TestXCUnregisterByNameNegative(t *testing.T) {
	_, index, data := xcIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.XConnectPairs_XConnectPair)

	foundIdx, metadata, exist := index.UnregisterName("non-existant")
	Expect(exist).To(BeFalse())
	Expect(foundIdx).To(BeEquivalentTo(0))
	Expect(metadata).To(BeNil())
}

// Tests watch name to index
func TestXCWatchNameToIdx(t *testing.T) {
	_, index, data := xcIndexTestInitialization(t)

	c := make(chan l2idx.XcChangeDto)
	index.WatchNameToIdx("testName", c)

	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.XConnectPairs_XConnectPair)

	var dto l2idx.XcChangeDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(entry.Name))
	Expect(dto.Metadata).To(Equal(entry.XConnectPairs_XConnectPair))
}
