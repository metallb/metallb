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

package keyval

import "github.com/ligato/cn-infra/datasync"

// BytesBroker allows storing, retrieving and removing data in a key-value form.
type BytesBroker interface {
	// Put puts single key-value pair into etcd.
	// The behavior of put can be adjusted using PutOptions.
	Put(key string, data []byte, opts ...datasync.PutOption) error
	// NewTxn creates a transaction.
	NewTxn() BytesTxn
	// GetValue retrieves one item under the provided key.
	GetValue(key string) (data []byte, found bool, revision int64, err error)
	// ListValues returns an iterator that enables to traverse all items stored
	// under the provided <key>.
	ListValues(key string) (BytesKeyValIterator, error)
	// ListKeys returns an iterator that allows to traverse all keys from data
	// store that share the given <prefix>.
	ListKeys(prefix string) (BytesKeyIterator, error)
	// Delete removes data stored under the <key>.
	Delete(key string, opts ...datasync.DelOption) (existed bool, err error)
}

// BytesTxn allows to group operations into the transaction.
// Transaction executes multiple operations in a more efficient way in contrast
// to executing them one by one.
type BytesTxn interface {
	// Put adds put operation (write raw <data> under the given <key>) into
	// the transaction.
	Put(key string, data []byte) BytesTxn
	// Delete adds delete operation (removal of <data> under the given <key>)
	// into the transaction.
	Delete(key string) BytesTxn
	// Commit tries to execute all the operations of the transaction.
	// In the end, either all of them have been successfully applied or none
	// of them and an error is returned.
	Commit() error
}

// BytesKvPair groups getters for a key-value pair.
type BytesKvPair interface {
	// GetValue returns the value of the pair.
	GetValue() []byte
	// GetPrevValue returns the previous value of the pair.
	GetPrevValue() []byte

	datasync.WithKey
}

// BytesKeyVal represents a single item in data store.
type BytesKeyVal interface {
	BytesKvPair
	datasync.WithRevision
}

// BytesKeyValIterator is an iterator returned by ListValues call.
type BytesKeyValIterator interface {
	// GetNext retrieves the following item from the context.
	// When there are no more items to get, <stop> is returned as *true*
	// and <kv> is simply *nil*.
	GetNext() (kv BytesKeyVal, stop bool)
}

// BytesKeyIterator is an iterator returned by ListKeys call.
type BytesKeyIterator interface {
	// GetNext retrieves the following key from the context.
	// When there are no more keys to get, <stop> is returned as *true*
	// and <key>, <rev> are default values.
	GetNext() (key string, rev int64, stop bool)
}

// CoreBrokerWatcher defines methods for full datastore access.
type CoreBrokerWatcher interface {
	BytesBroker
	BytesWatcher
	NewBroker(prefix string) BytesBroker
	NewWatcher(prefix string) BytesWatcher
	Close() error
}
