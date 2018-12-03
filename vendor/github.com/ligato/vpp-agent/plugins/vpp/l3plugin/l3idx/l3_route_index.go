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

package l3idx

import (
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/plugins/vpp/model/l3"
)

// RouteIndex provides read-only access to mapping between routes data and route names
type RouteIndex interface {
	// GetMapping returns internal read-only mapping with metadata of l3.StaticRoutes_Route type.
	GetMapping() idxvpp.NameToIdxRW

	// LookupIdx looks up previously stored item identified by index in mapping.
	LookupIdx(name string) (idx uint32, metadata *l3.StaticRoutes_Route, exists bool)

	// LookupName looks up previously stored item identified by name in mapping.
	LookupName(idx uint32) (name string, metadata *l3.StaticRoutes_Route, exists bool)

	// LookupRouteAndIDByOutgoingIfc returns structure with route name and data which contains specified ifName
	// in metadata as outgoing interface
	LookupRouteAndIDByOutgoingIfc(ifName string) []StaticRoutesRouteAndIdx
}

// RouteIndexRW is mapping between routes data (metadata) and routes entry names.
type RouteIndexRW interface {
	RouteIndex

	// RegisterName adds new item into name-to-index mapping.
	RegisterName(name string, idx uint32, ifMeta *l3.StaticRoutes_Route)

	// UnregisterName removes an item identified by name from mapping
	UnregisterName(name string) (idx uint32, metadata *l3.StaticRoutes_Route, exists bool)

	// Clear removes all Routes from the mapping.
	Clear()
}

// routeIndex is type-safe implementation of mapping between routeId and route data.
type routeIndex struct {
	mapping idxvpp.NameToIdxRW
}

// NewRouteIndex creates new instance of routeIndex.
func NewRouteIndex(mapping idxvpp.NameToIdxRW) RouteIndexRW {
	return &routeIndex{mapping: mapping}
}

// GetMapping returns internal read-only mapping. It is used in tests to inspect the content of the linuxArpIndex.
func (routeIndex *routeIndex) GetMapping() idxvpp.NameToIdxRW {
	return routeIndex.mapping
}

// LookupIdx looks up previously stored item identified by index in mapping.
func (routeIndex *routeIndex) LookupIdx(name string) (idx uint32, metadata *l3.StaticRoutes_Route, exists bool) {
	idx, meta, exists := routeIndex.mapping.LookupIdx(name)
	if exists {
		metadata = routeIndex.castMetadata(meta)
	}
	return idx, metadata, exists
}

// LookupName looks up previously stored item identified by name in mapping.
func (routeIndex *routeIndex) LookupName(idx uint32) (name string, metadata *l3.StaticRoutes_Route, exists bool) {
	name, meta, exists := routeIndex.mapping.LookupName(idx)
	if exists {
		metadata = routeIndex.castMetadata(meta)
	}
	return name, metadata, exists
}

// StaticRoutesRouteAndIdx is used for associating route with route name in func return value
// It is used as container to return more values
type StaticRoutesRouteAndIdx struct {
	Route   *l3.StaticRoutes_Route
	RouteID string
}

// LookupRouteAndIDByOutgoingIfc returns all names related to the provided interface
func (routeIndex *routeIndex) LookupRouteAndIDByOutgoingIfc(outgoingIfName string) []StaticRoutesRouteAndIdx {
	var result []StaticRoutesRouteAndIdx
	for _, routeID := range routeIndex.mapping.ListNames() {
		_, route, found := routeIndex.LookupIdx(routeID)
		if found && route != nil && route.OutgoingInterface == outgoingIfName {
			result = append(result, StaticRoutesRouteAndIdx{route, routeID})
		}
	}
	return result
}

// RegisterName adds new item into name-to-index mapping.
func (routeIndex *routeIndex) RegisterName(name string, idx uint32, ifMeta *l3.StaticRoutes_Route) {
	routeIndex.mapping.RegisterName(name, idx, ifMeta)
}

// UnregisterName removes an item identified by name from mapping
func (routeIndex *routeIndex) UnregisterName(name string) (idx uint32, metadata *l3.StaticRoutes_Route, exists bool) {
	idx, meta, exists := routeIndex.mapping.UnregisterName(name)
	return idx, routeIndex.castMetadata(meta), exists
}

// Clear removes all Routes from the cache.
func (routeIndex *routeIndex) Clear() {
	routeIndex.mapping.Clear()
}

func (routeIndex *routeIndex) castMetadata(meta interface{}) *l3.StaticRoutes_Route {
	if ifMeta, ok := meta.(*l3.StaticRoutes_Route); ok {
		return ifMeta
	}

	return nil
}
