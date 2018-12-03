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

package redis

import (
	"fmt"
	"strings"
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
)

// BytesConnectionRedis allows to store, read and watch values from Redis.
type BytesConnectionRedis struct {
	logging.Logger
	client Client

	// closeCh will be closed when this connection is closed, i.e. by the Close() method.
	// It is used to give go routines a signal to stop.
	closeCh chan string

	// Flag to indicate whether this connection is closed.
	closed bool
}

// bytesKeyIterator is an iterator returned by ListKeys call.
type bytesKeyIterator struct {
	index      int
	keys       []string
	db         *BytesConnectionRedis
	pattern    string
	cursor     uint64
	trimPrefix func(key string) string
	err        error
}

// bytesKeyValIterator is an iterator returned by ListValues call.
type bytesKeyValIterator struct {
	values [][]byte
	bytesKeyIterator
}

// bytesKeyVal represents a single key-value pair.
type bytesKeyVal struct {
	key       string
	value     []byte
	prevValue []byte
}

// NewBytesConnection creates a new instance of BytesConnectionRedis using the provided
// Client (be it node, or cluster, or sentinel client).
func NewBytesConnection(client Client, log logging.Logger) (*BytesConnectionRedis, error) {
	return &BytesConnectionRedis{log, client, make(chan string), false}, nil
}

// Close closes the connection to redis.
func (db *BytesConnectionRedis) Close() error {
	if db.closed {
		db.Debug("Close() called on a closed connection")
		return nil
	}
	db.Debug("Close()")
	db.closed = true
	safeclose.Close(db.closeCh)
	if db.client != nil {
		err := safeclose.Close(db.client)
		if err != nil {
			return fmt.Errorf("Close() encountered error: %s", err)
		}
	}
	return nil
}

// NewTxn creates new transaction.
func (db *BytesConnectionRedis) NewTxn() keyval.BytesTxn {
	if db.closed {
		db.Error("NewTxn() called on a closed connection")
		return nil
	}
	db.Debug("NewTxn()")

	return &Txn{db: db, ops: []op{}, addPrefix: nil}
}

// Put sets the key/value in Redis data store. Replaces value if the key already exists.
func (db *BytesConnectionRedis) Put(key string, data []byte, opts ...datasync.PutOption) error {
	if db.closed {
		return fmt.Errorf("Put(%s) called on a closed connection", key)
	}
	db.Debugf("Put(%s)", key)

	var ttl time.Duration
	for _, o := range opts {
		if withTTL, ok := o.(*datasync.WithTTLOpt); ok && withTTL.TTL > 0 {
			ttl = withTTL.TTL
		}
	}
	err := db.client.Set(key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("Set(%s) failed: %s", key, err)
	}
	return nil
}

// GetValue retrieves the value of the key from Redis.
func (db *BytesConnectionRedis) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	if db.closed {
		return nil, false, 0, fmt.Errorf("GetValue(%s) called on a closed connection", key)
	}
	db.Debugf("GetValue(%s)", key)

	statusCmd := db.client.Get(key)
	data, err = statusCmd.Bytes()
	if err != nil {
		if err == GoRedisNil {
			return data, false, 0, nil
		}
		return nil, false, 0, fmt.Errorf("Get(%s) failed: %s", key, err)
	}
	return data, true, 0, nil
}

// ListKeys returns an iterator used to traverse keys that start with the given match string.
// When done traversing, you must close the iterator by calling its Close() method.
func (db *BytesConnectionRedis) ListKeys(match string) (keyval.BytesKeyIterator, error) {
	if db.closed {
		return nil, fmt.Errorf("ListKeys(%s) called on a closed connection", match)
	}
	return listKeys(db, match, nil, nil)
}

// ListValues returns an iterator used to traverse key value pairs for all the keys that start with the given match string.
// When done traversing, you must close the iterator by calling its Close() method.
func (db *BytesConnectionRedis) ListValues(match string) (keyval.BytesKeyValIterator, error) {
	if db.closed {
		return nil, fmt.Errorf("ListValues(%s) called on a closed connection", match)
	}
	return listValues(db, match, nil, nil)
}

// Delete deletes all the keys that start with the given match string.
func (db *BytesConnectionRedis) Delete(key string, opts ...datasync.DelOption) (found bool, err error) {
	if db.closed {
		return false, fmt.Errorf("Delete(%s) called on a closed connection", key)
	}
	db.Debugf("Delete(%s)", key)

	keysToDelete := []string{}

	var keyIsPrefix bool
	for _, o := range opts {
		if _, ok := o.(*datasync.WithPrefixOpt); ok {
			keyIsPrefix = true
		}
	}
	if keyIsPrefix {
		iterator, err := db.ListKeys(key)
		if err != nil {
			return false, err
		}
		for {
			k, _, last := iterator.GetNext()
			if last {
				break
			}
			keysToDelete = append(keysToDelete, k)
		}
		if len(keysToDelete) == 0 {
			return false, nil
		}
		db.Debugf("Delete(%s): deleting %v", key, keysToDelete)
	} else {
		keysToDelete = append(keysToDelete, key)
	}

	intCmd := db.client.Del(keysToDelete...)
	if intCmd.Err() != nil {
		return false, fmt.Errorf("Delete(%s) failed: %s", key, intCmd.Err())
	}
	return (intCmd.Val() != 0), nil
}

// Close closes the iterator. It returns either an error (if any occurs), or nil.
func (it *bytesKeyIterator) Close() error {
	return it.err
}

// GetNext returns the next item from the iterator.
// If the iterator encounters an error or has reached the last item previously, lastReceived is set to true.
func (it *bytesKeyIterator) GetNext() (key string, rev int64, lastReceived bool) {
	if it.err != nil {
		return "", 0, true
	}
	if it.index >= len(it.keys) {
		if it.cursor == 0 {
			return "", 0, true
		}
		var err error
		it.keys, it.cursor, err = scanKeys(it.db, it.pattern, it.cursor)
		if err != nil {
			it.err = err
			it.db.Errorf("GetNext() failed: %s (pattern %s)", err.Error(), it.pattern)
			return "", 0, true
		}
		if len(it.keys) == 0 {
			return "", 0, it.cursor == 0
		}
		it.index = 0
	}

	key = it.keys[it.index]
	if it.trimPrefix != nil {
		key = it.trimPrefix(key)
	}
	it.index++

	return key, 0, false
}

// Close closes the iterator. It returns either an error (if it occurs), or nil.
func (it *bytesKeyValIterator) Close() error {
	return it.err
}

// GetNext returns the next item from the iterator.
// If the iterator encounters an error or has reached the last item previously, lastReceived is set to true.
func (it *bytesKeyValIterator) GetNext() (kv keyval.BytesKeyVal, lastReceived bool) {
	if it.err != nil {
		return nil, true
	}
	if it.index >= len(it.values) {
		if it.cursor == 0 {
			return nil, true
		}
		var err error
		it.keys, it.cursor, err = scanKeys(it.db, it.pattern, it.cursor)
		if err != nil {
			it.err = err
			it.db.Errorf("GetNext() failed: %s (pattern %s)", err.Error(), it.pattern)
			return nil, true
		}
		if len(it.keys) == 0 {
			return nil, it.cursor == 0
		}
		it.values, err = getValues(it.db, it.keys)
		if err != nil {
			it.err = err
			it.db.Errorf("GetNext() failed: %s (pattern %s)", err.Error(), it.pattern)
			return nil, true
		}
		it.index = 0
	}

	key := it.keys[it.index]
	if it.trimPrefix != nil {
		key = it.trimPrefix(key)
	}

	value := it.values[it.index]
	var prevValue []byte
	if it.index > 0 {
		prevValue = it.values[it.index-1]
	}

	kv = &bytesKeyVal{key, value, prevValue}
	it.index++

	return kv, false
}

// GetValue returns the value of the pair.
func (kv *bytesKeyVal) GetValue() []byte {
	return kv.value
}

// GetPrevValue returns the previous value of the pair.
func (kv *bytesKeyVal) GetPrevValue() []byte {
	return kv.prevValue
}

// GetKey returns the key of the pair.
func (kv *bytesKeyVal) GetKey() string {
	return kv.key
}

// GetRevision returns the revision associated with the pair.
func (kv *bytesKeyVal) GetRevision() int64 {
	return 0
}

func listKeys(db *BytesConnectionRedis, match string,
	addPrefix func(key string) string, trimPrefix func(key string) string) (keyval.BytesKeyIterator, error) {
	pattern := match
	if addPrefix != nil {
		pattern = addPrefix(pattern)
	}
	pattern = wildcard(pattern)
	db.Debugf("listKeys(%s): pattern %s", match, pattern)

	keys, cursor, err := scanKeys(db, pattern, 0)
	if err != nil {
		return nil, err
	}
	return &bytesKeyIterator{
		index:      0,
		keys:       keys,
		db:         db,
		pattern:    pattern,
		cursor:     cursor,
		trimPrefix: trimPrefix}, nil
}

func listValues(db *BytesConnectionRedis, match string,
	addPrefix func(key string) string, trimPrefix func(key string) string) (keyval.BytesKeyValIterator, error) {
	keyIterator, err := listKeys(db, match, addPrefix, trimPrefix)
	if err != nil {
		return nil, err
	}
	bkIterator := keyIterator.(*bytesKeyIterator)
	values, err := getValues(db, bkIterator.keys)
	if err != nil {
		return nil, err
	}
	return &bytesKeyValIterator{
		values:           values,
		bytesKeyIterator: *bkIterator}, nil
}

func scanKeys(db *BytesConnectionRedis, pattern string, cursor uint64) (keys []string, next uint64, err error) {
	for {
		// count == 0 defaults to Redis default. See https://redis.io/commands/scan.
		keys, next, err = db.client.Scan(cursor, pattern, 0).Result()
		if err != nil {
			db.Errorf("Scan(%s) failed: %s", pattern, err)
			return keys, next, err
		}
		if keys == nil {
			keys = []string{}
		}
		count := len(keys)
		if count > 0 || next == 0 {
			db.Debugf("scanKeys(%s): got %d keys @ cursor %d (next cursor %d)", pattern, count, cursor, next)
			return keys, next, nil
		}
		cursor = next
	}
}

func getValues(db *BytesConnectionRedis, keys []string) (values [][]byte, err error) {
	db.Debugf("getValues(%v)", keys)

	if len(keys) == 0 {
		return [][]byte{}, nil
	}

	sliceCmd := db.client.MGet(keys...)
	if sliceCmd.Err() != nil {
		return nil, fmt.Errorf("MGet(%v) failed: %s", keys, sliceCmd.Err())
	}
	vals := sliceCmd.Val()
	values = make([][]byte, len(vals))
	for i, v := range vals {
		switch o := v.(type) {
		case string:
			values[i] = []byte(o)
		case []byte:
			values[i] = o
		case nil:
			values[i] = nil
		}
	}
	return values, nil
}

// ListValuesRange returns an iterator used to traverse values stored under the provided key.
// TODO: Not in BytesBroker interface
/*
func (db *BytesConnectionRedis) ListValuesRange(fromPrefix string, toPrefix string) (keyval.BytesKeyValIterator, error) {
	db.Panic("Not implemented")
	return nil, nil
}
*/

///////////////////////////////////////////////////////////////////////////////////////////////////

// BytesBrokerWatcherRedis uses BytesConnectionRedis to access the datastore.
// The connection can be shared among multiple BytesBrokerWatcherRedis.
// BytesBrokerWatcherRedis allows to define a keyPrefix that is prepended to
// all keys in its methods in order to shorten keys used in arguments.
type BytesBrokerWatcherRedis struct {
	logging.Logger
	prefix   string
	delegate *BytesConnectionRedis

	// closeCh is a channel closed when Close method of data broker is closed.
	// It is used for giving go routines a signal to stop.
	closeCh chan string
}

// NewBrokerWatcher creates a new CRUD + KeyValProtoWatcher proxy instance to redis using BytesConnectionRedis.
// The given prefix will be prepended to key argument in all calls.
// Specify empty string ("") if not wanting to use prefix.
func (db *BytesConnectionRedis) NewBrokerWatcher(prefix string) *BytesBrokerWatcherRedis {
	return &BytesBrokerWatcherRedis{db.Logger, prefix, db, db.closeCh}
}

// NewBroker creates a new CRUD proxy instance to redis using BytesConnectionRedis.
// The given prefix will be prepended to key argument in all calls.
// Specify empty string ("") if not wanting to use prefix.
func (db *BytesConnectionRedis) NewBroker(prefix string) keyval.BytesBroker {
	return db.NewBrokerWatcher(prefix)
}

// NewWatcher creates a new KeyValProtoWatcher proxy instance to redis using BytesConnectionRedis.
// The given prefix will be prepended to key argument in all calls.
// Specify empty string ("") if not wanting to use prefix.
func (db *BytesConnectionRedis) NewWatcher(prefix string) keyval.BytesWatcher {
	return db.NewBrokerWatcher(prefix)
}

func (pdb *BytesBrokerWatcherRedis) addPrefix(key string) string {
	return pdb.prefix + key
}

func (pdb *BytesBrokerWatcherRedis) trimPrefix(key string) string {
	return strings.TrimPrefix(key, pdb.prefix)
}

// GetPrefix returns the prefix associated with this BytesBrokerWatcherRedis.
func (pdb *BytesBrokerWatcherRedis) GetPrefix() string {
	return pdb.prefix
}

// NewTxn creates new transaction. Prefix will be prepended to the key argument.
func (pdb *BytesBrokerWatcherRedis) NewTxn() keyval.BytesTxn {
	if pdb.delegate.closed {
		pdb.Error("NewTxn() called on a closed connection")
		return nil
	}
	pdb.Debug("NewTxn()")

	return &Txn{db: pdb.delegate, ops: []op{}, addPrefix: pdb.addPrefix}
}

// Put calls Put function of BytesConnectionRedis. Prefix will be prepended to the key argument.
func (pdb *BytesBrokerWatcherRedis) Put(key string, data []byte, opts ...datasync.PutOption) error {
	if pdb.delegate.closed {
		return fmt.Errorf("Put(%s) called on a closed connection", key)
	}
	pdb.Debugf("Put(%s)", key)

	return pdb.delegate.Put(pdb.addPrefix(key), data, opts...)
}

// GetValue calls GetValue function of BytesConnectionRedis.
// Prefix will be prepended to the key argument when searching.
func (pdb *BytesBrokerWatcherRedis) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	if pdb.delegate.closed {
		return nil, false, 0, fmt.Errorf("GetValue(%s) called on a closed connection", key)
	}
	pdb.Debugf("GetValue(%s)", key)

	return pdb.delegate.GetValue(pdb.addPrefix(key))
}

// ListKeys calls ListKeys function of BytesConnectionRedis.
// Prefix will be prepended to key argument when searching.
// The returned keys, however, will have the prefix trimmed.
// When done traversing, you must close the iterator by calling its Close() method.
func (pdb *BytesBrokerWatcherRedis) ListKeys(match string) (keyval.BytesKeyIterator, error) {
	if pdb.delegate.closed {
		return nil, fmt.Errorf("ListKeys(%s) called on a closed connection", match)
	}
	return listKeys(pdb.delegate, match, pdb.addPrefix, pdb.trimPrefix)
}

// ListValues calls ListValues function of BytesConnectionRedis.
// Prefix will be prepended to key argument when searching.
// The returned keys, however, will have the prefix trimmed.
// When done traversing, you must close the iterator by calling its Close() method.
func (pdb *BytesBrokerWatcherRedis) ListValues(match string) (keyval.BytesKeyValIterator, error) {
	if pdb.delegate.closed {
		return nil, fmt.Errorf("ListValues(%s) called on a closed connection", match)
	}
	return listValues(pdb.delegate, match, pdb.addPrefix, pdb.trimPrefix)
}

// Delete calls Delete function of BytesConnectionRedis.
// Prefix will be prepended to key argument when searching.
func (pdb *BytesBrokerWatcherRedis) Delete(match string, opts ...datasync.DelOption) (found bool, err error) {
	if pdb.delegate.closed {
		return false, fmt.Errorf("Delete(%s) called on a closed connection", match)
	}
	pdb.Debugf("Delete(%s)", match)

	return pdb.delegate.Delete(pdb.addPrefix(match), opts...)
}

// ListValuesRange calls ListValuesRange function of BytesConnectionRedis.
// Prefix will be prepended to key argument when searching.
// TODO: Not in BytesBroker interface
/*
func (pdb *BytesBrokerWatcherRedis) ListValuesRange(fromPrefix string, toPrefix string) (keyval.BytesKeyValIterator, error) {
	return pdb.delegate.ListValuesRange(pdb.addPrefix(fromPrefix), pdb.addPrefix(toPrefix))
}
*/

const redisWildcardChars = "*?[]"

func wildcard(match string) string {
	containsWildcard := strings.ContainsAny(match, redisWildcardChars)
	if !containsWildcard {
		return match + "*" //prefix
	}
	return match
}
