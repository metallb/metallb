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
	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
)

// KVIterator is a simple in memory implementation of data.Iterator.
type KVIterator struct {
	data  []datasync.KeyVal
	index int
}

// NewKVIterator creates a new instance of KVIterator.
func NewKVIterator(data []datasync.KeyVal) *KVIterator {
	return &KVIterator{data: data}
}

// GetNext TODO
func (it *KVIterator) GetNext() (kv datasync.KeyVal, allReceived bool) {
	if it.index >= len(it.data) {
		return nil, true
	}

	ret := it.data[it.index]
	it.index++

	return ret, false
}

// KeyVal represents a single key-value pair.
type KeyVal struct {
	key string
	datasync.LazyValue
	rev int64
}

// NewKeyVal creates a new instance of KeyVal.
func NewKeyVal(key string, value datasync.LazyValue, rev int64) *KeyVal {
	return &KeyVal{key, value, rev}
}

// GetKey returns the key of the pair.
func (kv *KeyVal) GetKey() string {
	return kv.key
}

// GetRevision returns revision associated with the latest change in the key-value pair.
func (kv *KeyVal) GetRevision() int64 {
	return kv.rev
}

type lazyProto struct {
	val proto.Message
}

// GetValue returns the value of the pair.
func (lazy *lazyProto) GetValue(out proto.Message) error {
	if lazy.val != nil {
		proto.Merge(out, lazy.val)
	}
	return nil
}

// KeyValBytes represents a single key-value pair.
type KeyValBytes struct {
	key   string
	value []byte
	rev   int64
}

// NewKeyValBytes creates a new instance of KeyValBytes.
func NewKeyValBytes(key string, value []byte, rev int64) *KeyValBytes {
	return &KeyValBytes{key, value, rev}
}

// GetKey returns the key of the pair.
func (kv *KeyValBytes) GetKey() string {
	return kv.key
}

// GetValue returns the value of the pair.
func (kv *KeyValBytes) GetValue(message proto.Message) error {
	return json.Unmarshal(kv.value, message)
}

// GetRevision returns revision associated with the latest change in the key-value pair.
func (kv *KeyValBytes) GetRevision() int64 {
	return kv.rev
}
