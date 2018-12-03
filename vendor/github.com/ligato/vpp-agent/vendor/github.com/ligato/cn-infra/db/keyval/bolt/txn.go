//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package bolt

import (
	"github.com/boltdb/bolt"
	"github.com/ligato/cn-infra/db/keyval"
)

// Txn allows grouping operations into the transaction. Transaction executes
// multiple operations in a more efficient way in contrast to executing
// them one by one.
type txn struct {
	db       *bolt.DB
	putPairs []*kvPair
	delKeys  []string
}

// Put adds a new 'put' operation to a previously created transaction.
// If the <key> does not exist in the data store, a new key-value item
// will be added to the data store. If <key> exists in the data store,
// the existing value will be overwritten with the <value> from this
// operation.
func (t *txn) Put(key string, value []byte) keyval.BytesTxn {
	t.putPairs = append(t.putPairs, &kvPair{
		Key:   key,
		Value: value,
	})
	return t
}

// Delete adds a new 'delete' operation to a previously created
// transaction. If <key> exists in the data store, the associated value
// will be removed.
func (t *txn) Delete(key string) keyval.BytesTxn {
	t.delKeys = append(t.delKeys, key)
	return t
}

// Commit commits all operations in a transaction to the data store.
// Commit is atomic - either all operations in the transaction are
// committed to the data store, or none of them.
func (t *txn) Commit() error {
	return t.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(rootBucket)
		for _, pair := range t.putPairs {
			if err := b.Put([]byte(pair.Key), pair.Value); err != nil {
				return err
			}
		}
		for _, key := range t.delKeys {
			if err := b.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
}
