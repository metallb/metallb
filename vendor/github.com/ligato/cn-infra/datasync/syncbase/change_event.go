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

package syncbase

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
)

// ChangeEvent is a simple structure that implements interface datasync.ChangeEvent.
type ChangeEvent struct {
	Key        string
	ChangeType datasync.Op
	CurrVal    datasync.LazyValue
	CurrRev    int64
	PrevVal    datasync.LazyValue
	delegate   datasync.CallbackResult
}

// GetChangeType returns type of the event.
func (ev *ChangeEvent) GetChangeType() datasync.Op {
	return ev.ChangeType
}

// GetKey returns the Key associated with the change.
func (ev *ChangeEvent) GetKey() string {
	return ev.Key
}

// GetValue - see the comments in the interface datasync.ChangeEvent
func (ev *ChangeEvent) GetValue(val proto.Message) (err error) {
	return ev.CurrVal.GetValue(val)
}

// GetRevision - see the comments in the interface datasync.ChangeEvent
func (ev *ChangeEvent) GetRevision() int64 {
	return ev.CurrRev
}

// GetPrevValue returns the value before change.
func (ev *ChangeEvent) GetPrevValue(prevVal proto.Message) (prevExists bool, err error) {
	if prevVal != nil && ev.PrevVal != nil {
		return true, ev.PrevVal.GetValue(prevVal)
	}
	return false, err
}

// Done propagates call to delegate. If the delegate is nil, then the error is logged (if occurred).
func (ev *ChangeEvent) Done(err error) {
	if ev.delegate != nil {
		ev.delegate.Done(err)
	} else if err != nil {
		logrus.DefaultLogger().Error(err)
	}
}
