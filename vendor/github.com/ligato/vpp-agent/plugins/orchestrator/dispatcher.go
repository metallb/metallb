//  Copyright (c) 2019 Cisco and/or its affiliates.
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

package orchestrator

import (
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/infra"
	"github.com/ligato/cn-infra/rpc/grpc"
	"golang.org/x/net/context"

	api "github.com/ligato/vpp-agent/api/genericmanager"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/pkg/models"
)

// Plugin implements sync service for GRPC.
type Plugin struct {
	Deps

	manager *genericManagerSvc

	// datasync channels
	changeChan   chan datasync.ChangeEvent
	resyncChan   chan datasync.ResyncEvent
	watchDataReg datasync.WatchRegistration

	mu    sync.Mutex
	store *memStore
}

// Deps represents dependencies for the plugin.
type Deps struct {
	infra.PluginDeps

	GRPC        grpc.Server
	KVScheduler kvs.KVScheduler
	Watcher     datasync.KeyValProtoWatcher
}

// Init registers the service to GRPC server.
func (p *Plugin) Init() error {
	p.store = newMemStore()

	// initialize datasync channels
	p.resyncChan = make(chan datasync.ResyncEvent)
	p.changeChan = make(chan datasync.ChangeEvent)

	// register grpc service
	p.manager = &genericManagerSvc{
		log:  p.Log,
		orch: p,
	}

	if grpcServer := p.GRPC.GetServer(); grpcServer != nil {
		api.RegisterGenericManagerServer(grpcServer, p.manager)
	} else {
		p.Log.Infof("grpc server not available")
	}

	return nil
}

// AfterInit subscribes to known NB prefixes.
func (p *Plugin) AfterInit() (err error) {
	go p.watchEvents()

	nbPrefixes := p.KVScheduler.GetRegisteredNBKeyPrefixes()
	if len(nbPrefixes) > 0 {
		p.Log.Infof("starting watch for %d NB prefixes", len(nbPrefixes))
	} else {
		p.Log.Warnf("no NB prefixes found")
	}

	var prefixes []string
	for _, prefix := range nbPrefixes {
		//prefix = path.Join("config", prefix)
		p.Log.Debugf("- watching NB prefix: %s", prefix)
		prefixes = append(prefixes, prefix)
	}

	p.watchDataReg, err = p.Watcher.Watch(p.PluginName.String(),
		p.changeChan, p.resyncChan, prefixes...)
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugin) watchEvents() {
	for {
		select {
		case e := <-p.changeChan:
			p.Log.Debugf("=> received CHANGE event (%v changes)", len(e.GetChanges()))

			/*var kvPairs []kvs.KeyValuePair
			for _, x := range e.GetChanges() {
				p.Log.Debugf(" - %v: %q (rev: %v)",
					x.GetChangeType(), x.GetKey(), x.GetRevision())

				var val proto.Message
				if x.GetChangeType() != datasync.Delete {
					val = x.G
				}

				kvPairs = append(kvPairs, ProtoWatchResp{
					Key:   x.GetKey(),
					Val: val,
				})
			}*/
			var err error
			var kvPairs []KeyValuePair
			for _, x := range e.GetChanges() {
				kv := KeyValuePair{Key:  x.GetKey()}
				if x.GetChangeType() != datasync.Delete {
					kv.Value, err = models.UnmarshalLazyValue(kv.Key, x)
					if err != nil {
						p.Log.Error(err)
						continue
					}
				}
				kvPairs = append(kvPairs, kv)
			}

			ctx := context.Background()
			ctx = kvs.WithRetryDefault(ctx)
			_, err = p.PushData(ctx, kvPairs)
			e.Done(err)

			/*txn := p.KVScheduler.StartNBTransaction()
			for _, x := range e.GetChanges() {
				p.Log.Debugf(" - %v: %q (rev: %v)",
					x.GetChangeType(), x.GetKey(), x.GetRevision())
				if x.GetChangeType() == datasync.Delete {
					txn.SetValue(x.GetKey(), nil)
				} else {
					txn.SetValue(x.GetKey(), x)
				}
			}

			ctx := context.Background()
			//ctx = kvs.WithRetry(ctx, time.Second, true)

			kvErrs, err := txn.Commit(ctx)
			if err != nil {
				p.Log.Errorf("transaction failed: %v", err)
			} else if len(kvErrs) > 0 {
				p.Log.Warnf("transaction finished with %d errors: %+v", len(kvErrs), kvErrs)
			} else {
				p.Log.Infof("transaction successful")
			}
			e.Done(err)*/

		case e := <-p.resyncChan:
			p.Log.Debugf("=> received RESYNC event (%v prefixes)", len(e.GetValues()))

			var kvPairs []KeyValuePair

			n := 0
			for prefix, iter := range e.GetValues() {
				var err error
				var keyVals []datasync.KeyVal
				for x, done := iter.GetNext(); !done; x, done = iter.GetNext() {
					kv := KeyValuePair{Key: x.GetKey()}
					kv.Value, err = models.UnmarshalLazyValue(kv.Key, x)
					if err != nil {
						p.Log.Error(err)
						continue
					}
					kvPairs = append(kvPairs, kv)
					p.Log.Debugf(" -- key: %s", x.GetKey())
					keyVals = append(keyVals, x)
					n++
				}
				if len(keyVals) > 0 {
					p.Log.Debugf("- %q (%v items)", prefix, len(keyVals))
				} else {
					p.Log.Debugf("- %q (no items)", prefix)
				}
				for _, x := range keyVals {
					p.Log.Debugf("\t - %q: (rev: %v)", x.GetKey(), x.GetRevision())
				}
			}
			p.Log.Debugf("Resync with %d items", n)

			ctx := context.Background()
			ctx = kvs.WithResync(ctx, kvs.FullResync, true)
			ctx = kvs.WithRetryDefault(ctx)
			_, err := p.PushData(ctx, kvPairs)
			e.Done(err)

			/*n := 0
			txn := p.KVScheduler.StartNBTransaction()
			for prefix, iter := range e.GetValues() {
				var keyVals []datasync.KeyVal
				for x, done := iter.GetNext(); !done; x, done = iter.GetNext() {
					keyVals = append(keyVals, x)
					txn.SetValue(x.GetKey(), x)
					n++
				}
				if len(keyVals) > 0 {
					p.Log.Debugf(" - Resync: %q (%v items)", prefix, len(keyVals))
				} else {
					p.Log.Debugf(" - Resync: %q", prefix)
				}
				for _, x := range keyVals {
					p.Log.Debugf("\t - %q: (rev: %v)", x.GetKey(), x.GetRevision())
				}
			}
			p.Log.Debugf("Resyncing %d items", n)

			ctx := context.Background()
			//ctx = kvs.WithRetry(ctx, time.Second, true)
			ctx = kvs.WithResync(ctx, kvs.FullResync, true)

			kvErrs, err := txn.Commit(ctx)
			if err != nil {
				p.Log.Errorf("transaction failed: %v", err)
			} else if len(kvErrs) > 0 {
				p.Log.Warnf("transaction finished with %d errors: %+v", len(kvErrs), kvErrs)
			} else {
				p.Log.Infof("transaction successful")
			}
			e.Done(err)*/
		}
	}
}

func (p *Plugin) ListData() map[string]proto.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.db
}

// PushData ...
func (p *Plugin) PushData(ctx context.Context, kvPairs []KeyValuePair) (kvErrs []kvs.KeyWithError, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if typ, _ := kvs.IsResync(ctx); typ == kvs.FullResync {
		p.store.Reset()
	}

	txn := p.KVScheduler.StartNBTransaction()

	for _, kv := range kvPairs {
		if kv.Value == nil {
			p.Log.Debugf(" - DELETE: %q", 	kv.Key)
		} else {

		}
		p.Log.Debugf(" - PUT: %q ", kv.Key)

		if kv.Value == nil {
			txn.SetValue(kv.Key, nil)
			p.store.Delete(kv.Key)
		} else {
			txn.SetValue(kv.Key, kv.Value)
			p.store.Update(kv.Key, kv.Value)
		}
	}

	seqID, err := txn.Commit(ctx)
	if err != nil {
		if txErr, ok := err.(*kvs.TransactionError); ok && len(txErr.GetKVErrors()) > 0 {
			kvErrs = txErr.GetKVErrors()
			p.Log.Errorf("Transaction finished with %d errors: %+v", len(kvErrs), kvErrs)
		} else {
			p.Log.Errorf("Transaction %d failed: %v", seqID, err)
		}
	} else {
		p.Log.Infof("Transaction %d successful!", seqID)
		return kvErrs, err
	}

	return nil, nil
}

// KeyValuePair associates value with its key.
type KeyValuePair struct {
	Key   string
	Value proto.Message
}