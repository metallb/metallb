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

package client

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/vpp-agent/pkg/models"
)

// Txn is the
type Txn struct {
	items map[string]proto.Message
}

// NewTxn
func NewTxn(commitFunc func() error) *Txn {
	return &Txn{
		items: make(map[string]proto.Message),
	}
}

// Update updates
func (t *Txn) Update(item proto.Message) {
	t.items[models.Key(item)] = item
}

func (t *Txn) Delete(item proto.Message) {
	t.items[models.Key(item)] = nil
}

func (t *Txn) Commit(ctx context.Context) {

}

// FindItem returns item with given ID from the request items.
// If the found is true the model with such ID is found
// and if the model is nil the item represents delete.
func (t *Txn) FindItem(id string) (model proto.Message, found bool) {
	item, ok := t.items[id]
	return item, ok
}

// Items returns map of items defined for the request,
// where key represents model ID and nil value represents delete.
// NOTE: Do not alter the returned map directly.
func (t *Txn) ListItems() map[string]proto.Message {
	return t.items
}

// RemoveItem removes an item from the transaction.
// This will revert any Update or Delete done for the item.
func (t *Txn) RemoveItem(model proto.Message) {
	delete(t.items, models.Key(model))
}
