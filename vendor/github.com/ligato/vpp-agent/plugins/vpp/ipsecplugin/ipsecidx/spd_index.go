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

package ipsecidx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/plugins/vpp/model/ipsec"
)

// SPDIndex provides read-only access to mapping between SPD data and SPD names
type SPDIndex interface {
	// GetMapping returns internal read-only mapping with metadata of ipsec.SecurityPolicyDatabases_SPD type.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool)

	// LookupByInterface returns structure with SPD interface assignment data.
	LookupByInterface(ifName string) []SPDEntry

	// LookupBySA returns structure with matched SPD entries.
	LookupBySA(saName string) []SPDEntry
}

// SPDIndexRW is mapping between SPD data (metadata) and SPD entry names.
type SPDIndexRW interface {
	SPDIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *ipsec.SecurityPolicyDatabases_SPD)

	// UnregisterName removes an item identified by name from mapping
	UnregisterName(name string) (idx uint32, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool)

	// Clear removes all SPD entries from the mapping.
	Clear()
}

// spdIndex is type-safe implementation of mapping between spdID and SPD data.
type spdIndex struct {
	mapping idxvpp.NameToIdxRW
}

// NewSPDIndex creates new instance of spdIndex.
func NewSPDIndex(mapping idxvpp.NameToIdxRW) SPDIndexRW {
	return &spdIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the linuxArpIndex.
func (index *spdIndex) GetMapping() idxvpp.NameToIdxRW {
	return index.mapping
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (index *spdIndex) LookupIdx(name string) (idx uint32, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool) {
	idx, meta, exists := index.mapping.LookupIdx(name)
	if exists {
		metadata = index.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (index *spdIndex) LookupName(idx uint32) (name string, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool) {
	name, meta, exists := index.mapping.LookupName(idx)
	if exists {
		metadata = index.castMetadata(meta)
	}
	return name, metadata, exists
}

// SPDEntry is used for matched SPD entries
type SPDEntry struct {
	SPD   *ipsec.SecurityPolicyDatabases_SPD
	SpdID uint32
}

// LookupSPDInterfaceAssignments returns all SPD interface assignments related to the provided interface
func (index *spdIndex) LookupByInterface(ifName string) (list []SPDEntry) {
	for _, spdName := range index.mapping.ListNames() {
		spdID, spd, found := index.LookupIdx(spdName)
		if found && spd != nil {
			for _, iface := range spd.Interfaces {
				if iface.Name == ifName {
					list = append(list, SPDEntry{spd, spdID})
				}
			}
		}
	}
	return
}

// LookupSPDBySA returns all SPDs related to the provided SA name.
func (index *spdIndex) LookupBySA(saName string) (list []SPDEntry) {
	for _, spdName := range index.mapping.ListNames() {
		spdID, spd, found := index.LookupIdx(spdName)
		if found && spd != nil {
			for _, entry := range spd.PolicyEntries {
				if entry.Sa == saName {
					list = append(list, SPDEntry{spd, spdID})
				}
			}
		}
	}
	return
}

// RegisterName adds new item into name-to-index mapping.
func (index *spdIndex) RegisterName(name string, idx uint32, metadata *ipsec.SecurityPolicyDatabases_SPD) {
	index.mapping.RegisterName(name, idx, metadata)
}

// UnregisterName removes an item identified by name from mapping
func (index *spdIndex) UnregisterName(name string) (idx uint32, metadata *ipsec.SecurityPolicyDatabases_SPD, exists bool) {
	idx, meta, exists := index.mapping.UnregisterName(name)
	return idx, index.castMetadata(meta), exists
}

// Clear removes all SPD entries from the cache.
func (index *spdIndex) Clear() {
	index.mapping.Clear()
}

func (index *spdIndex) castMetadata(meta interface{}) *ipsec.SecurityPolicyDatabases_SPD {
	if ifMeta, ok := meta.(*ipsec.SecurityPolicyDatabases_SPD); ok {
		return ifMeta
	}
	return nil
}
