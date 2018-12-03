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

package l3idx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// ARPIndex provides read-only access to mapping between software ARP indexes and ARP names
type ARPIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *l3.ArpTable_ArpEntry, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *l3.ArpTable_ArpEntry, exists bool)

	// LookupNamesByInterface returns names of items that contains given interface name in metadata
	LookupNamesByInterface(ifName string) []*l3.ArpTable_ArpEntry

	// WatchNameToIdx allows to subscribe for watching changes in SwIfIndex mapping
	WatchNameToIdx(subscriber string, pluginChannel chan ARPIndexDto)
}

// ARPIndexRW is mapping between software ARP indexes (used internally in VPP)
// and ARP entry names.
type ARPIndexRW interface {
	ARPIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *l3.ArpTable_ArpEntry)

	// UnregisterName removes an item identified by name from mapping
	UnregisterName(name string) (idx uint32, metadata *l3.ArpTable_ArpEntry, exists bool)

	// Clear removes all ARP entries from the mapping.
	Clear()
}

// ARPIndexDto represents an item sent through watch channel in ARPIndex.
// In contrast to NameToIdxDto it contains typed metadata.
type ARPIndexDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *l3.ArpTable_ArpEntry
}

// ArpIndex is type-safe implementation of mapping between Software ARP index
// and ARP name.
type ArpIndex struct {
	mapping idxvpp.NameToIdxRW
}

// NewARPIndex creates new instance of ArpIndex.
func NewARPIndex(mapping idxvpp.NameToIdxRW) ARPIndexRW {
	return &ArpIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the ArpIndex.
func (arpIndex *ArpIndex) GetMapping() idxvpp.NameToIdxRW {
	return arpIndex.mapping
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (arpIndex *ArpIndex) LookupIdx(name string) (idx uint32, metadata *l3.ArpTable_ArpEntry, exists bool) {
	idx, meta, exists := arpIndex.mapping.LookupIdx(name)
	if exists {
		metadata = arpIndex.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (arpIndex *ArpIndex) LookupName(idx uint32) (name string, metadata *l3.ArpTable_ArpEntry, exists bool) {
	name, meta, exists := arpIndex.mapping.LookupName(idx)
	if exists {
		metadata = arpIndex.castMetadata(meta)
	}
	return name, metadata, exists
}

// LookupNamesByInterface returns all names related to the provided interface
func (arpIndex *ArpIndex) LookupNamesByInterface(ifName string) []*l3.ArpTable_ArpEntry {
	var match []*l3.ArpTable_ArpEntry
	for _, name := range arpIndex.mapping.ListNames() {
		_, meta, found := arpIndex.LookupIdx(name)
		if found && meta != nil && meta.Interface == ifName {
			match = append(match, meta)
		}
	}
	return match
}

// RegisterName adds new item into name-to-index mapping.
func (arpIndex *ArpIndex) RegisterName(name string, idx uint32, ifMeta *l3.ArpTable_ArpEntry) {
	arpIndex.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping
func (arpIndex *ArpIndex) UnregisterName(name string) (idx uint32, metadata *l3.ArpTable_ArpEntry, exists bool) {
	idx, meta, exists := arpIndex.mapping.UnregisterName(name)
	return idx, arpIndex.castMetadata(meta), exists
}

// Clear removes all ARP entries from the cache.
func (arpIndex *ArpIndex) Clear() {
	arpIndex.mapping.Clear()
}

// WatchNameToIdx allows to subscribe for watching changes in SwIfIndex mapping
func (arpIndex *ArpIndex) WatchNameToIdx(subscriber string, pluginChannel chan ARPIndexDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	arpIndex.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- ARPIndexDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                arpIndex.castMetadata(c.Metadata),
			}

		}
	}()
}

func (arpIndex *ArpIndex) castMetadata(meta interface{}) *l3.ArpTable_ArpEntry {
	if ifMeta, ok := meta.(*l3.ArpTable_ArpEntry); ok {
		return ifMeta
	}

	return nil
}
