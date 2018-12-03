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

import "github.com/ligato/cn-infra/db/keyval"

// KvBytesPluginWrapper wraps keyval.KvBytesPlugin with additional support of reading encrypted data
type KvBytesPluginWrapper struct {
	keyval.KvBytesPlugin
	decryptData
}

// BytesBrokerWrapper wraps keyval.BytesBroker with additional support of reading encrypted data
type BytesBrokerWrapper struct {
	keyval.BytesBroker
	decryptData
}

// BytesWatcherWrapper wraps keyval.BytesWatcher with additional support of reading encrypted data
type BytesWatcherWrapper struct {
	keyval.BytesWatcher
	decryptData
}

// BytesKeyValWrapper wraps keyval.BytesKeyVal with additional support of reading encrypted data
type BytesKeyValWrapper struct {
	keyval.BytesKeyVal
	decryptData
}

// BytesWatchRespWrapper wraps keyval.BytesWatchResp with additional support of reading encrypted data
type BytesWatchRespWrapper struct {
	keyval.BytesWatchResp
	BytesKeyValWrapper
}

// BytesKeyValIteratorWrapper wraps keyval.BytesKeyValIterator with additional support of reading encrypted data
type BytesKeyValIteratorWrapper struct {
	keyval.BytesKeyValIterator
	decryptData
}

// NewKvBytesPluginWrapper creates wrapper for provided CoreBrokerWatcher, adding support for decrypting encrypted
// data
func NewKvBytesPluginWrapper(cbw keyval.KvBytesPlugin, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *KvBytesPluginWrapper {
	return &KvBytesPluginWrapper{
		KvBytesPlugin: cbw,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewBytesBrokerWrapper creates wrapper for provided BytesBroker, adding support for decrypting encrypted data
func NewBytesBrokerWrapper(pb keyval.BytesBroker, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *BytesBrokerWrapper {
	return &BytesBrokerWrapper{
		BytesBroker: pb,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewBytesWatcherWrapper creates wrapper for provided BytesWatcher, adding support for decrypting encrypted data
func NewBytesWatcherWrapper(pb keyval.BytesWatcher, decrypter ArbitraryDecrypter, decryptFunc DecryptFunc) *BytesWatcherWrapper {
	return &BytesWatcherWrapper{
		BytesWatcher: pb,
		decryptData: decryptData{
			decryptFunc: decryptFunc,
			decrypter:   decrypter,
		},
	}
}

// NewBroker returns a BytesBroker instance with support for decrypting values that prepends given <keyPrefix> to all
// keys in its calls.
// To avoid using a prefix, pass keyval.Root constant as argument.
func (cbw *KvBytesPluginWrapper) NewBroker(prefix string) keyval.BytesBroker {
	return NewBytesBrokerWrapper(cbw.KvBytesPlugin.NewBroker(prefix), cbw.decrypter, cbw.decryptFunc)
}

// NewWatcher returns a BytesWatcher instance with support for decrypting values that prepends given <keyPrefix> to all
// keys during watch subscribe phase.
// The prefix is removed from the key retrieved by GetKey() in BytesWatchResp.
// To avoid using a prefix, pass keyval.Root constant as argument.
func (cbw *KvBytesPluginWrapper) NewWatcher(prefix string) keyval.BytesWatcher {
	return NewBytesWatcherWrapper(cbw.KvBytesPlugin.NewWatcher(prefix), cbw.decrypter, cbw.decryptFunc)
}

// GetValue retrieves and tries to decrypt one item under the provided key.
func (cbb *BytesBrokerWrapper) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	data, found, revision, err = cbb.BytesBroker.GetValue(key)
	if err == nil {
		objData, err := cbb.decrypter.Decrypt(data, cbb.decryptFunc)
		if err != nil {
			return data, found, revision, err
		}
		outData, ok := objData.([]byte)
		if !ok {
			return data, found, revision, err
		}
		return outData, found, revision, err
	}
	return
}

// ListValues returns an iterator that enables to traverse all items stored
// under the provided <key>.
func (cbb *BytesBrokerWrapper) ListValues(key string) (keyval.BytesKeyValIterator, error) {
	kv, err := cbb.BytesBroker.ListValues(key)
	if err != nil {
		return kv, err
	}
	return &BytesKeyValIteratorWrapper{
		BytesKeyValIterator: kv,
		decryptData:         cbb.decryptData,
	}, nil
}

// Watch starts subscription for changes associated with the selected keys.
// Watch events will be delivered to callback (not channel) <respChan>.
// Channel <closeChan> can be used to close watching on respective key
func (b *BytesWatcherWrapper) Watch(respChan func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	return b.BytesWatcher.Watch(func(resp keyval.BytesWatchResp) {
		respChan(&BytesWatchRespWrapper{
			BytesWatchResp: resp,
			BytesKeyValWrapper: BytesKeyValWrapper{
				BytesKeyVal: resp,
				decryptData: b.decryptData,
			},
		})
	}, closeChan, keys...)
}

// GetValue returns the value of the pair.
func (r *BytesWatchRespWrapper) GetValue() []byte {
	return r.BytesKeyValWrapper.GetValue()
}

// GetPrevValue returns the previous value of the pair.
func (r *BytesWatchRespWrapper) GetPrevValue() []byte {
	return r.BytesKeyValWrapper.GetPrevValue()
}

// GetValue returns the value of the pair.
func (r *BytesKeyValWrapper) GetValue() []byte {
	value := r.BytesKeyVal.GetValue()
	data, err := r.decrypter.Decrypt(value, r.decryptFunc)
	if err != nil {
		return nil
	}
	outData, ok := data.([]byte)
	if !ok {
		return value
	}
	return outData
}

// GetPrevValue returns the previous value of the pair.
func (r *BytesKeyValWrapper) GetPrevValue() []byte {
	value := r.BytesKeyVal.GetPrevValue()
	data, err := r.decrypter.Decrypt(value, r.decryptFunc)
	if err != nil {
		return nil
	}
	outData, ok := data.([]byte)
	if !ok {
		return value
	}
	return outData
}

// GetNext retrieves the following item from the context.
// When there are no more items to get, <stop> is returned as *true*
// and <kv> is simply *nil*.
func (r *BytesKeyValIteratorWrapper) GetNext() (kv keyval.BytesKeyVal, stop bool) {
	kv, stop = r.BytesKeyValIterator.GetNext()
	if stop || kv == nil {
		return kv, stop
	}
	return &BytesKeyValWrapper{
		BytesKeyVal: kv,
		decryptData: r.decryptData,
	}, stop
}
