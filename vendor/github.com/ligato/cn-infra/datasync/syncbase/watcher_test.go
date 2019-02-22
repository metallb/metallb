// Copyright (c) 2018 Cisco and/or its affiliates.
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
	"context"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/syncbase/msg"
	. "github.com/onsi/gomega"
)

// TestDeleteNonExisting verifies that delete operation for key with no
// prev value does not trigger change event
func TestDeleteNonExisting(t *testing.T) {

	const subPrefix = "/sub/prefix/"

	RegisterTestingT(t)

	var changes []datasync.ProtoWatchResp

	ctx, cancelFnc := context.WithCancel(context.Background())
	changeCh := make(chan datasync.ChangeEvent)
	resynCh := make(chan datasync.ResyncEvent)
	reg := NewRegistry()

	// register watcher
	wr, err := reg.Watch("resyncname", changeCh, resynCh, subPrefix)
	Expect(err).To(BeNil())
	Expect(wr).NotTo(BeNil())

	// collect the change events
	go func() {
		for {
			select {
			case c := <-changeCh:
				changes = append(changes, c.GetChanges()...)
				c.Done(nil)
			case <-ctx.Done():
				break
			}
		}
	}()

	// execute the first set of changes
	changesToBePropagated := make(map[string]datasync.ChangeValue)

	// since the prev value does not exist this item should no trigger a change notification
	changesToBePropagated[subPrefix+"nonExistingDelete"] = NewChange(subPrefix+"nonExisting", nil, 0, datasync.Delete)
	// put should be propagated
	changesToBePropagated[subPrefix+"new"] = NewChange(subPrefix+"new", nil, 0, datasync.Put)

	err = reg.PropagateChanges(changesToBePropagated)
	Expect(err).To(BeNil())

	Expect(len(changes)).To(BeEquivalentTo(1))
	Expect(changes[0].GetKey()).To(BeEquivalentTo(subPrefix + "new"))
	Expect(changes[0].GetChangeType()).To(BeEquivalentTo(datasync.Put))

	// clear the changes
	changes = nil

	// remove an item that exist
	deleteItemThatExists := make(map[string]datasync.ChangeValue)
	deleteItemThatExists[subPrefix+"new"] = NewChange(subPrefix+"new", nil, 0, datasync.Delete)

	err = reg.PropagateChanges(deleteItemThatExists)
	Expect(err).To(BeNil())

	Expect(len(changes)).To(BeEquivalentTo(1))
	Expect(changes[0].GetKey()).To(BeEquivalentTo(subPrefix + "new"))
	Expect(changes[0].GetChangeType()).To(BeEquivalentTo(datasync.Delete))

	cancelFnc()

}

// TestRuntimeResync verifies that prev value in *Events contain expected value in case of runtime resync
func TestRuntimeResync(t *testing.T) {

	const subPrefix = "/sub/prefix/"

	RegisterTestingT(t)

	var changes []datasync.ProtoWatchResp
	var resyncChanges []datasync.KeyVal
	ctx, cancelFnc := context.WithCancel(context.Background())

	changeCh := make(chan datasync.ChangeEvent)
	resynCh := make(chan datasync.ResyncEvent)
	reg := NewRegistry()

	// register watcher
	wr, err := reg.Watch("resyncname", changeCh, resynCh, subPrefix)
	Expect(err).To(BeNil())
	Expect(wr).NotTo(BeNil())

	// collect  events
	go func() {
		for {
			select {
			case c := <-changeCh:
				changes = append(changes, c.GetChanges()...)
				c.Done(nil)
			case r := <-resynCh:
				for _, v := range r.GetValues() {
					for {
						kv, done := v.GetNext()
						if done {
							break
						}
						resyncChanges = append(resyncChanges, kv)
					}
				}
				r.Done(nil)
			case <-ctx.Done():
				break
			}
		}
	}()

	createData := func(s string) proto.Message {
		value := msg.PingRequest{}
		value.Message = s
		return &value
	}

	// 1. execute the first set of changes
	changesToBePropagated := make(map[string]datasync.ChangeValue)

	changesToBePropagated[subPrefix+"A"] = NewChange(subPrefix+"A", createData("A"), 0, datasync.Put)

	err = reg.PropagateChanges(changesToBePropagated)
	Expect(err).To(BeNil())

	Expect(len(changes)).To(BeEquivalentTo(1))
	Expect(len(resyncChanges)).To(BeEquivalentTo(0))
	Expect(changes[0].GetKey()).To(BeEquivalentTo(subPrefix + "A"))
	Expect(changes[0].GetChangeType()).To(BeEquivalentTo(datasync.Put))

	prev := msg.PingRequest{}
	exists, err := changes[0].GetPrevValue(&prev)
	Expect(err).To(BeNil())
	Expect(exists).To(BeFalse())

	changes = nil
	resyncChanges = nil

	// 2. runtime resync
	resyncToBePropagated := make(map[string]datasync.ChangeValue)

	resyncToBePropagated[subPrefix+"X"] = NewChange(subPrefix+"X", createData("X"), 0, datasync.Put)
	resyncToBePropagated[subPrefix+"Y"] = NewChange(subPrefix+"Y", createData("Y"), 0, datasync.Put)

	err = reg.PropagateResync(resyncToBePropagated)
	Expect(err).To(BeNil())

	// Since propagateResync doesn't wait for acknowledge whereas propagateChanges does 'Eventually' must be used.
	Eventually(func() int { return len(resyncChanges) }).Should(BeEquivalentTo(2))
	keys := []string{resyncChanges[0].GetKey(), resyncChanges[1].GetKey()}
	Expect(keys).To(ContainElement(subPrefix + "X"))
	Expect(keys).To(ContainElement(subPrefix + "Y"))

	changes = nil
	resyncChanges = nil

	// 3. put a key that is supposed to be removed by resync, verify that prev value does not exist
	changesToBePropagated[subPrefix+"A"] = NewChange(subPrefix+"A", createData("abc"), 1, datasync.Put)
	err = reg.PropagateChanges(changesToBePropagated)
	Expect(err).To(BeNil())

	Expect(len(changes)).To(BeEquivalentTo(1))
	Expect(changes[0].GetKey()).To(BeEquivalentTo(subPrefix + "A"))
	Expect(changes[0].GetChangeType()).To(BeEquivalentTo(datasync.Put))

	current := msg.PingRequest{}
	prev = msg.PingRequest{}

	err = changes[0].GetValue(&current)
	Expect(err).To(BeNil())
	Expect(current.Message).To(BeEquivalentTo("abc"))

	exists, err = changes[0].GetPrevValue(&prev)
	Expect(err).To(BeNil())
	Expect(exists).To(BeFalse())
	changes = nil
	resyncChanges = nil

	// 4. Update the value
	changesToBePropagated = make(map[string]datasync.ChangeValue)

	changesToBePropagated[subPrefix+"A"] = NewChange(subPrefix+"A", createData("A"), 0, datasync.Put)

	err = reg.PropagateChanges(changesToBePropagated)
	Expect(err).To(BeNil())

	Expect(len(changes)).To(BeEquivalentTo(1))
	Expect(len(resyncChanges)).To(BeEquivalentTo(0))
	Expect(changes[0].GetKey()).To(BeEquivalentTo(subPrefix + "A"))
	Expect(changes[0].GetChangeType()).To(BeEquivalentTo(datasync.Put))

	prev = msg.PingRequest{}
	exists, err = changes[0].GetPrevValue(&prev)
	Expect(err).To(BeNil())
	Expect(exists).To(BeTrue())
	Expect(prev.Message).To(BeEquivalentTo("abc"))

	cancelFnc()

}
