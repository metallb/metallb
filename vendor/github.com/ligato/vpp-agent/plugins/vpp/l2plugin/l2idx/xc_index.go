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

package l2idx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l2"
)

// XcIndex provides read-only access to mapping between indexes (used internally in VPP) and cross connects.
type XcIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *l2.XConnectPairs_XConnectPair, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *l2.XConnectPairs_XConnectPair, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in xcIndex mapping
	WatchNameToIdx(subscriber string, pluginChannel chan XcChangeDto)
}

// XcIndexRW is mapping between indices (used internally in VPP) and cross connect entries.
type XcIndexRW interface {
	XcIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, metadata *l2.XConnectPairs_XConnectPair)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *l2.XConnectPairs_XConnectPair, exists bool)

	// UpdateMetadata updates metadata in existing cross connect entry.
	UpdateMetadata(name string, metadata *l2.XConnectPairs_XConnectPair) (success bool)

	// Clear removes all cross connects from the mapping.
	Clear()
}

// xcIndex is type-safe implementation of mapping between cross connect receive interface and index.
// It holds as well metadata of type *l2.XConnectPairs_XConnectPair.
type xcIndex struct {
	mapping idxvpp.NameToIdxRW
}

// XcChangeDto represents an item sent through watch channel in xcIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type XcChangeDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *l2.XConnectPairs_XConnectPair
}

// NewXcIndex creates new instance of xcIndex.
func NewXcIndex(mapping idxvpp.NameToIdxRW) XcIndexRW {
	return &xcIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the xcIndex.
func (xc *xcIndex) GetMapping() idxvpp.NameToIdxRW {
	return xc.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (xc *xcIndex) RegisterName(name string, idx uint32, ifMeta *l2.XConnectPairs_XConnectPair) {
	xc.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping.
func (xc *xcIndex) UnregisterName(name string) (idx uint32, metadata *l2.XConnectPairs_XConnectPair, exists bool) {
	idx, meta, exists := xc.mapping.UnregisterName(name)
	return idx, castXcMetadata(meta), exists
}

// UpdateMetadata updates metadata in existing cross connect entry.
func (xc *xcIndex) UpdateMetadata(name string, metadata *l2.XConnectPairs_XConnectPair) (success bool) {
	return xc.mapping.UpdateMetadata(name, metadata)
}

// Clear removes all cross connects from the cache.
func (xc *xcIndex) Clear() {
	xc.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (xc *xcIndex) LookupIdx(name string) (idx uint32, metadata *l2.XConnectPairs_XConnectPair, exists bool) {
	idx, meta, exists := xc.mapping.LookupIdx(name)
	if exists {
		metadata = castXcMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (xc *xcIndex) LookupName(idx uint32) (name string, metadata *l2.XConnectPairs_XConnectPair, exists bool) {
	name, meta, exists := xc.mapping.LookupName(idx)
	if exists {
		metadata = castXcMetadata(meta)
	}
	return name, metadata, exists
}

// WatchNameToIdx allows to subscribe for watching changes in xcIndex mapping.
func (xc *xcIndex) WatchNameToIdx(subscriber string, pluginChannel chan XcChangeDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	xc.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- XcChangeDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                castXcMetadata(c.Metadata),
			}

		}
	}()
}

func castXcMetadata(meta interface{}) *l2.XConnectPairs_XConnectPair {
	ifMeta, ok := meta.(*l2.XConnectPairs_XConnectPair)
	if !ok {
		return nil
	}
	return ifMeta
}
