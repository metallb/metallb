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

// FIBIndex provides read-only access to mapping between indexes (used internally in VPP) and FIB entries.
type FIBIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *l2.FibTable_FibEntry, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *l2.FibTable_FibEntry, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in fibIndex mapping
	WatchNameToIdx(subscriber string, pluginChannel chan FibChangeDto)
}

// FIBIndexRW is mapping between indices (used internally in VPP) and FIB entries.
type FIBIndexRW interface {
	FIBIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, metadata *l2.FibTable_FibEntry)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *l2.FibTable_FibEntry, exists bool)

	// UpdateMetadata updates metadata in existing FIB entry.
	UpdateMetadata(name string, metadata *l2.FibTable_FibEntry) (success bool)

	// Clear removes all FIB entries from the mapping.
	Clear()
}

// fibIndex is type-safe implementation of mapping between FIB physical address and index.
// It holds as well metadata of type *l2.FibTableEntries_FibTableEntry.
type fibIndex struct {
	mapping idxvpp.NameToIdxRW
}

// FibChangeDto represents an item sent through watch channel in fibIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type FibChangeDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *l2.FibTable_FibEntry
}

// NewFIBIndex creates new instance of fibIndex.
func NewFIBIndex(mapping idxvpp.NameToIdxRW) FIBIndexRW {
	return &fibIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the fibIndex.
func (fib *fibIndex) GetMapping() idxvpp.NameToIdxRW {
	return fib.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (fib *fibIndex) RegisterName(name string, idx uint32, ifMeta *l2.FibTable_FibEntry) {
	fib.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping.
func (fib *fibIndex) UnregisterName(name string) (idx uint32, metadata *l2.FibTable_FibEntry, exists bool) {
	idx, meta, exists := fib.mapping.UnregisterName(name)
	return idx, castFibMetadata(meta), exists
}

// UpdateMetadata updates metadata in existing FIB entry.
func (fib *fibIndex) UpdateMetadata(name string, metadata *l2.FibTable_FibEntry) (success bool) {
	return fib.mapping.UpdateMetadata(name, metadata)
}

// Clear removes all FIB entries from the cache.
func (fib *fibIndex) Clear() {
	fib.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (fib *fibIndex) LookupIdx(name string) (idx uint32, metadata *l2.FibTable_FibEntry, exists bool) {
	idx, meta, exists := fib.mapping.LookupIdx(name)
	if exists {
		metadata = castFibMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (fib *fibIndex) LookupName(idx uint32) (name string, metadata *l2.FibTable_FibEntry, exists bool) {
	name, meta, exists := fib.mapping.LookupName(idx)
	if exists {
		metadata = castFibMetadata(meta)
	}
	return name, metadata, exists
}

// WatchNameToIdx allows to subscribe for watching changes in fibIndex mapping.
func (fib *fibIndex) WatchNameToIdx(subscriber string, pluginChannel chan FibChangeDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	fib.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- FibChangeDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                castFibMetadata(c.Metadata),
			}

		}
	}()
}

func castFibMetadata(meta interface{}) *l2.FibTable_FibEntry {
	ifMeta, ok := meta.(*l2.FibTable_FibEntry)
	if !ok {
		return nil
	}
	return ifMeta
}
