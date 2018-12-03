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

package aclidx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	acl_model "github.com/ligato/vpp-agent/plugins/vpp/model/acl"
)

// ACLIndex provides read-only access to mapping between ACL indices (used internally in VPP)
// and ACL names.
type ACLIndex interface {
	// GetMapping returns internal read-only mapping with metadata.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *acl_model.AccessLists_Acl, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *acl_model.AccessLists_Acl, exists bool)

	// WatchNameToIdx allows to subscribe for watching changes in aclIndex mapping.
	WatchNameToIdx(subscriber string, pluginChannel chan IdxDto)
}

// ACLIndexRW is mapping between ACL indices (used internally in VPP) and ACL names.
type ACLIndexRW interface {
	ACLIndex

	// RegisterName adds a new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *acl_model.AccessLists_Acl)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *acl_model.AccessLists_Acl, exists bool)

	// Clear removes all ACL entries from the mapping.
	Clear()
}

// aclIndex is type-safe implementation of mapping between ACL index and name. It holds metadata
// of type *AccessLists_Acl as well.
type aclIndex struct {
	mapping idxvpp.NameToIdxRW
}

// IdxDto represents an item sent through watch channel in aclIndex.
// In contrast to NameToIdxDto, it contains typed metadata.
type IdxDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *acl_model.AccessLists_Acl
}

// NewACLIndex creates new instance of aclIndex.
func NewACLIndex(mapping idxvpp.NameToIdxRW) ACLIndexRW {
	return &aclIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the aclIndex.
func (acl *aclIndex) GetMapping() idxvpp.NameToIdxRW {
	return acl.mapping
}

// RegisterName adds new item into name-to-index mapping.
func (acl *aclIndex) RegisterName(name string, idx uint32, ifMeta *acl_model.AccessLists_Acl) {
	acl.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping.
func (acl *aclIndex) UnregisterName(name string) (idx uint32, metadata *acl_model.AccessLists_Acl, exists bool) {
	idx, meta, exists := acl.mapping.UnregisterName(name)
	return idx, acl.castMetadata(meta), exists
}

// Clear removes all ACL entries from the cache.
func (acl *aclIndex) Clear() {
	acl.mapping.Clear()
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (acl *aclIndex) LookupIdx(name string) (idx uint32, metadata *acl_model.AccessLists_Acl, exists bool) {
	idx, meta, exists := acl.mapping.LookupIdx(name)
	if exists {
		metadata = acl.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (acl *aclIndex) LookupName(idx uint32) (name string, metadata *acl_model.AccessLists_Acl, exists bool) {
	name, meta, exists := acl.mapping.LookupName(idx)
	if exists {
		metadata = acl.castMetadata(meta)
	}
	return name, metadata, exists
}

func (acl *aclIndex) castMetadata(meta interface{}) *acl_model.AccessLists_Acl {
	if ifMeta, ok := meta.(*acl_model.AccessLists_Acl); ok {
		return ifMeta
	}
	return nil
}

// WatchNameToIdx allows to subscribe for watching changes in swIfIndex mapping.
func (acl *aclIndex) WatchNameToIdx(subscriber string, pluginChannel chan IdxDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	acl.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- IdxDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                acl.castMetadata(c.Metadata),
			}
		}
	}()
}
