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

package consul

import (
	"fmt"

	"github.com/hashicorp/consul/api"

	"github.com/ligato/cn-infra/db/keyval"
)

// Txn allows grouping operations into the transaction. Transaction executes
// multiple operations in a more efficient way in contrast to executing
// them one by one.
type txn struct {
	ops api.KVTxnOps
	kv  *api.KV
}

// Put adds a new 'put' operation to a previously created transaction.
// If the <key> does not exist in the data store, a new key-value item
// will be added to the data store. If <key> exists in the data store,
// the existing value will be overwritten with the <value> from this
// operation.
func (tx *txn) Put(key string, value []byte) keyval.BytesTxn {
	tx.ops = append(tx.ops, &api.KVTxnOp{
		Verb:  api.KVSet,
		Key:   key,
		Value: value,
	})
	return tx
}

// Delete adds a new 'delete' operation to a previously created
// transaction. If <key> exists in the data store, the associated value
// will be removed.
func (tx *txn) Delete(key string) keyval.BytesTxn {
	tx.ops = append(tx.ops, &api.KVTxnOp{
		Verb: api.KVDelete,
		Key:  key,
	})
	return tx
}

// Commit commits all operations in a transaction to the data store.
// Commit is atomic - either all operations in the transaction are
// committed to the data store, or none of them.
func (tx *txn) Commit() error {
	ok, resp, _, err := tx.kv.Txn(tx.ops, nil)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("transaction failed: %v", resp)
	}
	return nil
}
