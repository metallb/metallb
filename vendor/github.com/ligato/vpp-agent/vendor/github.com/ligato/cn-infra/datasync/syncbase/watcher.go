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
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
)

var (
	// PropagateChangesTimeout defines timeout used during
	// change propagation after which it will return an error.
	PropagateChangesTimeout = time.Second * 20
)

// Registry of subscriptions and latest revisions.
// This structure contains extracted reusable code among various datasync implementations.
// Because of this code, datasync plugins does not need to repeat code related management of subscriptions.
type Registry struct {
	subscriptions map[string]*Subscription
	access        sync.Mutex
	lastRev       *PrevRevisions
}

// Subscription represents single subscription for Registry.
type Subscription struct {
	ResyncName  string
	ChangeChan  chan datasync.ChangeEvent
	ResyncChan  chan datasync.ResyncEvent
	CloseChan   chan string
	KeyPrefixes []string
}

// WatchDataReg implements interface datasync.WatchDataRegistration.
type WatchDataReg struct {
	ResyncName string
	adapter    *Registry
}

// NewRegistry creates reusable registry of subscriptions for a particular datasync plugin.
func NewRegistry() *Registry {
	return &Registry{
		subscriptions: map[string]*Subscription{},
		lastRev:       NewLatestRev(),
	}
}

// Subscriptions returns the current subscriptions.
func (adapter *Registry) Subscriptions() map[string]*Subscription {
	return adapter.subscriptions
}

// LastRev is only a getter.
func (adapter *Registry) LastRev() *PrevRevisions {
	return adapter.lastRev
}

// Watch only appends channels.
func (adapter *Registry) Watch(resyncName string, changeChan chan datasync.ChangeEvent,
	resyncChan chan datasync.ResyncEvent, keyPrefixes ...string) (datasync.WatchRegistration, error) {

	adapter.access.Lock()
	defer adapter.access.Unlock()

	if _, found := adapter.subscriptions[resyncName]; found {
		return nil, errors.New("Already watching " + resyncName)
	}

	adapter.subscriptions[resyncName] = &Subscription{
		ResyncName:  resyncName,
		ChangeChan:  changeChan,
		ResyncChan:  resyncChan,
		CloseChan:   make(chan string),
		KeyPrefixes: keyPrefixes,
	}

	return &WatchDataReg{resyncName, adapter}, nil
}

// PropagateChanges fills registered channels with the data.
func (adapter *Registry) PropagateChanges(txData map[string]datasync.ChangeValue) error {
	var events []func(done chan error)

	for _, sub := range adapter.subscriptions {
		var changes []datasync.ProtoWatchResp

		for _, prefix := range sub.KeyPrefixes {
			for key, val := range txData {
				if !strings.HasPrefix(key, prefix) {
					continue
				}

				var (
					prev   datasync.KeyVal
					curRev int64
				)

				if val.GetChangeType() == datasync.Delete {
					if _, prev = adapter.lastRev.Del(key); prev != nil {
						curRev = prev.GetRevision() + 1
					} else {
						continue
					}
				} else {
					_, prev, curRev = adapter.lastRev.Put(key, val)
				}

				changes = append(changes, &ChangeResp{
					Key:        key,
					ChangeType: val.GetChangeType(),
					CurrVal:    val,
					CurrRev:    curRev,
					PrevVal:    prev,
				})
			}
		}

		if len(changes) > 0 {
			sendTo := func(sub *Subscription) func(done chan error) {
				return func(done chan error) {
					sub.ChangeChan <- &ChangeEvent{
						Changes:  changes,
						delegate: &DoneChannel{done},
					}
				}
			}
			events = append(events, sendTo(sub))
		}
	}

	done := make(chan error, 1)
	go AggregateDone(events, done)

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(PropagateChangesTimeout):
		logrus.DefaultLogger().Warnf("Timeout of aggregated change callback (%v)",
			PropagateChangesTimeout)
	}

	return nil
}

// PropagateResync fills registered channels with the data.
func (adapter *Registry) PropagateResync(txData map[string]datasync.ChangeValue) error {
	adapter.lastRev.Cleanup()

	for _, sub := range adapter.subscriptions {
		resyncEv := NewResyncEventDB(map[string]datasync.KeyValIterator{})

		for _, prefix := range sub.KeyPrefixes {
			var kvs []datasync.KeyVal

			for key, val := range txData {
				if strings.HasPrefix(key, prefix) {
					// TODO: call Put only once for each key (different subscriptions)
					adapter.lastRev.PutWithRevision(key, val)

					kvs = append(kvs, &KeyVal{
						key:       key,
						LazyValue: val,
						rev:       val.GetRevision(),
					})
				}
			}

			resyncEv.its[prefix] = NewKVIterator(kvs)
		}
		sub.ResyncChan <- resyncEv //TODO default and/or timeout
	}

	return nil
}

// Close stops watching of particular KeyPrefixes.
func (reg *WatchDataReg) Close() error {
	reg.adapter.access.Lock()
	defer reg.adapter.access.Unlock()

	for _, sub := range reg.adapter.subscriptions {
		// subscription should have also change channel, otherwise it is not registered
		// for change events
		if sub.ChangeChan != nil && sub.CloseChan != nil {
			// close the channel with all goroutines under subscription
			safeclose.Close(sub.CloseChan)
		}
	}

	delete(reg.adapter.subscriptions, reg.ResyncName)

	return nil
}

// Register starts watching of particular key prefix. Method returns error if key which should be added
// already exists
func (reg *WatchDataReg) Register(resyncName, keyPrefix string) error {
	reg.adapter.access.Lock()
	defer reg.adapter.access.Unlock()

	for resName, sub := range reg.adapter.subscriptions {
		if resName == resyncName {
			// Verify that prefix does not exist yet
			for _, regPrefix := range sub.KeyPrefixes {
				if regPrefix == keyPrefix {
					return fmt.Errorf("prefix %q already exists", keyPrefix)
				}
			}
			sub.KeyPrefixes = append(sub.KeyPrefixes, keyPrefix)
			return nil
		}
	}
	return fmt.Errorf("cannot register prefix %s, resync name %s not found", keyPrefix, resyncName)
}

// Unregister stops watching of particular key prefix. Method returns error if key which should be removed
// does not exist or in case the channel to close goroutine is nil
func (reg *WatchDataReg) Unregister(keyPrefix string) error {
	reg.adapter.access.Lock()
	defer reg.adapter.access.Unlock()

	subs := reg.adapter.subscriptions[reg.ResyncName]
	// verify if key is registered for change events
	if subs.ChangeChan == nil {
		// not an error
		logrus.DefaultLogger().Infof("key %v not registered for change events", keyPrefix)
		return nil
	}
	if subs.CloseChan == nil {
		return fmt.Errorf("unable to unregister key %v, close channel in subscription is nil", keyPrefix)
	}

	for index, prefix := range subs.KeyPrefixes {
		if prefix == keyPrefix {
			subs.KeyPrefixes = append(subs.KeyPrefixes[:index], subs.KeyPrefixes[index+1:]...)
			subs.CloseChan <- keyPrefix
			logrus.DefaultLogger().WithField("resyncName", reg.ResyncName).Infof("Key %v removed from subscription", keyPrefix)
			return nil
		}
	}

	return fmt.Errorf("key %v to unregister was not found", keyPrefix)
}
