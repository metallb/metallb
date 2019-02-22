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

package test

import (
	"strconv"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"
)

// NameToInteger is a idxmap specialization used in the UTs for scheduler.
// It extends plain metadata with integer exposed as a secondary index.
type NameToInteger interface {
	// LookupByName retrieves a previously stored metadata identified by <name>.
	LookupByName(valName string) (metadata MetaWithInteger, exists bool)

	// LookupByIndex retrieves a previously stored metadata identified by the given
	// integer index <metaInt>.
	LookupByIndex(metaInt int) (valName string, metadata MetaWithInteger, exists bool)
}

// NameToIntegerRW extends NameToInteger with write access.
type NameToIntegerRW interface {
	NameToInteger
	idxmap.NamedMappingRW
}

// MetaWithInteger is interface that metadata for NameToIntMap must implement.
type MetaWithInteger interface {
	// GetInteger returns the integer stored in the metadata.
	GetInteger() int
}

// OnlyInteger is a minimal implementation of MetaWithInteger.
type OnlyInteger struct {
	Integer int
}

// GetInteger returns the integer stored in the metadata.
func (idx *OnlyInteger) GetInteger() int {
	return idx.Integer
}

// nameToInteger implements NameToInteger.
type nameToInteger struct {
	idxmap.NamedMappingRW
	log logging.Logger
}

const (
	// IntegerKey is a secondary index for the integer value.
	IntegerKey = "integer"
)

// NewNameToInteger creates a new instance implementing NameToIntegerRW.
func NewNameToInteger(title string) NameToIntegerRW {
	return &nameToInteger{
		NamedMappingRW: mem.NewNamedMapping(logging.DefaultLogger, title, internalIndexFunction),
	}
}

// LookupByName retrieves a previously stored metadata identified by <name>.
func (metaMap *nameToInteger) LookupByName(valName string) (metadata MetaWithInteger, exists bool) {
	untypedMeta, found := metaMap.GetValue(valName)
	if found {
		if metadata, ok := untypedMeta.(MetaWithInteger); ok {
			return metadata, found
		}
	}
	return nil, false
}

// LookupByIndex retrieves a previously stored metadata identified by the given
// integer index <metaInt>.
func (metaMap *nameToInteger) LookupByIndex(metaInt int) (valName string, metadata MetaWithInteger, exists bool) {
	res := metaMap.ListNames(IntegerKey, strconv.FormatUint(uint64(metaInt), 10))
	if len(res) != 1 {
		return
	}
	untypedMeta, found := metaMap.GetValue(res[0])
	if found {
		if metadata, ok := untypedMeta.(MetaWithInteger); ok {
			return res[0], metadata, found
		}
	}
	return
}

func internalIndexFunction(untypedMeta interface{}) map[string][]string {
	indexes := map[string][]string{}
	metadata, ok := untypedMeta.(MetaWithInteger)
	if !ok || metadata == nil {
		return indexes
	}

	indexes[IntegerKey] = []string{strconv.FormatUint(uint64(metadata.GetInteger()), 10)}
	return indexes
}
