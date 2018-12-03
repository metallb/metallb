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

package kvproto

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/db/keyval"
)

// protoTxn represents a transaction.
type protoTxn struct {
	serializer keyval.Serializer
	err        error
	txn        keyval.BytesTxn
}

// Put adds a new 'put' operation to a previously created transaction.
// If the <key> does not exist in the data store, a new key-value item
// will be added to the data store. If <key> exists in the data store,
// the existing value will be overwritten with the <value> from this
// operation.
func (tx *protoTxn) Put(key string, value proto.Message) keyval.ProtoTxn {
	if tx.err != nil {
		return tx
	}

	// Marshal value to protobuf.
	binData, err := tx.serializer.Marshal(value)
	if err != nil {
		tx.err = err
		return tx
	}
	tx.txn = tx.txn.Put(key, binData)
	return tx
}

// Delete adds a new 'delete' operation to a previously created
// transaction.
func (tx *protoTxn) Delete(key string) keyval.ProtoTxn {
	if tx.err != nil {
		return tx
	}

	tx.txn = tx.txn.Delete(key)
	return tx
}

// Commit commits all operations in a transaction to the data store.
// Commit is atomic - either all operations in the transaction are
// committed to the data store, or none of them.
func (tx *protoTxn) Commit() error {
	if tx.err != nil {
		return tx.err
	}
	return tx.txn.Commit()
}
