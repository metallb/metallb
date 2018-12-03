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
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
)

// ProtoWrapper is a decorator which allows to read/write proto file modelled
// data. It marshals/unmarshals go structures to slice of bytes and vice versa
// behind the scenes.
type ProtoWrapper struct {
	broker     keyval.CoreBrokerWatcher
	serializer keyval.Serializer
}

type protoBroker struct {
	broker     keyval.BytesBroker
	serializer keyval.Serializer
}

// protoKeyValIterator is an iterator returned by ListValues call.
type protoKeyValIterator struct {
	delegate   keyval.BytesKeyValIterator
	serializer keyval.Serializer
}

// protoKeyIterator is an iterator returned by ListKeys call.
type protoKeyIterator struct {
	delegate keyval.BytesKeyIterator
}

// protoKeyVal represents single key-value pair.
type protoKeyVal struct {
	pair       keyval.BytesKeyVal
	serializer keyval.Serializer
}

// NewProtoWrapper initializes proto decorator.
// The default serializer is used - SerializerProto.
func NewProtoWrapper(db keyval.CoreBrokerWatcher, serializer ...keyval.Serializer) *ProtoWrapper {
	if len(serializer) > 0 {
		return &ProtoWrapper{db, serializer[0]}
	}
	return &ProtoWrapper{db, &keyval.SerializerProto{}}
}

// NewProtoWrapperWithSerializer initializes proto decorator with the specified
// serializer.
func NewProtoWrapperWithSerializer(db keyval.CoreBrokerWatcher, serializer keyval.Serializer) *ProtoWrapper {
	// OBSOLETE, use NewProtoWrapper
	return NewProtoWrapper(db, serializer)
}

// Close closes underlying connection to ETCD.
// Beware: if the connection is shared among multiple instances, this might
// unintentionally cancel the connection for them.
func (db *ProtoWrapper) Close() error {
	return db.broker.Close()
}

// NewBroker creates a new instance of the proxy that shares the underlying
// connection and allows to read/edit key-value pairs.
func (db *ProtoWrapper) NewBroker(prefix string) keyval.ProtoBroker {
	return &protoBroker{db.broker.NewBroker(prefix), db.serializer}
}

// NewWatcher creates a new instance of the proxy that shares the underlying
// connection and allows subscribing for watching of the changes.
func (db *ProtoWrapper) NewWatcher(prefix string) keyval.ProtoWatcher {
	return &protoWatcher{db.broker.NewWatcher(prefix), db.serializer}
}

// NewTxn creates a new Data Broker transaction. A transaction can
// hold multiple operations that are all committed to the data
// store together. After a transaction has been created, one or
// more operations (put or delete) can be added to the transaction
// before it is committed.
func (db *ProtoWrapper) NewTxn() keyval.ProtoTxn {
	return &protoTxn{txn: db.broker.NewTxn(), serializer: db.serializer}
}

// NewTxn creates a new Data Broker transaction. A transaction can
// hold multiple operations that are all committed to the data
// store together. After a transaction has been created, one or
// more operations (put or delete) can be added to the transaction
// before it is committed.
func (pdb *protoBroker) NewTxn() keyval.ProtoTxn {
	return &protoTxn{txn: pdb.broker.NewTxn(), serializer: pdb.serializer}
}

// Put writes the provided key-value item into the data store.
// It returns an error if the item could not be written, nil otherwise.
func (db *ProtoWrapper) Put(key string, value proto.Message, opts ...datasync.PutOption) error {
	return putProtoInternal(db.broker, db.serializer, key, value, opts...)
}

// Put writes the provided key-value item into the data store.
// It returns an error if the item could not be written, nil otherwise.
func (pdb *protoBroker) Put(key string, value proto.Message, opts ...datasync.PutOption) error {
	return putProtoInternal(pdb.broker, pdb.serializer, key, value, opts...)
}

func putProtoInternal(broker keyval.BytesBroker, serializer keyval.Serializer, key string, value proto.Message,
	opts ...datasync.PutOption) error {

	// Marshal value to protobuf.
	binData, err := serializer.Marshal(value)
	if err != nil {
		return err
	}

	return broker.Put(key, binData, opts...)
}

// Delete removes key-value items stored under <key> from datastore.
func (db *ProtoWrapper) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return db.broker.Delete(key, opts...)
}

// Delete removes key-value items stored under <key> from datastore.
func (pdb *protoBroker) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return pdb.broker.Delete(key, opts...)
}

// Watch subscribes for changes in datastore associated with any of the <keys>.
// Callback <resp> is used for delivery of watch events.
// Channel <closeChan> is used to close key-related goroutines
// Any encountered error is returned as well
func (db *ProtoWrapper) Watch(resp func(keyval.ProtoWatchResp), closeChan chan string, keys ...string) error {
	return db.broker.Watch(func(msg keyval.BytesWatchResp) {
		resp(NewWatchResp(db.serializer, msg))
	}, closeChan, keys...)
}

// GetValue retrieves one key-value item from the datastore. The item
// is identified by the provided <key>.
//
// If the item was found, its value is unmarshaled and placed in
// the <reqObj> message buffer and the function returns <found> as *true*.
// If the object was not found, the function returns <found> as *false*.
// Function also returns the revision of the latest modification.
// Any encountered error is returned in <err>.
func (db *ProtoWrapper) GetValue(key string, reqObj proto.Message) (found bool, revision int64, err error) {
	return getValueProtoInternal(db.broker, db.serializer, key, reqObj)
}

// GetValue retrieves one key-value item from the datastore. The item
// is identified by the provided <key>.
//
// If the item was found, its value is unmarshaled and placed in
// the <reqObj> message buffer and the function returns <found> as *true*.
// If the object was not found, the function returns <found> as *false*.
// Function also returns the revision of the latest modification.
// Any encountered error is returned in <err>.
func (pdb *protoBroker) GetValue(key string, reqObj proto.Message) (found bool, revision int64, err error) {
	return getValueProtoInternal(pdb.broker, pdb.serializer, key, reqObj)
}

func getValueProtoInternal(broker keyval.BytesBroker, serializer keyval.Serializer, key string, reqObj proto.Message) (found bool, revision int64, err error) {
	// get data from etcd
	resp, found, rev, err := broker.GetValue(key)
	if err != nil {
		return false, 0, err
	}

	if !found {
		return false, 0, nil
	}

	err = serializer.Unmarshal(resp, reqObj)
	if err != nil {
		return false, 0, err
	}
	return true, rev, nil
}

// ListValues retrieves an iterator for elements stored under the provided <key>.
func (db *ProtoWrapper) ListValues(key string) (keyval.ProtoKeyValIterator, error) {
	return listValuesProtoInternal(db.broker, db.serializer, key)
}

// ListValues retrieves an iterator for elements stored under the provided <key>.
func (pdb *protoBroker) ListValues(key string) (keyval.ProtoKeyValIterator, error) {
	return listValuesProtoInternal(pdb.broker, pdb.serializer, key)
}

func listValuesProtoInternal(broker keyval.BytesBroker, serializer keyval.Serializer, key string) (keyval.ProtoKeyValIterator, error) {
	ctx, err := broker.ListValues(key)
	if err != nil {
		return nil, err
	}
	return &protoKeyValIterator{ctx, serializer}, nil
}

// ListKeys returns an iterator that allows to traverse all keys that share the given <prefix>
// from data store.
func (db *ProtoWrapper) ListKeys(prefix string) (keyval.ProtoKeyIterator, error) {
	return listKeysProtoInternal(db.broker, prefix)
}

// ListKeys returns an iterator that allows to traverse all keys that share the given <prefix>
// from data store.
func (pdb *protoBroker) ListKeys(prefix string) (keyval.ProtoKeyIterator, error) {
	return listKeysProtoInternal(pdb.broker, prefix)
}

func listKeysProtoInternal(broker keyval.BytesBroker, prefix string) (keyval.ProtoKeyIterator, error) {
	ctx, err := broker.ListKeys(prefix)
	if err != nil {
		return nil, err
	}
	return &protoKeyIterator{ctx}, nil
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (ctx *protoKeyValIterator) Close() error {
	return nil
}

// GetNext returns the following item from the result set.
// When there are no more items to get, <stop> is returned as *true*
// and <kv> is simply *nil*.
func (ctx *protoKeyValIterator) GetNext() (kv keyval.ProtoKeyVal, stop bool) {
	pair, stop := ctx.delegate.GetNext()
	if stop {
		return nil, stop
	}

	return &protoKeyVal{pair, ctx.serializer}, stop
}

// Close does nothing since db cursors are not needed.
// The method is required in the code since it implements Iterator API.
func (ctx *protoKeyIterator) Close() error {
	return nil
}

// GetNext returns the following key from the result set.
// When there are no more keys to get, <stop> is returned as *true*
// and <key> and <rev> are default values.
func (ctx *protoKeyIterator) GetNext() (key string, rev int64, stop bool) {
	return ctx.delegate.GetNext()
}

// GetValue returns the value of the pair.
func (kv *protoKeyVal) GetValue(msg proto.Message) error {
	err := kv.serializer.Unmarshal(kv.pair.GetValue(), msg)
	if err != nil {
		return err
	}
	return nil
}

// GetPrevValue returns the previous value of the pair.
func (kv *protoKeyVal) GetPrevValue(msg proto.Message) (prevValueExist bool, err error) {
	prevVal := kv.pair.GetPrevValue()
	if prevVal == nil {
		return false, nil
	}
	err = kv.serializer.Unmarshal(prevVal, msg)
	if err != nil {
		return true, err
	}
	return true, nil
}

// GetKey returns the key of the pair.
func (kv *protoKeyVal) GetKey() string {
	return kv.pair.GetKey()
}

// GetRevision returns the revision associated with the pair.
func (kv *protoKeyVal) GetRevision() int64 {
	return kv.pair.GetRevision()
}
