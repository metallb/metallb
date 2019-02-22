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

package cryptodata

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
)

// KvProtoPluginWrapper wraps keyval.KvProtoPlugin with additional support of reading encrypted data
type KvProtoPluginWrapper struct {
	keyval.KvProtoPlugin
	decryptData
}

// ProtoBrokerWrapper wraps keyval.ProtoBroker with additional support of reading encrypted data
type ProtoBrokerWrapper struct {
	keyval.ProtoBroker
	decryptData
}

// ProtoWatcherWrapper wraps keyval.ProtoWatcher with additional support of reading encrypted data
type ProtoWatcherWrapper struct {
	keyval.ProtoWatcher
	decryptData
}

// ProtoKeyValWrapper wraps keyval.ProtoKeyVal with additional support of reading encrypted data
type ProtoKeyValWrapper struct {
	keyval.ProtoKeyVal
	decryptData
}

// ProtoWatchRespWrapper wraps keyval.ProtoWatchResp with additional support of reading encrypted data
type ProtoWatchRespWrapper struct {
	datasync.ProtoWatchResp
	ProtoKeyValWrapper
}

// ProtoKeyValIteratorWrapper wraps keyval.ProtoKeyValIterator with additional support of reading encrypted data
type ProtoKeyValIteratorWrapper struct {
	keyval.ProtoKeyValIterator
	decryptData
}

// NewKvProtoPluginWrapper creates wrapper for provided KvProtoPlugin, adding support for decrypting encrypted data
func NewKvProtoPluginWrapper(kvp keyval.KvProtoPlugin, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *KvProtoPluginWrapper {
	return &KvProtoPluginWrapper{
		KvProtoPlugin: kvp,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewProtoBrokerWrapper creates wrapper for provided ProtoBroker, adding support for decrypting encrypted data
func NewProtoBrokerWrapper(pb keyval.ProtoBroker, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *ProtoBrokerWrapper {
	return &ProtoBrokerWrapper{
		ProtoBroker: pb,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewProtoWatcherWrapper creates wrapper for provided ProtoWatcher, adding support for decrypting encrypted data
func NewProtoWatcherWrapper(pb keyval.ProtoWatcher, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *ProtoWatcherWrapper {
	return &ProtoWatcherWrapper{
		ProtoWatcher: pb,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewBroker returns a ProtoBroker instance with support for decrypting values that prepends given <keyPrefix> to all
// keys in its calls.
// To avoid using a prefix, pass keyval.Root constant as argument.
func (kvp *KvProtoPluginWrapper) NewBroker(prefix string) keyval.ProtoBroker {
	return NewProtoBrokerWrapper(kvp.KvProtoPlugin.NewBroker(prefix), kvp.decrypter, kvp.decryptFunc)
}

// NewWatcher returns a ProtoWatcher instance with support for decrypting values that prepends given <keyPrefix> to all
// keys during watch subscribe phase.
// The prefix is removed from the key retrieved by GetKey() in ProtoWatchResp.
// To avoid using a prefix, pass keyval.Root constant as argument.
func (kvp *KvProtoPluginWrapper) NewWatcher(prefix string) keyval.ProtoWatcher {
	return NewProtoWatcherWrapper(kvp.KvProtoPlugin.NewWatcher(prefix), kvp.decrypter, kvp.decryptFunc)
}

// GetValue retrieves one item under the provided <key>. If the item exists,
// it is unmarshaled into the <reqObj> and its fields are decrypted.
func (db *ProtoBrokerWrapper) GetValue(key string, reqObj proto.Message) (bool, int64, error) {
	found, revision, err := db.ProtoBroker.GetValue(key, reqObj)
	if !found || err != nil {
		return found, revision, err
	}

	_, err = db.decrypter.Decrypt(reqObj, db.decryptFunc)
	return found, revision, err
}

// ListValues returns an iterator that enables to traverse all items stored
// under the provided <key>.
func (db *ProtoBrokerWrapper) ListValues(key string) (keyval.ProtoKeyValIterator, error) {
	kv, err := db.ProtoBroker.ListValues(key)
	if err != nil {
		return kv, err
	}
	return &ProtoKeyValIteratorWrapper{
		ProtoKeyValIterator: kv,
		decryptData:         db.decryptData,
	}, nil
}

// Watch starts subscription for changes associated with the selected keys.
// Watch events will be delivered to callback (not channel) <respChan>.
// Channel <closeChan> can be used to close watching on respective key
func (b *ProtoWatcherWrapper) Watch(respChan func(datasync.ProtoWatchResp), closeChan chan string, keys ...string) error {
	return b.ProtoWatcher.Watch(func(resp datasync.ProtoWatchResp) {
		respChan(&ProtoWatchRespWrapper{
			ProtoWatchResp: resp,
			ProtoKeyValWrapper: ProtoKeyValWrapper{
				ProtoKeyVal: resp,
				decryptData: b.decryptData,
			},
		})
	}, closeChan, keys...)
}

// GetValue returns the value of the pair.
func (r *ProtoWatchRespWrapper) GetValue(value proto.Message) error {
	return r.ProtoKeyValWrapper.GetValue(value)
}

// GetPrevValue returns the previous value of the pair.
func (r *ProtoWatchRespWrapper) GetPrevValue(prevValue proto.Message) (prevValueExist bool, err error) {
	return r.ProtoKeyValWrapper.GetPrevValue(prevValue)
}

// GetValue returns the value of the pair.
func (r *ProtoKeyValWrapper) GetValue(value proto.Message) error {
	err := r.ProtoKeyVal.GetValue(value)
	if err != nil {
		return err
	}
	_, err = r.decrypter.Decrypt(value, r.decryptFunc)
	return err
}

// GetPrevValue returns the previous value of the pair.
func (r *ProtoKeyValWrapper) GetPrevValue(prevValue proto.Message) (prevValueExist bool, err error) {
	exists, err := r.ProtoKeyVal.GetPrevValue(prevValue)
	if !exists || err != nil {
		return exists, err
	}
	_, err = r.decrypter.Decrypt(prevValue, r.decryptFunc)
	return exists, err
}

// GetNext retrieves the following item from the context.
// When there are no more items to get, <stop> is returned as *true*
// and <kv> is simply *nil*.
func (r *ProtoKeyValIteratorWrapper) GetNext() (kv keyval.ProtoKeyVal, stop bool) {
	kv, stop = r.ProtoKeyValIterator.GetNext()
	if stop || kv == nil {
		return kv, stop
	}
	return &ProtoKeyValWrapper{
		ProtoKeyVal: kv,
		decryptData: r.decryptData,
	}, stop
}
