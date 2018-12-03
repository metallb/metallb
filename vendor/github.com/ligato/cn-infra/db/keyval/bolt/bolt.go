//  Copyright (c) 2018 Cisco and/or its affiliates.
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

package bolt

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/boltdb/bolt"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
)

var boltLogger = logrus.NewLogger("bolt")

func init() {
	if os.Getenv("DEBUG_BOLT_CLIENT") != "" {
		boltLogger.SetLevel(logging.DebugLevel)
	}
}

var rootBucket = []byte("root")

// Client serves as a client for Bolt KV storage and implements
// keyval.CoreBrokerWatcher interface.
type Client struct {
	db *bolt.DB
}

// NewClient creates new client for Bolt using given config.
func NewClient(cfg *Config) (client *Client, err error) {
	db, err := bolt.Open(cfg.DbPath, cfg.FileMode, &bolt.Options{
		Timeout: cfg.LockTimeout,
	})
	if err != nil {
		return nil, err
	}
	boltLogger.Infof("bolt path: %v", db.Path())

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(rootBucket)
		return err
	})

	return &Client{db: db}, nil
}

// NewTxn creates new transaction
func (client *Client) NewTxn() keyval.BytesTxn {
	return &txn{
		db: client.db,
	}
}

// Put stores given data for the key
func (client *Client) Put(key string, data []byte, opts ...datasync.PutOption) error {
	boltLogger.Debugf("Put: %q (len=%d)", key, len(data))

	return client.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(rootBucket).Put([]byte(key), data)
	})
}

// GetValue returns data for the given key
func (client *Client) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	boltLogger.Debugf("GetValue: %q", key)

	err = client.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(rootBucket).Get([]byte(key))
		if value == nil {
			return fmt.Errorf("value for key %q not found in bucket", key)
		}

		found = true
		data = append([]byte(nil), value...) // value needs to be copied

		return nil
	})
	return data, found, 0, err
}

// Delete deletes given key
func (client *Client) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	boltLogger.Debugf("Delete: %q", key)

	err = client.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(rootBucket)

		if data := bucket.Get([]byte(key)); data != nil {
			existed = true
			return bucket.Delete([]byte(key))
		}

		return fmt.Errorf("key %q not found in bucket", key)
	})

	return existed, err
}

// ListKeys returns iterator with keys for given key prefix
func (client *Client) ListKeys(keyPrefix string) (keyval.BytesKeyIterator, error) {
	boltLogger.Debugf("ListKeys: %q", keyPrefix)

	var keys []string
	err := client.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(rootBucket).Cursor()
		prefix := []byte(keyPrefix)

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			boltLogger.Debugf(" listing key: %q", string(k))
			keys = append(keys, string(k))
		}

		return nil
	})

	return &bytesKeyIterator{len: len(keys), keys: keys}, err
}

// ListValues returns iterator with key-value pairs for given key prefix
func (client *Client) ListValues(keyPrefix string) (keyval.BytesKeyValIterator, error) {
	boltLogger.Debugf("ListValues: %q", keyPrefix)

	var pairs []*kvPair
	err := client.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(rootBucket).Cursor()
		prefix := []byte(keyPrefix)

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			boltLogger.Debugf(" listing val: %q (len=%d)", string(k), len(v))

			pair := &kvPair{Key: string(k)}
			pair.Value = append([]byte(nil), v...) // value needs to be copied

			pairs = append(pairs, pair)
		}

		return nil
	})

	return &bytesKeyValIterator{len: len(pairs), pairs: pairs}, err
}

// Close closes Bolt database.
func (client *Client) Close() error {
	return client.db.Close()
}

// Watch watches given list of key prefixes.
func (client *Client) Watch(resp func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	return errors.New("not implemented")
}

// NewBroker creates a new instance of a proxy that provides
// access to Bolt. The proxy will reuse the connection from Client.
// <prefix> will be prepended to the key argument in all calls from the created
// BrokerWatcher. To avoid using a prefix, pass keyval. Root constant as
// an argument.
func (client *Client) NewBroker(prefix string) keyval.BytesBroker {
	return &BrokerWatcher{
		Client: client,
		prefix: prefix,
	}
}

// NewWatcher creates a new instance of a proxy that provides
// access to Bolt. The proxy will reuse the connection from Client.
// <prefix> will be prepended to the key argument in all calls on created
// BrokerWatcher. To avoid using a prefix, pass keyval. Root constant as
// an argument.
func (client *Client) NewWatcher(prefix string) keyval.BytesWatcher {
	return &BrokerWatcher{
		Client: client,
		prefix: prefix,
	}
}

// BrokerWatcher uses Client to access the datastore.
// The connection can be shared among multiple BrokerWatcher.
// In case of accessing a particular subtree in Bolt only,
// BrokerWatcher allows defining a keyPrefix that is prepended
// to all keys in its methods in order to shorten keys used in arguments.
type BrokerWatcher struct {
	*Client
	prefix string
}

func (pdb *BrokerWatcher) prefixKey(key string) string {
	return pdb.prefix + key
}

// Put calls 'Put' function of the underlying Client.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BrokerWatcher) Put(key string, data []byte, opts ...datasync.PutOption) error {
	return pdb.Client.Put(pdb.prefixKey(key), data, opts...)
}

// NewTxn creates a new transaction.
// KeyPrefix defined in constructor will be prepended to all key arguments
// in the transaction.
func (pdb *BrokerWatcher) NewTxn() keyval.BytesTxn {
	return pdb.Client.NewTxn()
}

// GetValue calls 'GetValue' function of the underlying Client.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BrokerWatcher) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	return pdb.Client.GetValue(pdb.prefixKey(key))
}

// Delete calls 'Delete' function of the underlying Client.
// KeyPrefix defined in constructor is prepended to the key argument.
func (pdb *BrokerWatcher) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return pdb.Client.Delete(pdb.prefixKey(key), opts...)
}

// ListKeys calls 'ListKeys' function of the underlying Client.
// KeyPrefix defined in constructor is prepended to the argument.
func (pdb *BrokerWatcher) ListKeys(keyPrefix string) (keyval.BytesKeyIterator, error) {
	boltLogger.Debugf("ListKeys: %q [namespace=%s]", keyPrefix, pdb.prefix)

	var keys []string
	err := pdb.Client.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(rootBucket).Cursor()
		prefix := []byte(pdb.prefixKey(keyPrefix))
		boltLogger.Debugf("listing keys: %q", string(prefix))

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			boltLogger.Debugf(" listing key: %q", string(k))
			keys = append(keys, string(k))
		}
		return nil
	})

	return &bytesKeyIterator{prefix: pdb.prefix, len: len(keys), keys: keys}, err
}

// ListValues calls 'ListValues' function of the underlying Client.
// KeyPrefix defined in constructor is prepended to the key argument.
// The prefix is removed from the keys of the returned values.
func (pdb *BrokerWatcher) ListValues(keyPrefix string) (keyval.BytesKeyValIterator, error) {
	boltLogger.Debugf("ListValues: %q [namespace=%s]", keyPrefix, pdb.prefix)

	var pairs []*kvPair
	err := pdb.Client.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(rootBucket).Cursor()
		prefix := []byte(pdb.prefixKey(keyPrefix))
		boltLogger.Debugf("listing vals: %q", string(prefix))

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			boltLogger.Debugf(" listing val: %q (len=%d)", string(k), len(v))

			pair := &kvPair{Key: string(k)}
			pair.Value = append([]byte(nil), v...) // value needs to be copied

			pairs = append(pairs, pair)
		}
		return nil
	})

	return &bytesKeyValIterator{prefix: pdb.prefix, pairs: pairs, len: len(pairs)}, err
}

// Watch starts subscription for changes associated with the selected <keys>.
// KeyPrefix defined in constructor is prepended to all <keys> in the argument
// list. The prefix is removed from the keys returned in watch events.
// Watch events will be delivered to <resp> callback.
func (pdb *BrokerWatcher) Watch(resp func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	return errors.New("not implemented")
}
