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

package ifaceidx

import (
	log "github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
)

// DefNs is name for default namespace
const DefNs = "default-ns"

const (
	ipAddressIndexKey = "ipAddrKey"
	hostIfNameKey     = "hostIfName"
)

// IndexedLinuxInterface is linux interface configuration with its system index. Used as a metadata object.
type IndexedLinuxInterface struct {
	Index uint32
	Data  *interfaces.LinuxInterfaces_Interface
}

// LinuxIfIndex provides read-only access to mapping between software interface indices and interface names.
type LinuxIfIndex interface {
	// GetMapping returns internal read-only mapping with metadata of type interface{}.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *IndexedLinuxInterface, exists bool)

	// LookupIdxByNamespace looks up previously stored item identified by system index in mapping using namespace.
	LookupIdxByNamespace(name string, ns string) (osIdx uint32, metadata *IndexedLinuxInterface, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *IndexedLinuxInterface, exists bool)

	// LookupNameByNamespace looks up previously stored item identified by name in mapping using namespace.
	LookupNameByNamespace(osIdx uint32, ns string) (name string, metadata *IndexedLinuxInterface, exists bool)

	// LookupNameByHostIfName looks up the interface identified by the name used in HostOs.
	LookupNameByHostIfName(hostIfName string) []string

	// WatchNameToIdx allows to subscribe for watching changes in linuxIfIndex mapping.
	WatchNameToIdx(subscriber string, pluginChannel chan LinuxIfIndexDto)
}

// LinuxIfIndexRW is mapping between software interface indices (used internally in VPP)
// and interface names.
type LinuxIfIndexRW interface {
	LinuxIfIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *IndexedLinuxInterface)

	// UnregisterName removes an item identified by name from mapping.
	UnregisterName(name string) (idx uint32, metadata *IndexedLinuxInterface, exists bool)
}

// LinuxIfIndexDto represents an item sent through watch channel in linuxIfIndex.
// In contrast to NameToIdxDto it contains typed metadata.
type LinuxIfIndexDto struct {
	idxvpp.NameToIdxDtoWithoutMeta
	Metadata *IndexedLinuxInterface
}

// linuxIfIndex is type-safe implementation of mapping between Software interface index
// and interface name. It holds metadata of type *InterfaceMeta as well.
type linuxIfIndex struct {
	mapping idxvpp.NameToIdxRW
}

// NewLinuxIfIndex creates new instance of linuxIfIndex.
func NewLinuxIfIndex(mapping idxvpp.NameToIdxRW) LinuxIfIndexRW {
	return &linuxIfIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the linuxIfIndex.
func (linuxIfIdx *linuxIfIndex) GetMapping() idxvpp.NameToIdxRW {
	return linuxIfIdx.mapping
}

// LookupIdx looks up previously stored item identified by index in mapping. Returned index is internal (IfIdxSeq)
func (linuxIfIdx *linuxIfIndex) LookupIdx(name string) (idx uint32, metadata *IndexedLinuxInterface, exists bool) {
	idx, meta, exists := linuxIfIdx.mapping.LookupIdx(name)
	if exists {
		metadata = linuxIfIdx.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupIdxByNamespace looks up previously stored item identified by system index in mapping using namespace.
// Returned index is an index representing interface in the host os.
func (linuxIfIdx *linuxIfIndex) LookupIdxByNamespace(name string, namespace string) (osIdx uint32, metadata *IndexedLinuxInterface, exists bool) {
	for _, ifName := range linuxIfIdx.mapping.ListNames() {
		_, meta, exists := linuxIfIdx.mapping.LookupIdx(ifName)
		if !exists || meta == nil {
			continue
		}
		metadata = linuxIfIdx.castMetadata(meta)
		if metadata == nil || metadata.Data == nil {
			continue
		}
		// Compare name and namespace
		if metadata.Data.Name == name && compareNs(metadata.Data.Namespace, namespace) {
			return metadata.Index, metadata, exists
		}
	}
	return osIdx, metadata, exists
}

// LookupName looks up previously stored item using internal index identified by name in mapping.
func (linuxIfIdx *linuxIfIndex) LookupName(idx uint32) (name string, metadata *IndexedLinuxInterface, exists bool) {
	name, meta, exists := linuxIfIdx.mapping.LookupName(idx)
	if exists {
		metadata = linuxIfIdx.castMetadata(meta)
	}
	return name, metadata, exists
}

// LookupIdxByNamespace looks up previously stored item identified by system index in mapping using namespace.
// Returned index is an index representing interface in the host os.
func (linuxIfIdx *linuxIfIndex) LookupNameByNamespace(osIdx uint32, namespace string) (name string, metadata *IndexedLinuxInterface, exists bool) {
	for _, ifName := range linuxIfIdx.mapping.ListNames() {
		_, meta, exists := linuxIfIdx.mapping.LookupIdx(ifName)
		if !exists || meta == nil {
			continue
		}
		metadata = linuxIfIdx.castMetadata(meta)
		if metadata == nil || metadata.Data == nil {
			continue
		}
		// Compare index and namespace
		if metadata.Index == osIdx && compareNs(metadata.Data.Namespace, namespace) {
			return metadata.Data.Name, metadata, exists
		}
	}
	return name, metadata, exists
}

// LookupNameByIP returns names of items that contains given IP address in metadata.
func (linuxIfIdx *linuxIfIndex) LookupNameByHostIfName(hostIfName string) []string {
	return linuxIfIdx.mapping.LookupNameByMetadata(hostIfNameKey, hostIfName)
}

// RegisterName adds new item into name-to-index mapping.
func (linuxIfIdx *linuxIfIndex) RegisterName(name string, idx uint32, ifMeta *IndexedLinuxInterface) {
	linuxIfIdx.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping.
func (linuxIfIdx *linuxIfIndex) UnregisterName(name string) (idx uint32, metadata *IndexedLinuxInterface, exists bool) {
	idx, meta, exists := linuxIfIdx.mapping.UnregisterName(name)
	return idx, linuxIfIdx.castMetadata(meta), exists
}

// WatchNameToIdx allows to subscribe for watching changes in linuxIfIndex mapping.
func (linuxIfIdx *linuxIfIndex) WatchNameToIdx(subscriber string, pluginChannel chan LinuxIfIndexDto) {
	ch := make(chan idxvpp.NameToIdxDto)
	linuxIfIdx.mapping.Watch(subscriber, nametoidx.ToChan(ch))
	go func() {
		for c := range ch {
			pluginChannel <- LinuxIfIndexDto{
				NameToIdxDtoWithoutMeta: c.NameToIdxDtoWithoutMeta,
				Metadata:                linuxIfIdx.castMetadata(c.Metadata),
			}

		}
	}()
}

// IndexMetadata creates indices for metadata. Index for IPAddress will be created.
func IndexMetadata(metaData interface{}) map[string][]string {
	log.DefaultLogger().Debug("IndexMetadata ", metaData)

	indexes := map[string][]string{}
	ifMeta, ok := metaData.(*IndexedLinuxInterface)
	if !ok || ifMeta == nil {
		return indexes
	}

	ip := ifMeta.Data.IpAddresses
	if ip != nil {
		indexes[ipAddressIndexKey] = ip
	}

	if ifMeta.Data.HostIfName != "" {
		indexes[hostIfNameKey] = []string{ifMeta.Data.HostIfName}
	} else {
		indexes[hostIfNameKey] = []string{ifMeta.Data.Name}
	}
	return indexes
}

func (linuxIfIdx *linuxIfIndex) castMetadata(meta interface{}) *IndexedLinuxInterface {
	if ifMeta, ok := meta.(*IndexedLinuxInterface); ok {
		return ifMeta
	}

	return nil
}

func compareNs(ns1 *interfaces.LinuxInterfaces_Interface_Namespace, ns2 string) bool {
	if ns2 == "" {
		ns2 = DefNs
	}
	if ns1 == nil && ns2 != DefNs || ns1 != nil && ns2 == DefNs {
		return false
	}
	if ns1 != nil && ns2 != DefNs && ns1.Name != ns2 {
		return false
	}
	return true
}
