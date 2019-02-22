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

package kvdbsync

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/syncbase"
)

// ChangeWatchResp is a structure that adapts the BytesWatchResp to the
// datasync api.
type ChangeWatchResp struct {
	changes []datasync.ProtoWatchResp
	*syncbase.DoneChannel
}

// NewChangeWatchResp creates a new instance of ChangeWatchResp.
func NewChangeWatchResp(delegate datasync.ProtoWatchResp, prevVal datasync.LazyValue) *ChangeWatchResp {
	return &ChangeWatchResp{
		changes: []datasync.ProtoWatchResp{
			&changePrev{
				ProtoWatchResp: delegate,
				prev:           prevVal,
			},
		},
		DoneChannel: &syncbase.DoneChannel{DoneChan: nil},
	}
}

// GetChanges returns list of changes for the change event.
func (ev *ChangeWatchResp) GetChanges() []datasync.ProtoWatchResp {
	return ev.changes
}

type changePrev struct {
	datasync.ProtoWatchResp
	prev datasync.LazyValue
}

// GetValue returns previous value associated with a change. For description of parameter and output
// values, see the comment in implemented interface datasync.ChangeEvent.
func (ev *changePrev) GetValue(val proto.Message) (err error) {
	if ev.ProtoWatchResp.GetChangeType() != datasync.Delete {
		return ev.ProtoWatchResp.GetValue(val)
	}
	return nil
}

// GetPrevValue returns previous value associated with a change. For description of parameter and output
// values, see the comment in implemented interface datasync.ChangeEvent.
func (ev *changePrev) GetPrevValue(prevVal proto.Message) (exists bool, err error) {
	if ev.prev != nil {
		return true, ev.prev.GetValue(prevVal)
	}
	return false, nil
}
