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

package nametoidx

import (
	"strconv"
	"time"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
)

const idxKey = "idxKey"

type nameToIdxMeta struct {
	// added index
	idx uint32
	// original user's meta data
	meta interface{}
}

type nameToIdxMem struct {
	logging.Logger
	internal idxmap.NamedMappingRW
}

// NewNameToIdx creates a new instance implementing NameToIdxRW.
// Argument indexFunction may be nil if you do not want to use secondary indexes.
func NewNameToIdx(logger logging.Logger, title string,
	indexFunction func(interface{}) map[string][]string) idxvpp.NameToIdxRW {
	m := nameToIdxMem{}
	m.Logger = logger
	m.internal = mem.NewNamedMapping(logger, title, func(meta interface{}) map[string][]string {
		var idxs map[string][]string

		internalMeta, ok := meta.(*nameToIdxMeta)
		if !ok {
			return nil
		}
		if indexFunction != nil {
			idxs = indexFunction(internalMeta.meta)
		}
		if idxs == nil {
			idxs = map[string][]string{}
		}
		internal := indexInternalMetadata(meta)
		for k, v := range internal {
			idxs[k] = v
		}
		return idxs
	})
	return &m
}

// RegisterName inserts or updates index and metadata for the given name.
func (mem *nameToIdxMem) RegisterName(name string, idx uint32, metadata interface{}) {
	mem.internal.Put(name, &nameToIdxMeta{idx, metadata})
}

// UnregisterName removes data associated with the given name.
func (mem *nameToIdxMem) UnregisterName(name string) (idx uint32, metadata interface{}, found bool) {
	meta, found := mem.internal.Delete(name)
	if found {
		if internalMeta, ok := meta.(*nameToIdxMeta); ok {
			return internalMeta.idx, internalMeta.meta, found
		}
	}
	return
}

// Update metadata in mapping entry associated with the provided name.
func (mem *nameToIdxMem) UpdateMetadata(name string, metadata interface{}) (success bool) {
	meta, found := mem.internal.GetValue(name)
	if found {
		if internalMeta, ok := meta.(*nameToIdxMeta); ok {
			return mem.internal.Update(name, &nameToIdxMeta{internalMeta.idx, metadata})
		}
	}
	return false
}

// Clear removes all entries from the mapping
func (mem *nameToIdxMem) Clear() {
	mem.internal.Clear()
}

// GetRegistryTitle returns a name assigned to mapping.
func (mem *nameToIdxMem) GetRegistryTitle() string {
	return mem.internal.GetRegistryTitle()
}

// LookupIdx allows to retrieve previously stored index for particular name.
func (mem *nameToIdxMem) LookupIdx(name string) (uint32, interface{}, bool) {
	meta, found := mem.internal.GetValue(name)
	if found {
		if internalMeta, ok := meta.(*nameToIdxMeta); ok {
			return internalMeta.idx, internalMeta.meta, found
		}
	}
	return 0, nil, false
}

// LookupName looks up the name associated with the given softwareIfIndex.
func (mem *nameToIdxMem) LookupName(idx uint32) (name string, metadata interface{}, exists bool) {
	res := mem.internal.ListNames(idxKey, strconv.FormatUint(uint64(idx), 10))
	if len(res) != 1 {
		return
	}
	m, found := mem.internal.GetValue(res[0])
	if found {
		if internalMeta, ok := m.(*nameToIdxMeta); ok {
			return res[0], internalMeta.meta, found
		}
	}
	return
}

func (mem *nameToIdxMem) LookupNameByMetadata(key string, value string) []string {
	return mem.internal.ListNames(key, value)
}

// ListNames returns all names in the mapping.
func (mem *nameToIdxMem) ListNames() (names []string) {
	return mem.internal.ListAllNames()
}

// Watch starts monitoring a change in the mapping. When yhe change occurs, the callback is called.
// ToChan utility can be used to receive changes through channel.
func (mem *nameToIdxMem) Watch(subscriber string, callback func(idxvpp.NameToIdxDto)) {
	watcher := func(dto idxmap.NamedMappingGenericEvent) {
		internalMeta, ok := dto.Value.(*nameToIdxMeta)
		if !ok {
			return
		}
		msg := idxvpp.NameToIdxDto{
			NameToIdxDtoWithoutMeta: idxvpp.NameToIdxDtoWithoutMeta{
				NamedMappingEvent: dto.NamedMappingEvent,
				Idx:               internalMeta.idx},
			Metadata: internalMeta.meta,
		}
		callback(msg)
	}
	mem.internal.Watch(infra.PluginName(subscriber), watcher)
}

// ToChan is an utility that allows to receive notification through a channel.
// If a notification can not be delivered until timeout, it is dropped.
func ToChan(ch chan idxvpp.NameToIdxDto) func(dto idxvpp.NameToIdxDto) {
	return func(dto idxvpp.NameToIdxDto) {
		select {
		case ch <- dto:
		case <-time.After(idxmap.DefaultNotifTimeout):
			logrus.DefaultLogger().Warn("Unable to deliver notification")
		}
	}
}

func indexInternalMetadata(metaData interface{}) map[string][]string {
	indexes := map[string][]string{}
	internalMeta, ok := metaData.(*nameToIdxMeta)
	if !ok || internalMeta == nil {
		return indexes
	}

	idx := internalMeta.idx
	indexes[idxKey] = []string{strconv.FormatUint(uint64(idx), 10)}

	return indexes
}
