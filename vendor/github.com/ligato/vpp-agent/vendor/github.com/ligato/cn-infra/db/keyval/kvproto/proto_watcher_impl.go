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

type protoWatcher struct {
	watcher    keyval.BytesWatcher
	serializer keyval.Serializer
}

// protoWatchResp represents a notification about data change.
// It is sent through the <resp> callback as proto-modelled data.
type protoWatchResp struct {
	serializer keyval.Serializer
	keyval.BytesWatchResp
}

// Watch watches for changes in datastore.
// <resp> callback is used for delivery of watch events.
func (pdb *protoWatcher) Watch(resp func(keyval.ProtoWatchResp), closeChan chan string, keys ...string) error {
	err := pdb.watcher.Watch(func(msg keyval.BytesWatchResp) {
		resp(NewWatchResp(pdb.serializer, msg))
	}, closeChan, keys...)
	if err != nil {
		return err
	}
	return nil
}

// NewWatchResp initializes proto watch response from raw WatchResponse <resp>.
func NewWatchResp(serializer keyval.Serializer, resp keyval.BytesWatchResp) keyval.ProtoWatchResp {
	return &protoWatchResp{serializer, resp}
}

// GetValue returns the value after the change.
func (wr *protoWatchResp) GetValue(msg proto.Message) error {
	return wr.serializer.Unmarshal(wr.BytesWatchResp.GetValue(), msg)
}

// GetPrevValue returns the previous value after the change.
func (wr *protoWatchResp) GetPrevValue(msg proto.Message) (prevValueExist bool, err error) {
	prevVal := wr.BytesWatchResp.GetPrevValue()
	if prevVal == nil {
		return false, nil
	}
	err = wr.serializer.Unmarshal(prevVal, msg)
	if err != nil {
		return true, err
	}
	return true, nil
}
