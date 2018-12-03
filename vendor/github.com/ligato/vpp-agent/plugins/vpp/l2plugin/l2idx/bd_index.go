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

package l2idx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// BDIndex provides read-only access to mapping between indices (used internally in VPP) and Bridge Domain names.
type BDIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *BdMetadata, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *BdMetadata, exists bool)

	// LookupBdForInterface looks up for bridge domain the interface belongs to
	LookupBdForInterface(ifName string) (bdIdx uint32, bd *l2.BridgeDomains_BridgeDomain, bdIf *l2.BridgeDomains_BridgeDomain_Interfaces, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in bdIndex mapping
	WatchNameToIdx(subscriber string, pluginChannel chan BdChangeDto)
}

// BDIndexRW is mapping between indices (used internally in VPP) and Bridge Domain names.
type BDIndexRW interface {
	BDIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, metadata *BdMetadata)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *BdMetadata, exists bool)

	// UpdateMetadata updates metadata in existing bridge domain entry.
	UpdateMetadata(name string, metadata *BdMetadata) (success bool)

	// Clear removes all bridge domains from the mapping.
	Clear()
}

// bdIndex is type-safe implementation of mapping between bridge domain name and index.
// It holds as well metadata of type *l2.BridgeDomains_BridgeDomain.
type bdIndex struct {
	mapping idxvpp.NameToIdxRW
}

// BdChangeDto represents an item sent through watch channel in bdIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type BdChangeDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *BdMetadata
}

// BdMetadata is bridge domain metadata and consists from base bridge domain data and a list of interfaces which were
// (according to L2 bridge domain configurator) already configured as a part of bridge domain
type BdMetadata struct {
	BridgeDomain         *l2.BridgeDomains_BridgeDomain
	ConfiguredInterfaces []string
}

const (
	ifaceNameIndexKey = "ipAddrKey" // TODO: interfaces in the bridge domain
)

// NewBDIndex creates new instance of bdIndex.
func NewBDIndex(mapping idxvpp.NameToIdxRW) BDIndexRW {
	return &bdIndex{mapping: mapping}
}

// NewBDMetadata returns new instance of metadata
func NewBDMetadata(bd *l2.BridgeDomains_BridgeDomain, confIfs []string) *BdMetadata {
	return &BdMetadata{BridgeDomain: bd, ConfiguredInterfaces: confIfs}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the bdIndex.
func (bdi *bdIndex) GetMapping() idxvpp.NameToIdxRW {
	return bdi.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (bdi *bdIndex) RegisterName(name string, idx uint32, bdMeta *BdMetadata) {
	bdi.mapping.RegisterName(name, idx, bdMeta)
}

// IndexMetadata creates indices for metadata. Index for IPAddress will be created.
func IndexMetadata(metaData interface{}) map[string][]string {
	indexes := map[string][]string{}

	ifMeta := castBdMetadata(metaData)
	if ifMeta == nil || ifMeta.BridgeDomain == nil {
		return indexes
	}

	var ifacenNames []string
	for _, bdIface := range ifMeta.BridgeDomain.Interfaces {
		if bdIface != nil {
			ifacenNames = append(ifacenNames, bdIface.Name)
		}
	}
	indexes[ifaceNameIndexKey] = ifacenNames

	return indexes
}

// UnregisterName removes an item identified by name from mapping.
func (bdi *bdIndex) UnregisterName(name string) (idx uint32, metadata *BdMetadata, exists bool) {
	idx, meta, exists := bdi.mapping.UnregisterName(name)
	return idx, castBdMetadata(meta), exists
}

// UpdateMetadata updates metadata in existing bridge domain entry.
func (bdi *bdIndex) UpdateMetadata(name string, metadata *BdMetadata) (success bool) {
	return bdi.mapping.UpdateMetadata(name, metadata)
}

// Clear removes all bridge domains from the cache.
func (bdi *bdIndex) Clear() {
	bdi.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (bdi *bdIndex) LookupIdx(name string) (idx uint32, metadata *BdMetadata, exists bool) {
	idx, meta, exists := bdi.mapping.LookupIdx(name)
	if exists {
		metadata = castBdMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (bdi *bdIndex) LookupName(idx uint32) (name string, metadata *BdMetadata, exists bool) {
	name, meta, exists := bdi.mapping.LookupName(idx)
	if exists {
		metadata = castBdMetadata(meta)
	}
	return name, metadata, exists
}

// LookupBdForInterface returns a bridge domain which contains provided interface with bvi/shg details about it
func (bdi *bdIndex) LookupBdForInterface(ifName string) (bdIdx uint32, bd *l2.BridgeDomains_BridgeDomain, bdIf *l2.BridgeDomains_BridgeDomain_Interfaces, exists bool) {
	bdNames := bdi.mapping.ListNames()
	for _, bdName := range bdNames {
		bdIdx, meta, exists := bdi.mapping.LookupIdx(bdName)
		if exists && meta != nil {
			bdMeta := castBdMetadata(meta)
			if bdMeta != nil && bdMeta.BridgeDomain != nil {
				for _, iface := range bdMeta.BridgeDomain.Interfaces {
					if iface.Name == ifName {
						return bdIdx, bdMeta.BridgeDomain, iface, true
					}
				}
			}
		}
	}

	return bdIdx, bd, nil, false
}

// WatchNameToIdx allows to subscribe for watching changes in bdIndex mapping.
func (bdi *bdIndex) WatchNameToIdx(subscriber string, pluginChannel chan BdChangeDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	bdi.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- BdChangeDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                castBdMetadata(c.Metadata),
			}

		}
	}()
}

func castBdMetadata(meta interface{}) *BdMetadata {
	bdMeta, ok := meta.(*BdMetadata)
	if !ok {
		return nil
	}
	return bdMeta
}
