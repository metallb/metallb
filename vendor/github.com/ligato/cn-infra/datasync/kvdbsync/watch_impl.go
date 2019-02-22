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
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/datasync/syncbase"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging/logrus"
)

var (
	// ResyncTimeout defines timeout used during
	// resync after which resync will return an error.
	ResyncTimeout = time.Second * 5
)

// WatchBrokerKeys implements go routines on top of Change & Resync channels.
type watchBrokerKeys struct {
	resyncReg  resync.Registration
	changeChan chan datasync.ChangeEvent
	resyncChan chan datasync.ResyncEvent
	prefixes   []string
	adapter    *watcher
}

type watcher struct {
	db   keyval.ProtoBroker
	dbW  keyval.ProtoWatcher
	base *syncbase.Registry
}

// WatchAndResyncBrokerKeys calls keyval watcher Watch() & resync Register().
// This creates go routines for each tuple changeChan + resyncChan.
func watchAndResyncBrokerKeys(resyncReg resync.Registration, changeChan chan datasync.ChangeEvent, resyncChan chan datasync.ResyncEvent,
	closeChan chan string, adapter *watcher, keyPrefixes ...string) (keys *watchBrokerKeys, err error) {
	keys = &watchBrokerKeys{
		resyncReg:  resyncReg,
		changeChan: changeChan,
		resyncChan: resyncChan,
		adapter:    adapter,
		prefixes:   keyPrefixes,
	}

	var wasErr error
	if err := keys.resyncRev(); err != nil {
		wasErr = err
	}
	if resyncReg != nil {
		go keys.watchResync(resyncReg)
	}
	if changeChan != nil {
		if err := keys.adapter.dbW.Watch(keys.watchChanges, closeChan, keys.prefixes...); err != nil {
			wasErr = err
		}
	}
	return keys, wasErr
}

func (keys *watchBrokerKeys) watchChanges(x datasync.ProtoWatchResp) {
	var prev datasync.LazyValue
	if datasync.Delete == x.GetChangeType() {
		_, prev = keys.adapter.base.LastRev().Del(x.GetKey())
	} else {
		_, prev = keys.adapter.base.LastRev().PutWithRevision(x.GetKey(),
			syncbase.NewKeyVal(x.GetKey(), x, x.GetRevision()))
	}

	ch := NewChangeWatchResp(x, prev)
	keys.changeChan <- ch
	// TODO NICE-to-HAVE publish the err using the transport asynchronously
}

// resyncReg.StatusChan == Started => resync
func (keys *watchBrokerKeys) watchResync(resyncReg resync.Registration) {
	for resyncStatus := range resyncReg.StatusChan() {
		if resyncStatus.ResyncStatus() == resync.Started {
			err := keys.resync()
			if err != nil {
				// We are not able to propagate it somewhere else.
				logrus.DefaultLogger().Errorf("getting resync data failed: %v", err)
				// TODO NICE-to-HAVE publish the err using the transport asynchronously
			}
		}
		resyncStatus.Ack()
	}
}

// ResyncRev fill the PrevRevision map. This step needs to be done even if resync is ommited
func (keys *watchBrokerKeys) resyncRev() error {
	for _, keyPrefix := range keys.prefixes {
		revIt, err := keys.adapter.db.ListValues(keyPrefix)
		if err != nil {
			return err
		}
		// if there are data for given prefix, register it
		for {
			data, stop := revIt.GetNext()
			if stop {
				break
			}
			logrus.DefaultLogger().Debugf("registering key found in KV: %q", data.GetKey())

			keys.adapter.base.LastRev().PutWithRevision(data.GetKey(),
				syncbase.NewKeyVal(data.GetKey(), data, data.GetRevision()))
		}
	}

	return nil
}

// Resync fills the resyncChan with the most recent snapshot (db.ListValues).
func (keys *watchBrokerKeys) resync() error {
	iterators := map[string]datasync.KeyValIterator{}
	for _, keyPrefix := range keys.prefixes {
		it, err := keys.adapter.db.ListValues(keyPrefix)
		if err != nil {
			return err
		}
		iterators[keyPrefix] = NewIterator(it)
	}

	resyncEvent := syncbase.NewResyncEventDB(iterators)
	keys.resyncChan <- resyncEvent

	select {
	case err := <-resyncEvent.DoneChan:
		if err != nil {
			return err
		}
	case <-time.After(ResyncTimeout):
		logrus.DefaultLogger().Warn("Timeout of resync callback")
	}

	return nil
}

// String returns resyncName.
func (keys *watchBrokerKeys) String() string {
	return keys.resyncReg.String()
}
