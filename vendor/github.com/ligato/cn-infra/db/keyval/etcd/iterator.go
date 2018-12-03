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

package etcd

import (
	"github.com/coreos/etcd/clientv3"
	"github.com/ligato/cn-infra/db/keyval"
)

// bytesKeyValIterator is an iterator returned by ListValues call.
type bytesKeyValIterator struct {
	index int
	len   int
	resp  *clientv3.GetResponse
}

// bytesKeyIterator is an iterator returned by ListKeys call.
type bytesKeyIterator struct {
	index int
	len   int
	resp  *clientv3.GetResponse
}

// bytesKeyVal represents a single key-value pair.
type bytesKeyVal struct {
	key       string
	value     []byte
	prevValue []byte
	revision  int64
}

// GetNext returns the following item from the result set.
// When there are no more items to get, <stop> is returned as *true* and <val>
// is simply *nil*.
func (ctx *bytesKeyValIterator) GetNext() (val keyval.BytesKeyVal, stop bool) {
	if ctx.index >= ctx.len {
		return nil, true
	}

	key := string(ctx.resp.Kvs[ctx.index].Key)
	data := ctx.resp.Kvs[ctx.index].Value
	rev := ctx.resp.Kvs[ctx.index].ModRevision

	var prevValue []byte
	if len(ctx.resp.Kvs) > 0 && ctx.index > 0 {
		prevValue = ctx.resp.Kvs[ctx.index-1].Value
	}

	ctx.index++

	return &bytesKeyVal{key, data, prevValue, rev}, false
}

// GetNext returns the following key (+ revision) from the result set.
// When there are no more keys to get, <stop> is returned as *true*
// and <key> and <rev> are default values.
func (ctx *bytesKeyIterator) GetNext() (key string, rev int64, stop bool) {
	if ctx.index >= ctx.len {
		return "", 0, true
	}

	key = string(ctx.resp.Kvs[ctx.index].Key)
	rev = ctx.resp.Kvs[ctx.index].ModRevision
	ctx.index++

	return key, rev, false
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (ctx *bytesKeyIterator) Close() error {
	return nil
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (kv *bytesKeyVal) Close() error {
	return nil
}

// GetValue returns the value of the pair.
func (kv *bytesKeyVal) GetValue() []byte {
	return kv.value
}

// GetPrevValue returns the previous value of the pair.
func (kv *bytesKeyVal) GetPrevValue() []byte {
	return kv.prevValue
}

// GetKey returns the key of the pair.
func (kv *bytesKeyVal) GetKey() string {
	return kv.key
}

// GetRevision returns the revision associated with the pair.
func (kv *bytesKeyVal) GetRevision() int64 {
	return kv.revision
}
