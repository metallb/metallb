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

	"github.com/ligato/cn-infra/datasync/kvdbsync/local"
	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/ligato/cn-infra/db/keyval"
	api "github.com/ligato/vpp-agent/api/genericmanager"
	"github.com/ligato/vpp-agent/pkg/models"
)

// LocalClient is global client for direct local access.
var LocalClient = NewClient(&txnFactory{local.DefaultRegistry})

type client struct {
	txnFactory ProtoTxnFactory
}

// NewClient returns new instance that uses given registry for data propagation.
func NewClient(factory ProtoTxnFactory) ConfigClient {
	return &client{factory}
}

func (c *client) KnownModels() ([]api.ModelInfo, error) {
	var modules []api.ModelInfo
	for _, info := range models.RegisteredModels() {
		modules = append(modules, *info)
	}
	return modules, nil
}

func (c *client) ResyncConfig(items ...proto.Message) error {
	txn := c.txnFactory.NewTxn(true)

	for _, item := range items {
		key, err := models.GetKey(item)
		if err != nil {
			return err
		}
		txn.Put(key, item)
	}

	return txn.Commit()
}

func (c *client) GetConfig(dsts ...interface{}) error {

	return nil
}

func (c *client) ChangeRequest() ChangeRequest {
	return &changeRequest{txn: c.txnFactory.NewTxn(false)}
}

type changeRequest struct {
	txn keyval.ProtoTxn
	err error
}

func (r *changeRequest) Update(items ...proto.Message) ChangeRequest {
	if r.err != nil {
		return r
	}
	for _, item := range items {
		key, err := models.GetKey(item)
		if err != nil {
			r.err = err
			return r
		}
		r.txn.Put(key, item)
	}
	return r
}

func (r *changeRequest) Delete(items ...proto.Message) ChangeRequest {
	if r.err != nil {
		return r
	}
	for _, item := range items {
		key, err := models.GetKey(item)
		if err != nil {
			r.err = err
			return r
		}
		r.txn.Delete(key)
	}
	return r
}

func (r *changeRequest) Send(ctx context.Context) error {
	if r.err != nil {
		return r.err
	}
	return r.txn.Commit()
}

// ProtoTxnFactory defines interface for keyval transaction provider.
type ProtoTxnFactory interface {
	NewTxn(resync bool) keyval.ProtoTxn
}

type txnFactory struct {
	registry *syncbase.Registry
}

func (p *txnFactory) NewTxn(resync bool) keyval.ProtoTxn {
	if resync {
		return local.NewProtoTxn(p.registry.PropagateResync)
	}
	return local.NewProtoTxn(p.registry.PropagateChanges)
}
