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

package syncbase

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
)

// NewLatestRev is a constructor.
func NewLatestRev() *PrevRevisions {
	return &PrevRevisions{
		revisions: make(map[string]datasync.LazyValueWithRev),
	}
}

// PrevRevisions maintains the map of keys & values with revision.
type PrevRevisions struct {
	sync.RWMutex
	revisions map[string]datasync.LazyValueWithRev
}

// valWithRev stores the tuple (see the map above).
type valWithRev struct {
	val datasync.LazyValue
	rev int64
}

// GetValue gets the current value in the data change event.
// The caller must provide an address of a proto message buffer
// for each value.
// returns:
// - error if value argument can not be properly filled.
func (d *valWithRev) GetValue(value proto.Message) error {
	return d.val.GetValue(value)
}

// GetRevision gets the revision associated with the value in the data change event.
// The caller must provide an address of a proto message buffer
// for each value.
// returns:
// - revision associated with the latest change in the key-value pair.
func (d *valWithRev) GetRevision() (rev int64) {
	return d.rev
}

// Put updates the entry in the revisions and returns previous value.
func (r *PrevRevisions) Put(key string, val datasync.LazyValue) (
	found bool, prev datasync.LazyValueWithRev, currRev int64) {

	found, prev = r.Get(key)
	if prev != nil {
		currRev = prev.GetRevision() + 1
	} else {
		currRev = 0
	}

	r.Lock()
	r.revisions[key] = &valWithRev{val, currRev}
	r.Unlock()

	return found, prev, currRev
}

// PutWithRevision updates the entry in the revisions and returns previous value.
func (r *PrevRevisions) PutWithRevision(key string, inCurrent datasync.LazyValueWithRev) (
	found bool, prev datasync.LazyValueWithRev) {

	found, prev = r.Get(key)

	currentRev := inCurrent.GetRevision()
	if currentRev == 0 && prev != nil {
		currentRev = prev.GetRevision() + 1
	}

	r.Lock()
	r.revisions[key] = &valWithRev{inCurrent, currentRev}
	r.Unlock()

	return found, prev
}

// Del deletes the entry from revisions and returns previous value.
func (r *PrevRevisions) Del(key string) (found bool, prev datasync.LazyValueWithRev) {
	found, prev = r.Get(key)

	if found {
		r.Lock()
		delete(r.revisions, key)
		r.Unlock()
	}

	return found, prev
}

// Get gets the last proto.Message with it's revision.
func (r *PrevRevisions) Get(key string) (found bool, value datasync.LazyValueWithRev) {
	r.RLock()
	prev, found := r.revisions[key]
	r.RUnlock()

	if found {
		return found, prev
	}

	return found, nil
}

// ListKeys returns all stored keys.
func (r *PrevRevisions) ListKeys() []string {
	var ret []string

	r.RLock()
	for key := range r.revisions {
		ret = append(ret, key)
	}
	r.RUnlock()

	return ret
}

// Cleanup removes all data from the registry
func (r *PrevRevisions) Cleanup() {
	r.Lock()
	defer r.Unlock()

	r.revisions = map[string]datasync.LazyValueWithRev{}
}
