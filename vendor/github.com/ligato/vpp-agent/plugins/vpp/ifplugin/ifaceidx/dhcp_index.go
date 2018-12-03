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
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
)

// DHCPSettings contains all DHCP related information. Used as a metadata type for DHCP mapping.
type DHCPSettings struct {
	IfName        string
	IsIPv6        bool
	IPAddress     string
	Mask          uint32
	PhysAddress   string
	RouterAddress string
}

// DhcpIndex provides read-only access to mapping between software interface names, indices
// (used internally in VPP) and DHCP configuration.
type DhcpIndex interface {
	// GetMapping returns internal read-only mapping with metadata.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *DHCPSettings, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *DHCPSettings, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in DhcpIndex mapping.
	WatchNameToIdx(subscriber string, pluginChannel chan DhcpIdxDto)
}

// DhcpIndexRW is mapping between software interface names, indices
// (used internally in VPP) and DHCP configuration.
type DhcpIndexRW interface {
	DhcpIndex

	// RegisterName adds a new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *DHCPSettings)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *DHCPSettings, exists bool)

	// Clear removes all DHCP entries from the mapping.
	Clear()
}

// dhcpIndex is type-safe implementation of mapping. It holds metadata of type *DhcpIndex as well.
type dhcpIndex struct {
	mapping idxvpp.NameToIdxRW
}

// DhcpIdxDto represents an item sent through watch channel in dhcpIfIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type DhcpIdxDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *DHCPSettings
}

// NewDHCPIndex creates new instance of dhcpIndex.
func NewDHCPIndex(mapping idxvpp.NameToIdxRW) DhcpIndexRW {
	return &dhcpIndex{mapping: mapping}
}

// IndexDHCPMetadata creates indexes for metadata.
func IndexDHCPMetadata(metaData interface{}) map[string][]string {
	log.DefaultLogger().Debugf("IndexMetadata: %v", metaData)

	indexes := map[string][]string{}
	ifMeta, ok := metaData.(*DHCPSettings)
	if !ok || ifMeta == nil {
		return indexes
	}

	return indexes
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the dhcpIndex.
func (dhcp *dhcpIndex) GetMapping() idxvpp.NameToIdxRW {
	return dhcp.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (dhcp *dhcpIndex) RegisterName(name string, idx uint32, ifMeta *DHCPSettings) {
	dhcp.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping.
func (dhcp *dhcpIndex) UnregisterName(name string) (idx uint32, metadata *DHCPSettings, exists bool) {
	idx, meta, exists := dhcp.mapping.UnregisterName(name)
	return idx, dhcp.castMetadata(meta), exists
}

// Clear removes all DHCP entries from the mapping.
func (dhcp *dhcpIndex) Clear() {
	dhcp.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (dhcp *dhcpIndex) LookupIdx(name string) (idx uint32, metadata *DHCPSettings, exists bool) {
	idx, meta, exists := dhcp.mapping.LookupIdx(name)
	if exists {
		metadata = dhcp.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (dhcp *dhcpIndex) LookupName(idx uint32) (name string, metadata *DHCPSettings, exists bool) {
	name, meta, exists := dhcp.mapping.LookupName(idx)
	if exists {
		metadata = dhcp.castMetadata(meta)
	}
	return name, metadata, exists
}

// WatchNameToIdx allows to subscribe for watching changes in dhcpIndex mapping.
func (dhcp *dhcpIndex) WatchNameToIdx(subscriber string, pluginChannel chan DhcpIdxDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	dhcp.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- DhcpIdxDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                dhcp.castMetadata(c.Metadata),
			}

		}
	}()
}

func (dhcp *dhcpIndex) castMetadata(meta interface{}) *DHCPSettings {
	if ifMeta, ok := meta.(*DHCPSettings); ok {
		return ifMeta
	}

	return nil
}
