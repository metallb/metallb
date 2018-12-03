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
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/namespace"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"golang.org/x/net/context"
)

// BytesConnectionEtcd encapsulates the connection to etcd.
// It provides API to read/edit and watch values from etcd.
type BytesConnectionEtcd struct {
	logging.Logger
	etcdClient *clientv3.Client
	lessor     clientv3.Lease
	opTimeout  time.Duration
}

// BytesBrokerWatcherEtcd uses BytesConnectionEtcd to access the datastore.
// The connection can be shared among multiple BytesBrokerWatcherEtcd.
// In case of accessing a particular subtree in etcd only,
// BytesBrokerWatcherEtcd allows defining a keyPrefix that is prepended
// to all keys in its methods in order to shorten keys used in arguments.
type BytesBrokerWatcherEtcd struct {
	logging.Logger
	lessor    clientv3.Lease
	kv        clientv3.KV
	watcher   clientv3.Watcher
	opTimeout time.Duration
}

// NewEtcdConnectionWithBytes creates new connection to etcd based on the given
// config file.
func NewEtcdConnectionWithBytes(config ClientConfig, log logging.Logger) (*BytesConnectionEtcd, error) {
	start := time.Now()
	etcdClient, err := clientv3.New(*config.Config)
	if err != nil {
		log.Debugf("Unable to connect to ETCD %v, Error: '%s'", config.Endpoints, err)
		return nil, err
	}
	etcdConnectTime := time.Since(start)
	log.WithField("durationInNs", etcdConnectTime.Nanoseconds()).Info("Connecting to etcd took ", etcdConnectTime)

	conn, err := NewEtcdConnectionUsingClient(etcdClient, log)
	conn.opTimeout = config.OpTimeout

	return conn, err
}

// NewEtcdConnectionUsingClient creates a new instance of BytesConnectionEtcd
// using the provided etcd client.
// This constructor is used primarily for testing.
func NewEtcdConnectionUsingClient(etcdClient *clientv3.Client, log logging.Logger) (*BytesConnectionEtcd, error) {
	log.Debug("NewEtcdConnectionWithBytes", etcdClient)
	conn := BytesConnectionEtcd{
		Logger:     log,
		etcdClient: etcdClient,
		lessor:     clientv3.NewLease(etcdClient),
		opTimeout:  defaultOpTimeout,
	}
	return &conn, nil
}

// Close closes the connection to ETCD.
func (db *BytesConnectionEtcd) Close() error {
	if db.etcdClient != nil {
		return db.etcdClient.Close()
	}
	return nil
}

// NewBroker creates a new instance of a proxy that provides
// access to etcd. The proxy will reuse the connection from BytesConnectionEtcd.
// <prefix> will be prepended to the key argument in all calls from the created
// BytesBrokerWatcherEtcd. To avoid using a prefix, pass keyval. Root constant as
// an argument.
func (db *BytesConnectionEtcd) NewBroker(prefix string) keyval.BytesBroker {
	return &BytesBrokerWatcherEtcd{
		Logger:    db.Logger,
		kv:        namespace.NewKV(db.etcdClient, prefix),
		lessor:    db.lessor,
		opTimeout: db.opTimeout,
		watcher:   namespace.NewWatcher(db.etcdClient, prefix),
	}
}

// NewWatcher creates a new instance of a proxy that provides
// access to etcd. The proxy will reuse the connection from BytesConnectionEtcd.
// <prefix> will be prepended to the key argument in all calls on created
// BytesBrokerWatcherEtcd. To avoid using a prefix, pass keyval. Root constant as
// an argument.
func (db *BytesConnectionEtcd) NewWatcher(prefix string) keyval.BytesWatcher {
	return &BytesBrokerWatcherEtcd{
		Logger:    db.Logger,
		kv:        namespace.NewKV(db.etcdClient, prefix),
		lessor:    db.lessor,
		opTimeout: db.opTimeout,
		watcher:   namespace.NewWatcher(db.etcdClient, prefix),
	}
}

// Put calls 'Put' function of the underlying BytesConnectionEtcd.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BytesBrokerWatcherEtcd) Put(key string, data []byte, opts ...datasync.PutOption) error {
	return putInternal(pdb.Logger, pdb.kv, pdb.lessor, pdb.opTimeout, key, data, opts...)
}

// NewTxn creates a new transaction.
// KeyPrefix defined in constructor will be prepended to all key arguments
// in the transaction.
func (pdb *BytesBrokerWatcherEtcd) NewTxn() keyval.BytesTxn {
	return newTxnInternal(pdb.kv)
}

// GetValue calls 'GetValue' function of the underlying BytesConnectionEtcd.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BytesBrokerWatcherEtcd) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	return getValueInternal(pdb.Logger, pdb.kv, pdb.opTimeout, key)
}

// ListValues calls 'ListValues' function of the underlying BytesConnectionEtcd.
// KeyPrefix defined in constructor is prepended to the key argument.
// The prefix is removed from the keys of the returned values.
func (pdb *BytesBrokerWatcherEtcd) ListValues(key string) (keyval.BytesKeyValIterator, error) {
	return listValuesInternal(pdb.Logger, pdb.kv, pdb.opTimeout, key)
}

// ListKeys calls 'ListKeys' function of the underlying BytesConnectionEtcd.
// KeyPrefix defined in constructor is prepended to the argument.
func (pdb *BytesBrokerWatcherEtcd) ListKeys(prefix string) (keyval.BytesKeyIterator, error) {
	return listKeysInternal(pdb.Logger, pdb.kv, pdb.opTimeout, prefix)
}

// Delete calls 'Delete' function of the underlying BytesConnectionEtcd.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BytesBrokerWatcherEtcd) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return deleteInternal(pdb.Logger, pdb.kv, pdb.opTimeout, key, opts...)
}

// Watch starts subscription for changes associated with the selected <keys>.
// KeyPrefix defined in constructor is prepended to all <keys> in the argument
// list. The prefix is removed from the keys returned in watch events.
// Watch events will be delivered to <resp> callback.
func (pdb *BytesBrokerWatcherEtcd) Watch(resp func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	for _, key := range keys {
		err := watchInternal(pdb.Logger, pdb.watcher, closeChan, key, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleWatchEvent(log logging.Logger, resp func(keyval.BytesWatchResp), ev *clientv3.Event) {
	var prevKvValue []byte
	if ev.PrevKv != nil {
		prevKvValue = ev.PrevKv.Value
	}

	if ev.Type == mvccpb.DELETE {
		resp(NewBytesWatchDelResp(string(ev.Kv.Key), prevKvValue, ev.Kv.ModRevision))
	} else if ev.IsCreate() || ev.IsModify() {
		if ev.Kv.Value != nil {
			resp(NewBytesWatchPutResp(string(ev.Kv.Key), ev.Kv.Value, prevKvValue, ev.Kv.ModRevision))
		}
	}
}

// NewTxn creates a new transaction. A transaction can hold multiple operations
// that are all committed to the data store together. After a transaction
// has been created, one or more operations (put or delete) can be added
// to the transaction before it is committed.
func (db *BytesConnectionEtcd) NewTxn() keyval.BytesTxn {
	return newTxnInternal(db.etcdClient)
}

func newTxnInternal(kv clientv3.KV) keyval.BytesTxn {
	return &bytesTxn{
		kv: kv,
	}
}

// Watch starts subscription for changes associated with the selected keys.
// Watch events will be delivered to <resp> callback.
// closeCh is a channel closed when Close method is called.It is leveraged
// to stop go routines from specific subscription, or only goroutine with
// provided key prefix
func (db *BytesConnectionEtcd) Watch(resp func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	for _, key := range keys {
		err := watchInternal(db.Logger, db.etcdClient, closeChan, key, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

// watchInternal starts the watch subscription for the key.
func watchInternal(log logging.Logger, watcher clientv3.Watcher, closeCh chan string, prefix string, resp func(keyval.BytesWatchResp)) error {
	recvChan := watcher.Watch(context.Background(), prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())

	go func(registeredKey string) {
		var compactRev int64
		for {
			select {
			case wresp, ok := <-recvChan:
				if !ok {
					log.WithField("prefix", prefix).Warn("Watch recv channel was closed")
					if compactRev != 0 {
						recvChan = watcher.Watch(context.Background(), prefix,
							clientv3.WithPrefix(), clientv3.WithPrevKV(), clientv3.WithRev(compactRev))
						log.WithFields(logging.Fields{
							"prefix": prefix,
							"rev":    compactRev,
						}).Warn("Watch recv channel was re-created")
						compactRev = 0
						continue
					}
					return
				}
				if wresp.Canceled {
					log.WithField("prefix", prefix).Warn("Watch was canceled")
				}
				err := wresp.Err()
				if err != nil {
					log.WithFields(logging.Fields{
						"prefix": prefix,
						"err":    err,
					}).Warn("Watch returned error")
				}
				compactRev = wresp.CompactRevision
				if compactRev != 0 {
					log.WithFields(logging.Fields{
						"prefix": prefix,
						"rev":    compactRev,
					}).Warn("Watched data were compacted ")
				}
				for _, ev := range wresp.Events {
					handleWatchEvent(log, resp, ev)
				}

			case closeVal, ok := <-closeCh:
				if !ok || closeVal == registeredKey {
					log.WithField("prefix", prefix).Debug("Watch ended")
					return
				}
			}
		}
	}(prefix)

	return nil
}

// Put writes the provided key-value item into the data store.
// Returns an error if the item could not be written, nil otherwise.
func (db *BytesConnectionEtcd) Put(key string, binData []byte, opts ...datasync.PutOption) error {
	return putInternal(db.Logger, db.etcdClient, db.lessor, db.opTimeout, key, binData, opts...)
}

func putInternal(log logging.Logger, kv clientv3.KV, lessor clientv3.Lease, opTimeout time.Duration, key string,
	binData []byte, opts ...datasync.PutOption) error {

	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	var etcdOpts []clientv3.OpOption
	for _, o := range opts {
		if withTTL, ok := o.(*datasync.WithTTLOpt); ok && withTTL.TTL > 0 {
			lease, err := lessor.Grant(ctx, int64(withTTL.TTL/time.Second))
			if err != nil {
				return err
			}

			etcdOpts = append(etcdOpts, clientv3.WithLease(lease.ID))
		}
	}

	if _, err := kv.Put(ctx, key, string(binData), etcdOpts...); err != nil {
		log.Error("etcd put error: ", err)
		return err
	}

	return nil
}

// PutIfNotExists puts given key-value pair into etcd if there is no value set for the key. If the put was successful
// succeeded is true. If the key already exists succeeded is false and the value for the key is untouched.
func (db *BytesConnectionEtcd) PutIfNotExists(key string, binData []byte) (succeeded bool, err error) {
	// if key doesn't exist its version is equal to 0
	response, err := db.etcdClient.Txn(context.Background()).
		If(clientv3.Compare(clientv3.Version(key), "=", 0)).
		Then(clientv3.OpPut(key, string(binData))).
		Commit()
	if err != nil {
		return false, err
	}
	return response.Succeeded, nil
}

// Delete removes data identified by the <key>.
func (db *BytesConnectionEtcd) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return deleteInternal(db.Logger, db.etcdClient, db.opTimeout, key, opts...)
}

func deleteInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, key string, opts ...datasync.DelOption) (existed bool, err error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	var etcdOpts []clientv3.OpOption
	for _, o := range opts {
		if _, ok := o.(*datasync.WithPrefixOpt); ok {
			etcdOpts = append(etcdOpts, clientv3.WithPrefix())
		}
	}

	// delete data from etcd
	resp, err := kv.Delete(ctx, key, etcdOpts...)
	if err != nil {
		log.Error("etcd error: ", err)
		return false, err
	}

	if len(resp.PrevKvs) != 0 {
		return true, nil
	}

	return false, nil
}

// GetValue retrieves one key-value item from the data store. The item
// is identified by the provided <key>.
func (db *BytesConnectionEtcd) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	return getValueInternal(db.Logger, db.etcdClient, db.opTimeout, key)
}

func getValueInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, key string) (data []byte, found bool, revision int64, err error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// get data from etcd
	resp, err := kv.Get(ctx, key)
	if err != nil {
		log.Error("etcd get error: ", err)
		return nil, false, 0, err
	}

	for _, ev := range resp.Kvs {
		return ev.Value, true, ev.ModRevision, nil
	}

	return nil, false, 0, nil
}

// GetValueRev retrieves one key-value item from the data store. The item
// is identified by the provided <key>.
func (db *BytesConnectionEtcd) GetValueRev(key string, rev int64) (data []byte, found bool, revision int64, err error) {
	return getValueRevInternal(db.Logger, db.etcdClient, db.opTimeout, key, rev)
}

func getValueRevInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration,
	key string, rev int64) (data []byte, found bool, revision int64, err error) {

	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// get data from etcd
	resp, err := kv.Get(ctx, key, clientv3.WithRev(rev))
	if err != nil {
		log.Error("etcd get error: ", err)
		return nil, false, 0, err
	}

	for _, ev := range resp.Kvs {
		return ev.Value, true, ev.ModRevision, nil
	}
	return nil, false, 0, nil
}

// ListValues returns an iterator that enables traversing values stored under
// the provided <key>.
func (db *BytesConnectionEtcd) ListValues(key string) (keyval.BytesKeyValIterator, error) {
	return listValuesInternal(db.Logger, db.etcdClient, db.opTimeout, key)
}

func listValuesInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, key string) (keyval.BytesKeyValIterator, error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// get data from etcd
	resp, err := kv.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		log.Error("etcd error: ", err)
		return nil, err
	}

	return &bytesKeyValIterator{len: len(resp.Kvs), resp: resp}, nil
}

// ListKeys returns an iterator that allows traversing all keys from data
// store that share the given <prefix>.
func (db *BytesConnectionEtcd) ListKeys(prefix string) (keyval.BytesKeyIterator, error) {
	return listKeysInternal(db.Logger, db.etcdClient, db.opTimeout, prefix)
}

func listKeysInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, prefix string) (keyval.BytesKeyIterator, error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// get data from etcd
	resp, err := kv.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		log.Error("etcd error: ", err)
		return nil, err
	}

	return &bytesKeyIterator{len: len(resp.Kvs), resp: resp}, nil
}

// ListValuesRange returns an iterator that enables traversing values stored
// under the keys from a given range.
func (db *BytesConnectionEtcd) ListValuesRange(fromPrefix string, toPrefix string) (keyval.BytesKeyValIterator, error) {
	return listValuesRangeInternal(db.Logger, db.etcdClient, db.opTimeout, fromPrefix, toPrefix)
}

func listValuesRangeInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, fromPrefix string, toPrefix string) (keyval.BytesKeyValIterator, error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// get data from etcd
	resp, err := kv.Get(ctx, fromPrefix, clientv3.WithRange(toPrefix))
	if err != nil {
		log.Error("etcd error: ", err)
		return nil, err
	}

	return &bytesKeyValIterator{len: len(resp.Kvs), resp: resp}, nil
}

// Compact compacts the ETCD database to specific revision
func (db *BytesConnectionEtcd) Compact(rev ...int64) (int64, error) {
	return compactInternal(db.Logger, db.etcdClient, db.opTimeout, rev...)
}

func compactInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration, rev ...int64) (int64, error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	var toRev int64
	if len(rev) == 0 {
		resp, err := kv.Get(ctx, "\x00")
		if err != nil {
			log.Error("etcd error: ", err)
			return 0, err
		}
		toRev = resp.Header.Revision
	} else {
		toRev = rev[0]
	}

	log.Debugf("compacting ETCD to revision %v", toRev)
	t := time.Now()
	if _, err := kv.Compact(ctx, toRev, clientv3.WithCompactPhysical()); err != nil {
		log.Error("etcd compact error: ", err)
		return 0, err
	}
	log.Debugf("compacting ETCD took %v", time.Since(t))

	return toRev, nil
}

// GetRevision returns current revision of ETCD database
func (db *BytesConnectionEtcd) GetRevision() (revision int64, err error) {
	return getRevisionInternal(db.Logger, db.etcdClient, db.opTimeout)
}

func getRevisionInternal(log logging.Logger, kv clientv3.KV, opTimeout time.Duration) (revision int64, err error) {
	deadline := time.Now().Add(opTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	resp, err := kv.Get(ctx, "\x00")
	if err != nil {
		log.Error("etcd error: ", err)
		return 0, err
	}

	return resp.Header.Revision, nil
}
