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

package local

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/ligato/cn-infra/db/keyval"
)

// ProtoTxnItem is used in ProtoTxn.
type ProtoTxnItem struct {
	Data   proto.Message
	Delete bool
}

// GetValue returns the value of the pair.
func (item *ProtoTxnItem) GetValue(out proto.Message) error {
	if item.Data != nil {
		proto.Merge(out, item.Data)
	}
	return nil
}

// ProtoTxn is a concurrent map of proto messages.
// The intent is to collect the user data and propagate them when commit happens.
type ProtoTxn struct {
	access sync.Mutex
	items  map[string]*ProtoTxnItem
	commit func(map[string]datasync.ChangeValue) error
}

// NewProtoTxn is a constructor.
func NewProtoTxn(commit func(map[string]datasync.ChangeValue) error) *ProtoTxn {
	return &ProtoTxn{
		items:  make(map[string]*ProtoTxnItem),
		commit: commit,
	}
}

// Put adds store operation into transaction.
func (txn *ProtoTxn) Put(key string, data proto.Message) keyval.ProtoTxn {
	txn.access.Lock()
	defer txn.access.Unlock()

	txn.items[key] = &ProtoTxnItem{Data: data}

	return txn
}

// Delete adds delete operation into transaction.
func (txn *ProtoTxn) Delete(key string) keyval.ProtoTxn {
	txn.access.Lock()
	defer txn.access.Unlock()

	txn.items[key] = &ProtoTxnItem{Delete: true}

	return txn
}

// Commit executes the transaction.
func (txn *ProtoTxn) Commit() error {
	txn.access.Lock()
	defer txn.access.Unlock()

	kvs := map[string]datasync.ChangeValue{}
	for key, item := range txn.items {
		changeType := datasync.Put
		if item.Delete {
			changeType = datasync.Delete
		}

		kvs[key] = syncbase.NewChange(key, item.Data, 0, changeType)
	}
	return txn.commit(kvs)
}
