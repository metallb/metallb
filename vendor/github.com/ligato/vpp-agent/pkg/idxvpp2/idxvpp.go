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

package idxvpp2

import (
	"strconv"
	"time"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"
)

// WithIndex is interface that items with integer handle must implement to get
// indexed by NameToIndex.
type WithIndex interface {
	// GetIndex should return integer handle assigned to the item.
	GetIndex() uint32
}

// NameToIndex is the "user API" to the registry of items with integer handles.
// It provides read-only access intended for plugins that need to do the conversions
// between logical names from NB and VPP/Linux item IDs.
type NameToIndex interface {
	// LookupByName retrieves a previously stored item identified by
	// <name>. If there is no item associated with the give name in the mapping,
	// the <exists> is returned as *false* and <item> as *nil*.
	LookupByName(name string) (item WithIndex, exists bool)

	// LookupByIndex retrieves a previously stored item identified in VPP/Linux
	// by the given <index>.
	// If there is no item associated with the given index, <exists> is returned
	// as *false* with <name> and <item> both set to empty values.
	LookupByIndex(index uint32) (name string, item WithIndex, exists bool)

	// WatchItems subscribes to receive notifications about the changes in the
	// mapping related to items with integer handles.
	WatchItems(subscriber string, channel chan<- NameToIndexDto)
}

// NameToIndexRW is the "owner API" to the NameToIndex registry. Using this
// API the owner is able to add/update and delete associations between logical
// names and VPP/Linux items identified by integer handles.
type NameToIndexRW interface {
	NameToIndex
	idxmap.NamedMappingRW
}

// OnlyIndex can be used to add items into NameToIndex with the integer handle
// as the only information associated with each item.
type OnlyIndex struct {
	Index uint32
}

// GetIndex returns index assigned to the item.
func (item *OnlyIndex) GetIndex() uint32 {
	return item.Index
}

// NameToIndexDto represents an item sent through watch channel in NameToIndex.
// In contrast to NamedMappingGenericEvent, it contains item casted to WithIndex.
type NameToIndexDto struct {
	idxmap.NamedMappingEvent
	Item WithIndex
}

// nameToIndex implements NamedMapping for items with integer handles.
type nameToIndex struct {
	idxmap.NamedMappingRW
	log logging.Logger
}

const (
	// indexKey is a secondary index used to create association between
	// item name and the integer handle.
	indexKey = "index"
)

// NewNameToIndex creates a new instance implementing NameToIndexRW.
// User can optionally extend the secondary indexes through <indexFunction>.
func NewNameToIndex(logger logging.Logger, title string,
	indexFunction mem.IndexFunction) NameToIndexRW {
	return &nameToIndex{
		NamedMappingRW: mem.NewNamedMapping(logger, title,
			func(item interface{}) map[string][]string {
				idxs := internalIndexFunction(item)

				if indexFunction != nil {
					userIdxs := indexFunction(item)
					for k, v := range userIdxs {
						idxs[k] = v
					}
				}
				return idxs
			}),
	}
}

// LookupByName retrieves a previously stored item identified by
// <name>. If there is no item associated with the give name in the mapping,
// the <exists> is returned as *false* and <item> as *nil*.
func (idx *nameToIndex) LookupByName(name string) (item WithIndex, exists bool) {
	value, found := idx.GetValue(name)
	if found {
		if itemWithIndex, ok := value.(WithIndex); ok {
			return itemWithIndex, found
		}
	}
	return nil, false
}

// LookupByIndex retrieves a previously stored item identified in VPP/Linux
// by the given <index>.
// If there is no item associated with the given index, <exists> is returned
// as *false* with <name> and <item> both set to empty values.
func (idx *nameToIndex) LookupByIndex(index uint32) (name string, item WithIndex, exists bool) {
	res := idx.ListNames(indexKey, strconv.FormatUint(uint64(index), 10))
	if len(res) != 1 {
		return
	}
	value, found := idx.GetValue(res[0])
	if found {
		if itemWithIndex, ok := value.(WithIndex); ok {
			return res[0], itemWithIndex, found
		}
	}
	return
}

// WatchItems subscribes to receive notifications about the changes in the
// mapping related to items with integer handles.
func (idx *nameToIndex) WatchItems(subscriber string, channel chan<- NameToIndexDto) {
	watcher := func(dto idxmap.NamedMappingGenericEvent) {
		itemWithIndex, ok := dto.Value.(WithIndex)
		if !ok {
			return
		}
		msg := NameToIndexDto{
			NamedMappingEvent: dto.NamedMappingEvent,
			Item:              itemWithIndex,
		}
		select {
		case channel <- msg:
		case <-time.After(idxmap.DefaultNotifTimeout):
			idx.log.Warn("Unable to deliver notification")
		}
	}
	idx.Watch(subscriber, watcher)
}

// internalIndexFunction is an index function used internally for nameToIndex.
func internalIndexFunction(item interface{}) map[string][]string {
	indexes := map[string][]string{}
	itemWithIndex, ok := item.(WithIndex)
	if !ok || itemWithIndex == nil {
		return indexes
	}

	indexes[indexKey] = []string{strconv.FormatUint(uint64(itemWithIndex.GetIndex()), 10)}
	return indexes
}
