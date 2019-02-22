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

package utils

import (
	"sort"
	"strings"
	"encoding/json"
)

// KeySet defines API for a set of keys.
type KeySet interface {
	// String return human-readable string representation of the key-set.
	String() string

	// Iterate exposes the set of keys as slice which can be iterated through.
	// The returned slice should not be modified.
	Iterate() []string

	// Length returns the number of keys in the set.
	Length() int

	// Has returns true if the given key is in the set.
	Has(key string) bool

	// Add adds key into the set.
	Add(key string) (changed bool)

	// Del removes key from the set.
	Del(key string) (changed bool)

	// Subtract removes keys from this set which are also in <ks2>.
	Subtract(ks2 KeySet) (changed bool)

	// Intersect removes keys from this set which are not in <ks2>.
	Intersect(ks2 KeySet) (changed bool)

	// CopyOnWrite returns first a shallow copy of the key set, which gets
	// deep-copied when it is about to get modified.
	CopyOnWrite() KeySet
}

/****************************** Singleton KeySet  ******************************/

// singletonKeySet is the KeySet implementation for set which is guaranteed to
// contain at most one key.
type singletonKeySet struct {
	set [1]string // empty string = empty set
}

// NewSingletonKeySet returns KeySet implementation for at most one key.
func NewSingletonKeySet(key string) KeySet {
	s := &singletonKeySet{}
	s.set[0] = key
	return s
}

// String return human-readable string representation of the key-set.
func (s *singletonKeySet) String() string {
	if s == nil {
		return "{}"
	}
	return "{" + s.set[0] + "}"
}

// Iterate exposes the set of keys as slice which can be iterated through.
// The returned slice should not be modified.
func (s *singletonKeySet) Iterate() []string {
	if s == nil {
		return nil
	}
	return s.set[:s.Length()]
}

// Length returns the number of keys in the set.
func (s *singletonKeySet) Length() int {
	if s == nil {
		return 0
	}
	if s.set[0] == "" {
		return 0
	}
	return 1
}

// Has returns true if the given key is in the set.
func (s *singletonKeySet) Has(key string) bool {
	if s == nil {
		return false
	}
	if s.set[0] == key {
		return true
	}
	return false
}

// Add adds key into the set.
func (s *singletonKeySet) Add(key string) (changed bool) {
	if s.set[0] == key {
		return false
	}
	s.set[0] = key
	return true
}

// Del removes key from the set.
func (s *singletonKeySet) Del(key string) (changed bool) {
	if s.set[0] == key {
		s.set[0] = ""
		return true
	}
	return false
}

// Subtract removes keys from this set which are also in <ks2>.
func (s *singletonKeySet) Subtract(ks2 KeySet) (changed bool) {
	if s.set[0] == "" {
		return false
	}
	if ks2.Has(s.set[0]) {
		s.set[0] = ""
		return true
	}
	return false
}

// Intersect removes keys from this set which are not in <ks2>.
func (s *singletonKeySet) Intersect(ks2 KeySet) (changed bool) {
	if s.set[0] == "" {
		return false
	}
	if !ks2.Has(s.set[0]) {
		s.set[0] = ""
		return true
	}
	return false
}

// CopyOnWrite actually returns a deep copy, but that is super cheap for singleton.
func (s *singletonKeySet) CopyOnWrite() KeySet {
	return &singletonKeySet{set: s.set}
}

// MarshalJSON marshalls the set into JSON.
func (s *singletonKeySet) MarshalJSON() ([]byte, error) {
	if s.set[0] == "" {
		return []byte("[]"), nil
	}
	return []byte("[\"" + s.set[0] + "\"]"), nil
}

/***************************** KeySet based on map *****************************/

// mapKeySet implements KeySet using a map.
// Quicker lookups in average than the slice-based implementation, but bigger
// memory footprint and much slower copying.
type mapKeySet struct {
	shallowCopy bool
	set         mapWithKeys
	iter        []string
	iterInSync  bool
}

// mapWithKeys is used to represent a set of keys using a map with empty values.
type mapWithKeys map[string]struct{}

// NewMapBasedKeySet returns KeySet implemented using map.
func NewMapBasedKeySet(keys ...string) KeySet {
	s := &mapKeySet{set: make(mapWithKeys), iter: []string{}, iterInSync: true}
	for _, key := range keys {
		s.Add(key)
	}
	return s
}

// String return human-readable string representation of the key-set.
func (s *mapKeySet) String() string {
	return s.string(false)
}

// string return human-readable string representation of the key-set.
func (s *mapKeySet) string(json bool) string {
	if s == nil {
		if json {
			return "[]"
		}
		return "{}"
	}
	str := "{"
	if json {
		str = "["
	}
	idx := 0
	for key := range s.set {
		if json {
			str += "\"" + key + "\""
		} else {
			str += key
		}
		if idx < len(s.set)-1 {
			str += ", "
		}
		idx++
	}
	if json {
		str += "]"
	} else {
		str += "}"
	}
	return str
}

// Iterate exposes the set of keys as slice which can be iterated through.
// The returned slice should not be modified.
func (s *mapKeySet) Iterate() (keys []string) {
	if s == nil {
		return keys
	}
	if s.iterInSync {
		return s.iter
	}
	s.iter = make([]string, len(s.set))
	i := 0
	for key := range s.set {
		s.iter[i] = key
		i++
	}
	s.iterInSync = true
	return s.iter
}

// Length returns the number of keys in the set.
func (s *mapKeySet) Length() int {
	if s == nil {
		return 0
	}
	return len(s.set)
}

// Has returns true if the given key is in the set.
func (s *mapKeySet) Has(key string) bool {
	if s == nil {
		return false
	}
	_, has := s.set[key]
	return has
}

// Add adds key into the set.
func (s *mapKeySet) Add(key string) (changed bool) {
	if !s.Has(key) {
		if s.shallowCopy {
			s.set = s.deepCopyMap()
			s.shallowCopy = false
		}
		s.set[key] = struct{}{}
		if s.iterInSync {
			s.iter = append(s.iter, key)
		}
		changed = true
	}
	return
}

// Del removes key from the set.
func (s *mapKeySet) Del(key string) (changed bool) {
	if s.Has(key) {
		if s.shallowCopy {
			s.set = s.deepCopyMap()
			s.shallowCopy = false
		}
		delete(s.set, key)
		s.iterInSync = false
		changed = true
	}
	return
}

// Subtract removes keys from this set which are also in <ks2>.
func (s *mapKeySet) Subtract(ks2 KeySet) (changed bool) {
	for _, key := range ks2.Iterate() {
		if s.Del(key) {
			changed = true
		}
	}
	return
}

// Intersect removes keys from this set which are not in <ks2>.
func (s *mapKeySet) Intersect(ks2 KeySet) (changed bool) {
	for key := range s.set {
		if !ks2.Has(key) {
			s.Del(key)
			changed = true
		}
	}
	return
}

// CopyOnWrite returns first a shallow copy of this key set, which gets deep-copied
// when it is about to get modified.
func (s *mapKeySet) CopyOnWrite() KeySet {
	return &mapKeySet{
		shallowCopy: true,
		set:         s.set,
	}
}

// deepCopyMap returns a deep-copy of the internal map representing the key set.
func (s *mapKeySet) deepCopyMap() mapWithKeys {
	copy := make(mapWithKeys)
	for key := range s.set {
		copy[key] = struct{}{}
	}
	return copy
}

// MarshalJSON marshalls the set into JSON.
func (s *mapKeySet) MarshalJSON() ([]byte, error) {
	return []byte(s.string(true)), nil
}

/**************************** KeySet based on slice ****************************/

// sliceKeySet implements KeySet using a slice with ordered keys.
// The main advantage over the map-based implementation, is much smaller
// memory footprint and quick (deep-)copying.
type sliceKeySet struct {
	shallowCopy bool
	set         []string
	length      int // len(set) can be > than length - the rest are empty strings
}

// NewSliceBasedKeySet returns KeySet implemented using a slice with ordered keys.
func NewSliceBasedKeySet(keys ...string) KeySet {
	s := &sliceKeySet{set: []string{}}
	for _, key := range keys {
		s.Add(key)
	}
	return s
}

// String return human-readable string representation of the key-set.
func (s *sliceKeySet) String() string {
	if s == nil {
		return "{}"
	}
	return "{" + strings.Join(s.set[:s.length], ", ") + "}"
}

// Iterate exposes the set of keys as slice which can be iterated through.
// The returned slice should not be modified.
func (s *sliceKeySet) Iterate() (keys []string) {
	if s == nil {
		return keys
	}
	return s.set[:s.length]
}

// Length returns the number of keys in the set.
func (s *sliceKeySet) Length() int {
	if s == nil {
		return 0
	}
	return s.length
}

// Has returns true if the given key is in the set.
func (s *sliceKeySet) Has(key string) bool {
	if s == nil {
		return false
	}
	_, exists := s.getKeyIndex(key)
	return exists
}

// Add adds key into the set.
func (s *sliceKeySet) Add(key string) (changed bool) {
	idx, exists := s.getKeyIndex(key)
	if !exists {
		if s.shallowCopy {
			s.set = s.deepCopySlice()
			s.shallowCopy = false
		}
		if s.length == len(s.set) {
			// increase capacity
			s.set = append(s.set, "")
		}
		if idx < s.length {
			copy(s.set[idx+1:], s.set[idx:])
		}
		s.set[idx] = key
		s.length++
		changed = true
	}
	return
}

// Del removes key from the set.
func (s *sliceKeySet) Del(key string) (changed bool) {
	idx, exists := s.getKeyIndex(key)
	if exists {
		if s.shallowCopy {
			s.set = s.deepCopySlice()
			s.shallowCopy = false
		}
		if idx < s.length-1 {
			copy(s.set[idx:], s.set[idx+1:])
		}
		s.length--
		s.set[s.length] = ""
		changed = true
	}
	return
}

// Subtract removes keys from this set which are also in <ks2>.
func (s *sliceKeySet) Subtract(ks2 KeySet) (changed bool) {
	s2, isSliceKeySet := ks2.(*sliceKeySet)
	if isSliceKeySet {
		// optimized case when both are slice-based
		var i, j, newLen int
		for ; i < s.length; i++ {
			subtract := false
			for ; j < s2.length; j++ {
				if s.set[i] > s2.set[j] {
					continue
				}
				if s.set[i] == s2.set[j] {
					subtract = true
				} else {
					break
				}
			}
			if subtract {
				if s.shallowCopy {
					s.set = s.deepCopySlice()
					s.shallowCopy = false
				}
				changed = true
			}
			if !subtract {
				if newLen != i {
					s.set[newLen] = s.set[i]
				}
				newLen++
			}
		}
		if newLen != s.length {
			s.length = newLen
		}
		return
	}
	for _, key := range ks2.Iterate() {
		if s.Del(key) {
			changed = true
		}
	}
	return
}

// Intersect removes keys from this set which are not in <ks2>.
func (s *sliceKeySet) Intersect(ks2 KeySet) (changed bool) {
	for i := 0; i < s.length; {
		key := s.set[i]
		if !ks2.Has(key) {
			s.Del(key)
			changed = true
		} else {
			i++
		}
	}
	return
}

// CopyOnWrite returns first a shallow copy of this key set, which gets deep-copied
// when it is about to get modified.
func (s *sliceKeySet) CopyOnWrite() KeySet {
	return &sliceKeySet{
		shallowCopy: true,
		set:         s.set,
		length:      s.length,
	}
}

// getKeyIndex returns index at which the given key would be stored.
func (s *sliceKeySet) getKeyIndex(key string) (idx int, exists bool) {
	if s.length <= 5 {
		for idx = 0; idx < s.length; idx++ {
			if key <= s.set[idx] {
				break
			}
		}
	} else {
		idx = sort.Search(s.length,
			func(i int) bool {
				return key <= s.set[i]
			})
	}
	return idx, idx < s.length && key == s.set[idx]
}

// deepCopyMap returns a deep-copy of the internal slice representing the key set.
func (s *sliceKeySet) deepCopySlice() []string {
	c := make([]string, s.length)
	copy(c, s.set)
	return c
}

// MarshalJSON marshalls the set into JSON.
func (s *sliceKeySet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.set[:s.length])
}
