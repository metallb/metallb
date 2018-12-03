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

package etcd

import (
	"github.com/ligato/cn-infra/datasync"
)

// BytesWatchPutResp is sent when new key-value pair has been inserted
// or the value has been updated.
type BytesWatchPutResp struct {
	key       string
	value     []byte
	prevValue []byte
	rev       int64
}

// NewBytesWatchPutResp creates an instance of BytesWatchPutResp.
func NewBytesWatchPutResp(key string, value []byte, prevValue []byte, revision int64) *BytesWatchPutResp {
	return &BytesWatchPutResp{key: key, value: value, prevValue: prevValue, rev: revision}
}

// GetChangeType returns "Put" for BytesWatchPutResp.
func (resp *BytesWatchPutResp) GetChangeType() datasync.Op {
	return datasync.Put
}

// GetKey returns the key that the value has been inserted under.
func (resp *BytesWatchPutResp) GetKey() string {
	return resp.key
}

// GetValue returns the value that has been inserted.
func (resp *BytesWatchPutResp) GetValue() []byte {
	return resp.value
}

// GetPrevValue returns the previous value that has been inserted.
func (resp *BytesWatchPutResp) GetPrevValue() []byte {
	return resp.prevValue
}

// GetRevision returns the revision associated with the 'put' operation.
func (resp *BytesWatchPutResp) GetRevision() int64 {
	return resp.rev
}

// BytesWatchDelResp is sent when a key-value pair has been removed.
type BytesWatchDelResp struct {
	key       string
	prevValue []byte
	rev       int64
}

// NewBytesWatchDelResp creates an instance of BytesWatchDelResp.
func NewBytesWatchDelResp(key string, prevValue []byte, revision int64) *BytesWatchDelResp {
	return &BytesWatchDelResp{key: key, prevValue: prevValue, rev: revision}
}

// GetChangeType returns "Delete" for BytesWatchPutResp.
func (resp *BytesWatchDelResp) GetChangeType() datasync.Op {
	return datasync.Delete
}

// GetKey returns the key that a value has been deleted from.
func (resp *BytesWatchDelResp) GetKey() string {
	return resp.key
}

// GetValue returns nil for BytesWatchDelResp.
func (resp *BytesWatchDelResp) GetValue() []byte {
	return nil
}

// GetPrevValue returns previous value for BytesWatchDelResp.
func (resp *BytesWatchDelResp) GetPrevValue() []byte {
	return resp.prevValue
}

// GetRevision returns the revision associated with the 'delete' operation.
func (resp *BytesWatchDelResp) GetRevision() int64 {
	return resp.rev
}
