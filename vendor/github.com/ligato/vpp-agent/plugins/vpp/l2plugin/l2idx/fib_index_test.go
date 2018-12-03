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

type fibIndexData struct {
	Name string
	*l2.FibTable_FibEntry
}

func fibIndexTestInitialization(t *testing.T) (idxvpp.NameToIdxRW, l2idx.FIBIndexRW, []*fibIndexData) {
	RegisterTestingT(t)

	// create sample data
	toData := map[string]string{
		"fib0": "192.168.0.1",
	}

	// initialize index
	nameToIdx := nametoidx.NewNameToIdx(logrus.DefaultLogger(), "index_test", nil)
	index := l2idx.NewFIBIndex(nameToIdx)
	names := nameToIdx.ListNames()

	// check if names were empty
	Expect(names).To(BeEmpty())

	// data preparation
	var data []*fibIndexData

	for name, address := range toData {
		data = append(data, &fibIndexData{
			Name: name,
			FibTable_FibEntry: &l2.FibTable_FibEntry{
				PhysAddress: address,
			},
		})
	}

	return index.GetMapping(), index, data
}

// Tests registering and unregistering name to index
func TestFIBRegisterAndUnregisterName(t *testing.T) {
	mapping, index, data := fibIndexTestInitialization(t)
	entry := data[0]

	// Register entry
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(entry.Name))

	// Unregister entry
	index.UnregisterName(entry.Name)
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests registering and updating metadata
func TestFIBUpdateMetadata(t *testing.T) {
	mapping, index, data := fibIndexTestInitialization(t)
	entry := data[0]

	// Register entry
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(1))
	Expect(names).To(ContainElement(entry.Name))

	// Check first metadata
	name, metadata, exist := index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(metadata.PhysAddress).To(Equal("192.168.0.1"))

	// Update metadata and check them
	index.UpdateMetadata(name, &l2.FibTable_FibEntry{
		PhysAddress: "192.168.0.2",
	})

	name, metadata, exist = index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(metadata.PhysAddress).To(Equal("192.168.0.2"))
}

// Tests index mapping clear
func TestClearFIB(t *testing.T) {
	mapping, index, _ := testInitialization(t, nil)

	// Register entries
	index.RegisterName("fib1", 0, nil)
	index.RegisterName("fib2", 1, nil)
	index.RegisterName("fib3", 2, nil)
	names := mapping.ListNames()
	Expect(names).To(HaveLen(3))

	// Clear
	index.Clear()
	names = mapping.ListNames()
	Expect(names).To(BeEmpty())
}

// Tests lookup by index
func TestFIBLookupByIndex(t *testing.T) {
	_, index, data := fibIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)

	foundIdx, metadata, exist := index.LookupIdx("fib0")
	Expect(exist).To(BeTrue())
	Expect(foundIdx).To(BeEquivalentTo(0))
	Expect(metadata).To(Equal(entry.FibTable_FibEntry))
}

// Tests lookup by name
func TestFIBLookupByName(t *testing.T) {
	_, index, data := fibIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)

	foundName, metadata, exist := index.LookupName(0)
	Expect(exist).To(BeTrue())
	Expect(foundName).To(Equal(entry.Name))
	Expect(metadata).To(Equal(entry.FibTable_FibEntry))
}

// Tests negative unregister by name
func TestFIBUnregisterByNameNegative(t *testing.T) {
	_, index, data := fibIndexTestInitialization(t)
	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)

	foundIdx, metadata, exist := index.UnregisterName("non-existant")
	Expect(exist).To(BeFalse())
	Expect(foundIdx).To(BeEquivalentTo(0))
	Expect(metadata).To(BeNil())
}

// Tests watch name to index
func TestFIBWatchNameToIdx(t *testing.T) {
	_, index, data := fibIndexTestInitialization(t)

	c := make(chan l2idx.FibChangeDto)
	index.WatchNameToIdx("testName", c)

	entry := data[0]
	index.RegisterName(entry.Name, 0, entry.FibTable_FibEntry)

	var dto l2idx.FibChangeDto
	Eventually(c).Should(Receive(&dto))

	Expect(dto.Name).To(Equal(entry.Name))
	Expect(dto.Metadata).To(Equal(entry.FibTable_FibEntry))
}
