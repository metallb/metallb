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
	"github.com/ligato/cn-infra/idxmap/mem"
	"github.com/ligato/cn-infra/logging"

	"github.com/ligato/vpp-agent/api/models/linux/namespace"
)

// LinuxIfMetadataIndex provides read-only access to mapping with Linux interface
// metadata. It extends from NameToIndex.
type LinuxIfMetadataIndex interface {
	// LookupByName retrieves a previously stored metadata of interface
	// identified by logical <name>. If there is no interface associated with
	// the given name in the mapping, the <exists> is returned as *false* and
	// <metadata> as *nil*.
	LookupByName(name string) (metadata *LinuxIfMetadata, exists bool)

	// LookupByVPPTap retrieves a previously configured TAP_TO_VPP interface
	// by the logical name of the associated VPP-side of the TAP.
	// If there is no such interface, <exists> is returned as *false* with <name>
	// and <metadata> both set to empty values.
	LookupByVPPTap(vppTapName string) (name string, metadata *LinuxIfMetadata, exists bool)

	// ListAllInterfaces returns slice of names of all interfaces in the mapping.
	ListAllInterfaces() (names []string)

	// WatchInterfaces allows to subscribe to watch for changes in the mapping
	// of interface metadata.
	WatchInterfaces(subscriber string, channel chan<- LinuxIfMetadataIndexDto)
}

// LinuxIfMetadataIndexRW provides read-write access to mapping with interface
// metadata.
type LinuxIfMetadataIndexRW interface {
	LinuxIfMetadataIndex
	idxmap.NamedMappingRW
}

// LinuxIfMetadata collects metadata for Linux interface used in secondary lookups.
type LinuxIfMetadata struct {
	LinuxIfIndex int
	VPPTapName   string // empty for VETHs
	Namespace    *linux_namespace.NetNamespace
}

// LinuxIfMetadataIndexDto represents an item sent through watch channel in LinuxIfMetadataIndex.
// In contrast to NamedMappingGenericEvent, it contains typed interface metadata.
type LinuxIfMetadataIndexDto struct {
	idxmap.NamedMappingEvent
	Metadata *LinuxIfMetadata
}

// linuxIfMetadataIndex is type-safe implementation of mapping between interface
// name and metadata of type *LinuxIfMetadata.
type linuxIfMetadataIndex struct {
	idxmap.NamedMappingRW /* embeds */
	log                   logging.Logger
}

const (
	// tapVPPNameIndexKey is used as a secondary key used to search TAP_TO_VPP
	// interface by the logical name of the VPP-side of the TAP.
	tapVPPNameIndexKey = "tap-vpp-name"
)

// NewLinuxIfIndex creates a new instance implementing LinuxIfMetadataIndexRW.
func NewLinuxIfIndex(logger logging.Logger, title string) LinuxIfMetadataIndexRW {
	return &linuxIfMetadataIndex{
		NamedMappingRW: mem.NewNamedMapping(logger, title, indexMetadata),
	}
}

// LookupByName retrieves a previously stored metadata of interface
// identified by logical <name>. If there is no interface associated with
// the give/ name in the mapping, the <exists> is returned as *false* and
// <metadata> as *nil*.
func (ifmx *linuxIfMetadataIndex) LookupByName(name string) (metadata *LinuxIfMetadata, exists bool) {
	meta, found := ifmx.GetValue(name)
	if found {
		if typedMeta, ok := meta.(*LinuxIfMetadata); ok {
			return typedMeta, found
		}
	}
	return nil, false
}

// LookupByVPPTap retrieves a previously configured TAP_TO_VPP interface
// by the logical name of the associated VPP-side of the TAP.
// If there is no such interface, <exists> is returned as *false* with <name>
// and <metadata> both set to empty values.
func (ifmx *linuxIfMetadataIndex) LookupByVPPTap(vppTapName string) (name string, metadata *LinuxIfMetadata, exists bool) {
	res := ifmx.ListNames(tapVPPNameIndexKey, vppTapName)
	if len(res) != 1 {
		return
	}
	untypedMeta, found := ifmx.GetValue(res[0])
	if found {
		if ifMeta, ok := untypedMeta.(*LinuxIfMetadata); ok {
			return res[0], ifMeta, found
		}
	}
	return
}

// ListAllInterfaces returns slice of names of all interfaces in the mapping.
func (ifmx *linuxIfMetadataIndex) ListAllInterfaces() (names []string) {
	return ifmx.ListAllNames()
}

// WatchInterfaces allows to subscribe to watch for changes in the mapping
// if interface metadata.
func (ifmx *linuxIfMetadataIndex) WatchInterfaces(subscriber string, channel chan<- LinuxIfMetadataIndexDto) {
	watcher := func(dto idxmap.NamedMappingGenericEvent) {
		typedMeta, ok := dto.Value.(*LinuxIfMetadata)
		if !ok {
			return
		}
		msg := LinuxIfMetadataIndexDto{
			NamedMappingEvent: dto.NamedMappingEvent,
			Metadata:          typedMeta,
		}
		select {
		case channel <- msg:
		case <-time.After(idxmap.DefaultNotifTimeout):
			ifmx.log.Warn("Unable to deliver notification")
		}
	}
	ifmx.Watch(subscriber, watcher)
}

// indexMetadata is an index function used for interface metadata.
func indexMetadata(metaData interface{}) map[string][]string {
	indexes := make(map[string][]string)

	ifMeta, ok := metaData.(*LinuxIfMetadata)
	if !ok || ifMeta == nil {
		return indexes
	}

	if ifMeta.VPPTapName != "" {
		indexes[tapVPPNameIndexKey] = []string{ifMeta.VPPTapName}
	}
	return indexes
}
