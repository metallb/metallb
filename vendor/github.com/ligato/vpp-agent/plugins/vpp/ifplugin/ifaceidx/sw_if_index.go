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

package ifaceidx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// SwIfIndex provides read-only access to mapping between software interface indices
// (used internally in VPP) and interface names.
type SwIfIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *intf.Interfaces_Interface, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *intf.Interfaces_Interface, exists bool)

	// LookupNameByIP returns name of items, that contains given IP address in metadata.
	LookupNameByIP(ip string) []string

	// WatchNameToIdx allows to subscribe for watching changes in swIfIndex mapping.
	WatchNameToIdx(subscriber string, pluginChannel chan SwIfIdxDto)
}

// SwIfIndexRW is mapping between software interface indices
// (used internally in VPP) and interface names.
type SwIfIndexRW interface {
	SwIfIndex

	// RegisterName adds a new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *intf.Interfaces_Interface)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *intf.Interfaces_Interface, exists bool)

	// UpdateMetadata updates metadata in existing interface entry.
	UpdateMetadata(name string, metadata *intf.Interfaces_Interface) (success bool)

	// Clear removes all DHCP entries from the mapping.
	Clear()
}

// swIfIndex is type-safe implementation of mapping between Software interface index
// and interface name. It holds metadata of type *InterfaceMeta as well.
type swIfIndex struct {
	mapping idxvpp.NameToIdxRW
}

// SwIfIdxDto represents an item sent through watch channel in swIfIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type SwIfIdxDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *intf.Interfaces_Interface
}

const (
	ipAddressIndexKey = "ipAddrKey"
)

// NewSwIfIndex creates new instance of swIfIndex.
func NewSwIfIndex(mapping idxvpp.NameToIdxRW) SwIfIndexRW {
	return &swIfIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the swIfIndex.
func (swi *swIfIndex) GetMapping() idxvpp.NameToIdxRW {
	return swi.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (swi *swIfIndex) RegisterName(name string, idx uint32, ifMeta *intf.Interfaces_Interface) {
	swi.mapping.RegisterName(name, idx, ifMeta)
}

// IndexMetadata creates indexes for metadata. Index for IPAddress will be created.
func IndexMetadata(metaData interface{}) map[string][]string {
	indexes := map[string][]string{}
	ifMeta, ok := metaData.(*intf.Interfaces_Interface)
	if !ok || ifMeta == nil {
		return indexes
	}

	ip := ifMeta.IpAddresses
	if ip != nil {
		indexes[ipAddressIndexKey] = ip
	}
	return indexes
}

// UnregisterName removes an item identified by name from mapping.
func (swi *swIfIndex) UnregisterName(name string) (idx uint32, metadata *intf.Interfaces_Interface, exists bool) {
	idx, meta, exists := swi.mapping.UnregisterName(name)
	return idx, swi.castMetadata(meta), exists
}

// UpdateMetadata updates metadata in existing interface entry.
func (swi *swIfIndex) UpdateMetadata(name string, metadata *intf.Interfaces_Interface) (success bool) {
	return swi.mapping.UpdateMetadata(name, metadata)
}

// Clear removes all interface entries from the cache.
func (swi *swIfIndex) Clear() {
	swi.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (swi *swIfIndex) LookupIdx(name string) (idx uint32, metadata *intf.Interfaces_Interface, exists bool) {
	idx, meta, exists := swi.mapping.LookupIdx(name)
	if exists {
		metadata = swi.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (swi *swIfIndex) LookupName(idx uint32) (name string, metadata *intf.Interfaces_Interface, exists bool) {
	name, meta, exists := swi.mapping.LookupName(idx)
	if exists {
		metadata = swi.castMetadata(meta)
	}
	return name, metadata, exists
}

// LookupNameByIP returns names of items that contain given IP address in metadata.
func (swi *swIfIndex) LookupNameByIP(ip string) []string {
	return swi.mapping.LookupNameByMetadata(ipAddressIndexKey, ip)
}

func (swi *swIfIndex) castMetadata(meta interface{}) *intf.Interfaces_Interface {
	if ifMeta, ok := meta.(*intf.Interfaces_Interface); ok {
		return ifMeta
	}

	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in swIfIndex mapping.
func (swi *swIfIndex) WatchNameToIdx(subscriber string, pluginChannel chan SwIfIdxDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	swi.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- SwIfIdxDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                swi.castMetadata(c.Metadata),
			}

		}
	}()
}
