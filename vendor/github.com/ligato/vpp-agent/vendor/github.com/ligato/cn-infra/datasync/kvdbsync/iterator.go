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

package kvdbsync

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
)

// Iterator adapts the db_proto.KeyValIterator to the datasync.KeyValIterator.
type Iterator struct {
	delegate keyval.ProtoKeyValIterator
}

// NewIterator creates a new instance of Iterator.
func NewIterator(delegate keyval.ProtoKeyValIterator) *Iterator {
	return &Iterator{delegate: delegate}
}

// GetNext only delegates the call to internal iterator.
func (it *Iterator) GetNext() (kv datasync.KeyVal, stop bool) {
	kv, stop = it.delegate.GetNext()
	if stop {
		return nil, stop
	}
	return kv, stop
}
