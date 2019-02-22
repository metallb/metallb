//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package aclidx

import (
	"time"

	"github.com/ligato/cn-infra/idxmap"
	"github.com/ligato/cn-infra/logging"

	"github.com/ligato/vpp-agent/pkg/idxvpp2"
)

// ACLMetadataIndex provides read-only access to mapping between ACL indices (used internally in VPP)
// and ACL names.
type ACLMetadataIndex interface {
	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupByName(name string) (metadata *ACLMetadata, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupByIndex(idx uint32) (name string, metadata *ACLMetadata, exists bool)

	// WatchAcls
	WatchAcls(subscriber string, channel chan<- ACLMetadataDto)
}

// ACLMetadataIndexRW is mapping between ACL indices (used internally in VPP) and ACL names.
type ACLMetadataIndexRW interface {
	ACLMetadataIndex
	idxmap.NamedMappingRW
}

// ACLMetadata represents metadata for ACL.
type ACLMetadata struct {
	Index uint32
	L2    bool
}

// GetIndex returns index of the ACL.
func (m *ACLMetadata) GetIndex() uint32 {
	return m.Index
}

// ACLMetadataDto represents an item sent through watch channel in aclIndex.
type ACLMetadataDto struct {
	idxmap.NamedMappingEvent
	Metadata *ACLMetadata
}

type aclMetadataIndex struct {
	idxmap.NamedMappingRW

	log         logging.Logger
	nameToIndex idxvpp2.NameToIndex
}

// NewACLIndex creates new instance of aclMetadataIndex.
func NewACLIndex(logger logging.Logger, title string) ACLMetadataIndexRW {
	mapping := idxvpp2.NewNameToIndex(logger, title, indexMetadata)
	return &aclMetadataIndex{
		NamedMappingRW: mapping,
		log:            logger,
		nameToIndex:    mapping,
	}
}

// LookupByName looks up previously stored item identified by index in mapping.
func (aclIdx *aclMetadataIndex) LookupByName(name string) (metadata *ACLMetadata, exists bool) {
	meta, found := aclIdx.GetValue(name)
	if found {
		if typedMeta, ok := meta.(*ACLMetadata); ok {
			return typedMeta, found
		}
	}
	return nil, false
}

// LookupByIndex looks up previously stored item identified by name in mapping.
func (aclIdx *aclMetadataIndex) LookupByIndex(idx uint32) (name string, metadata *ACLMetadata, exists bool) {
	var item idxvpp2.WithIndex
	name, item, exists = aclIdx.nameToIndex.LookupByIndex(idx)
	if exists {
		var isIfaceMeta bool
		metadata, isIfaceMeta = item.(*ACLMetadata)
		if !isIfaceMeta {
			exists = false
		}
	}
	return
}

// WatchAcls ...
func (aclIdx *aclMetadataIndex) WatchAcls(subscriber string, channel chan<- ACLMetadataDto) {
	watcher := func(dto idxmap.NamedMappingGenericEvent) {
		typedMeta, ok := dto.Value.(*ACLMetadata)
		if !ok {
			return
		}
		msg := ACLMetadataDto{
			NamedMappingEvent: dto.NamedMappingEvent,
			Metadata:          typedMeta,
		}
		select {
		case channel <- msg:
		case <-time.After(idxmap.DefaultNotifTimeout):
			aclIdx.log.Warn("Unable to deliver notification")
		}
	}
	if err := aclIdx.Watch(subscriber, watcher); err != nil {
		aclIdx.log.Error(err)
	}
}

// indexMetadata is an index function used for ACL metadata.
func indexMetadata(metaData interface{}) map[string][]string {
	indexes := make(map[string][]string)

	ifMeta, ok := metaData.(*ACLMetadata)
	if !ok || ifMeta == nil {
		return indexes
	}

	return indexes
}
