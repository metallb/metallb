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
	"time"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging"

	"github.com/ligato/vpp-agent/pkg/idxvpp2"
)

// IfaceMetadataIndex provides read-only access to mapping with VPP interface
// metadata. It extends from NameToIndex.
type IfaceMetadataIndex interface {
	// LookupByName retrieves a previously stored metadata of interface
	// identified by <name>. If there is no interface associated with the give
	// name in the mapping, the <exists> is returned as *false* and <metadata>
	// as *nil*.
	LookupByName(name string) (metadata *IfaceMetadata, exists bool)

	// LookupBySwIfIndex retrieves a previously stored interface identified in
	// VPP by the given <swIfIndex>.
	// If there is no interface associated with the given index, <exists> is returned
	// as *false* with <name> and <metadata> both set to empty values.
	LookupBySwIfIndex(swIfIndex uint32) (name string, metadata *IfaceMetadata, exists bool)

	// LookupByIP returns a list of interfaces that have the given IP address
	// assigned.
	LookupByIP(ip string) []string /* name */

	// ListAllInterfaces returns slice of names of all interfaces in the mapping.
	ListAllInterfaces() (names []string)

	// WatchInterfaces allows to subscribe to watch for changes in the mapping
	// if interface metadata.
	WatchInterfaces(subscriber string, channel chan<- IfaceMetadataDto)
}

// IfaceMetadataIndexRW provides read-write access to mapping with interface
// metadata.
type IfaceMetadataIndexRW interface {
	IfaceMetadataIndex
	idxmap.NamedMappingRW
}

// IfaceMetadata collects metadata for VPP interface used in secondary lookups.
type IfaceMetadata struct {
	SwIfIndex     uint32
	Vrf           uint32
	IPAddresses   []string
	TAPHostIfName string /* host interface name set for the Linux-side of the TAP interface; empty for non-TAPs */
}

// GetIndex returns sw_if_index assigned to the interface.
func (ifm *IfaceMetadata) GetIndex() uint32 {
	return ifm.SwIfIndex
}

// IfaceMetadataDto represents an item sent through watch channel in IfaceMetadataIndex.
// In contrast to NamedMappingGenericEvent, it contains typed interface metadata.
type IfaceMetadataDto struct {
	idxmap.NamedMappingEvent
	Metadata *IfaceMetadata
}

// ifaceMetadataIndex is type-safe implementation of mapping between interface
// name and metadata of type *InterfaceMeta.
type ifaceMetadataIndex struct {
	idxmap.NamedMappingRW /* embeds */

	log         logging.Logger
	nameToIndex idxvpp2.NameToIndex /* contains */
}

const (
	// ipAddressIndexKey is a secondary index for IP-based look-ups.
	ipAddressIndexKey = "ip_addresses"
)

// NewIfaceIndex creates a new instance implementing IfaceMetadataIndexRW.
func NewIfaceIndex(logger logging.Logger, title string) IfaceMetadataIndexRW {
	mapping := idxvpp2.NewNameToIndex(logger, title, indexMetadata)
	return &ifaceMetadataIndex{
		NamedMappingRW: mapping,
		log:            logger,
		nameToIndex:    mapping,
	}
}

// LookupByName retrieves a previously stored metadata of interface
// identified by <name>. If there is no interface associated with the give
// name in the mapping, the <exists> is returned as *false* and <metadata>
// as *nil*.
func (ifmx *ifaceMetadataIndex) LookupByName(name string) (metadata *IfaceMetadata, exists bool) {
	meta, found := ifmx.GetValue(name)
	if found {
		if typedMeta, ok := meta.(*IfaceMetadata); ok {
			return typedMeta, found
		}
	}
	return nil, false
}

// LookupBySwIfIndex retrieves a previously stored interface identified in
// VPP by the given/ <swIfIndex>.
// If there is no interface associated with the given index, <exists> is returned
// as *false* with <name> and <metadata> both set to empty values.
func (ifmx *ifaceMetadataIndex) LookupBySwIfIndex(swIfIndex uint32) (name string, metadata *IfaceMetadata, exists bool) {
	var item idxvpp2.WithIndex
	name, item, exists = ifmx.nameToIndex.LookupByIndex(swIfIndex)
	if exists {
		var isIfaceMeta bool
		metadata, isIfaceMeta = item.(*IfaceMetadata)
		if !isIfaceMeta {
			exists = false
		}
	}
	return
}

// LookupByIP returns a list of interfaces that have the given IP address
// assigned.
func (ifmx *ifaceMetadataIndex) LookupByIP(ip string) []string {
	return ifmx.ListNames(ipAddressIndexKey, ip)
}

// ListAllInterfaces returns slice of names of all interfaces in the mapping.
func (ifmx *ifaceMetadataIndex) ListAllInterfaces() (names []string) {
	return ifmx.ListAllNames()
}

// WatchInterfaces allows to subscribe to watch for changes in the mapping
// if interface metadata.
func (ifmx *ifaceMetadataIndex) WatchInterfaces(subscriber string, channel chan<- IfaceMetadataDto) {
	watcher := func(dto idxmap.NamedMappingGenericEvent) {
		typedMeta, ok := dto.Value.(*IfaceMetadata)
		if !ok {
			return
		}
		msg := IfaceMetadataDto{
			NamedMappingEvent: dto.NamedMappingEvent,
			Metadata:          typedMeta,
		}
		select {
		case channel <- msg:
		case <-time.After(idxmap.DefaultNotifTimeout):
			ifmx.log.Warn("Unable to deliver notification")
		}
	}
	if err := ifmx.Watch(subscriber, watcher); err != nil {
		ifmx.log.Error(err)
	}
}

// indexMetadata is an index function used for interface metadata.
func indexMetadata(metaData interface{}) map[string][]string {
	indexes := make(map[string][]string)

	ifMeta, ok := metaData.(*IfaceMetadata)
	if !ok || ifMeta == nil {
		return indexes
	}

	ip := ifMeta.IPAddresses
	if ip != nil {
		indexes[ipAddressIndexKey] = ip
	}
	return indexes
}
