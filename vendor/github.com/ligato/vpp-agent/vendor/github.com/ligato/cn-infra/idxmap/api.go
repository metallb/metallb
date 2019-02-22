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

package idxmap

// NamedMapping is the "user API" to the mapping. It provides read-only access.
type NamedMapping interface {
	// GetRegistryTitle returns the title assigned to the registry.
	GetRegistryTitle() string

	// GetValue retrieves a previously stored item identified by
	// <name>. If there is no item associated with the give name in the mapping,
	// the <exists> flag is returned as *false*.
	GetValue(name string) (value interface{}, exists bool)

	// ListNames looks up the items by a secondary index.
	// It returns the names of all indexes for which the value of a secondary
	// key <field> equals to <value>.
	ListNames(field string, value string) (names []string)

	// ListAllNames returns all names in the mapping.
	ListAllNames() (names []string)

	// ListFields returns a map of fields (secondary indexes) and their values
	// currently associated with the item identified by <name>.
	ListFields(name string) map[string][]string // field -> values

	// Watch subscribes to receive notifications about the changes in the
	// mapping. To receive changes through a channel, ToChan utility can be used.
	//
	// Example usage:
	//
	//  map.Watch(subscriber, ToChan(myChannel))
	//
	//  map.Watch(subscriber, func(msgNamedMappingGenericEvent) {/*handle callback*/ return nil})
	//
	Watch(subscriber string, callback func(NamedMappingGenericEvent)) error
}

// NamedMappingRW is the "owner API" to the mapping. Using this API the owner
// can modify the content of the mapping.
type NamedMappingRW interface {
	NamedMapping

	// Put registers a new item into the mapping under the given <name>.
	// Name is the primary unique key, if an item was registered before
	// it is overwritten.
	Put(name string, value interface{})

	// Update replaces a metadata value in an existing mapping under
	// provided <name>. If item is missing, it is NOT created
	// and false value is returned in such a case.
	Update(name string, value interface{}) (success bool)

	// Delete removes an item associated with the <name> from the mapping.
	Delete(name string) (value interface{}, exists bool)

	// Clear removes all entries from the mapping
	Clear()
}

// NamedMappingEvent is a part of the change notification. It is a generic
// part that does not contain metadata of type interface{} thus it can be reused
// in mapping with typed metadata.
type NamedMappingEvent struct {
	// Logical name of the object
	Name string
	// Del denotes a type of change
	// - it is true if an item was removed
	// - it false if an item was added or updated
	Del bool
	// Update denotes a type of change
	// - it is true if and item metadata was updated
	// - it is false if an item was added or removed
	Update bool
	// RegistryTitle identifies the registry (NameToIndexMapping)
	RegistryTitle string
}

// NamedMappingGenericEvent represents a single change in the mapping.
// The structure is created when an item is inserted or removed from the mapping.
type NamedMappingGenericEvent struct {
	NamedMappingEvent

	Value interface{}
}
