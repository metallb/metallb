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

package msg

import (
	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
)

//go:generate protoc --proto_path=. --gogo_out=plugins=grpc:. datamsg.proto

// NewChangeWatchResp is a constructor.
func NewChangeWatchResp(message *DataChangeRequest, callback func(error)) *ChangeEvent {
	return &ChangeEvent{
		changes: []datasync.ProtoWatchResp{
			&ChangeWatchResp{message: message},
		},
		callback: callback,
	}
}

// ChangeEvent represents change event with changes.
type ChangeEvent struct {
	changes  []datasync.ProtoWatchResp
	callback func(error)
}

// GetChanges returns list of changes for the change event.
func (ev *ChangeEvent) GetChanges() []datasync.ProtoWatchResp {
	return ev.changes
}

// Done does nothing yet.
func (ev *ChangeEvent) Done(err error) {
	//TODO publish response to the topic
	if err != nil {
		logrus.DefaultLogger().Error(err)
	}
}

// ChangeWatchResp adapts Datamessage to interface datasync.ChangeEvent.
type ChangeWatchResp struct {
	message *DataChangeRequest
}

// GetChangeType - see the comment in implemented interface datasync.ChangeEvent.
func (ev *ChangeWatchResp) GetChangeType() datasync.Op {
	if ev.message.OperationType == PutDel_DEL {
		return datasync.Delete
	}

	return datasync.Put
}

// GetKey returns the key associated with the change.
func (ev *ChangeWatchResp) GetKey() string {
	return ev.message.Key
}

// GetRevision //TODO
func (ev *ChangeWatchResp) GetRevision() int64 {
	return 0
}

// GetValue - see the comments in the interface datasync.ChangeEvent.
func (ev *ChangeWatchResp) GetValue(val proto.Message) error {
	return json.Unmarshal(ev.message.Content, val) //TODO use contentType...
}

// GetPrevValue returns the value before change.
func (ev *ChangeWatchResp) GetPrevValue(prevVal proto.Message) (prevExists bool, err error) {
	if ev.message.OperationType == PutDel_DEL {
		return false, err
	}

	return false, err //TODO prev value
}
