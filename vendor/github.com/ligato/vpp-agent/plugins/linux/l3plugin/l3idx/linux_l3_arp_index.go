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
	"github.com/ligato/vpp-agent/plugins/linux/model/l3"
)

const hostARPNameKey = "hostARPName"

// LinuxARPIndex provides read-only access to mapping between software ARP indexes and ARP names
type LinuxARPIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool)

	// LookupNameByHostIfName looks up the interface identified by the name used in HostOs
	LookupNameByHostIfName(hostIfName string) []string

	// WatchNameToIdx allows to subscribe for watching changes in linuxIfIndex mapping
	WatchNameToIdx(subscriber string, pluginChannel chan LinuxARPIndexDto)
}

// LinuxARPIndexRW is mapping between software ARP indexes (used internally in VPP)
// and ARP entry names.
type LinuxARPIndexRW interface {
	LinuxARPIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *l3.LinuxStaticArpEntries_ArpEntry)

	// UnregisterName removes an item identified by name from mapping
	UnregisterName(name string) (idx uint32, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool)
}

// LinuxARPIndexDto represents an item sent through watch channel in linuxARPIndex.
// In contrast to NameToIdxDto it contains typed metadata.
type LinuxARPIndexDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *l3.LinuxStaticArpEntries_ArpEntry
}

// linuxArpIndex is type-safe implementation of mapping between Software ARP index
// and ARP name.
type linuxArpIndex struct {
	mapping idxvpp.NameToIdxRW
}

// NewLinuxARPIndex creates new instance of linuxArpIndex.
func NewLinuxARPIndex(mapping idxvpp.NameToIdxRW) LinuxARPIndexRW {
	return &linuxArpIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the linuxArpIndex.
func (linuxArpIndex *linuxArpIndex) GetMapping() idxvpp.NameToIdxRW {
	return linuxArpIndex.mapping
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (linuxArpIndex *linuxArpIndex) LookupIdx(name string) (idx uint32, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool) {
	idx, meta, exists := linuxArpIndex.mapping.LookupIdx(name)
	if exists {
		metadata = linuxArpIndex.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (linuxArpIndex *linuxArpIndex) LookupName(idx uint32) (name string, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool) {
	name, meta, exists := linuxArpIndex.mapping.LookupName(idx)
	if exists {
		metadata = linuxArpIndex.castMetadata(meta)
	}
	return name, metadata, exists
}

// LookupNameByIP returns names of items that contains given IP address in metadata
func (linuxArpIndex *linuxArpIndex) LookupNameByHostIfName(hostARPName string) []string {
	return linuxArpIndex.mapping.LookupNameByMetadata(hostARPNameKey, hostARPName)
}

// RegisterName adds new item into name-to-index mapping.
func (linuxArpIndex *linuxArpIndex) RegisterName(name string, idx uint32, ifMeta *l3.LinuxStaticArpEntries_ArpEntry) {
	linuxArpIndex.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping
func (linuxArpIndex *linuxArpIndex) UnregisterName(name string) (idx uint32, metadata *l3.LinuxStaticArpEntries_ArpEntry, exists bool) {
	idx, meta, exists := linuxArpIndex.mapping.UnregisterName(name)
	return idx, linuxArpIndex.castMetadata(meta), exists
}

// WatchNameToIdx allows to subscribe for watching changes in linuxIfIndex mapping
func (linuxArpIndex *linuxArpIndex) WatchNameToIdx(subscriber string, pluginChannel chan LinuxARPIndexDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	linuxArpIndex.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- LinuxARPIndexDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                linuxArpIndex.castMetadata(c.Metadata),
			}

		}
	}()
}

func (linuxArpIndex *linuxArpIndex) castMetadata(meta interface{}) *l3.LinuxStaticArpEntries_ArpEntry {
	if ifMeta, ok := meta.(*l3.LinuxStaticArpEntries_ArpEntry); ok {
		return ifMeta
	}

	return nil
}
