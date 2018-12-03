package bolt

import (
	"strings"

	"github.com/ligato/cn-infra/db/keyval"
)

// kvPair is used to represent a single K/V entry
type kvPair struct {
	// Key is the name of the key.
	Key string
	// Value is the value for the key.
	Value []byte
}

// bytesKeyIterator is an iterator returned by ListKeys call.
type bytesKeyIterator struct {
	prefix string
	index  int
	len    int
	keys   []string
}

// bytesKeyValIterator is an iterator returned by ListValues call.
type bytesKeyValIterator struct {
	prefix string
	index  int
	len    int
	pairs  []*kvPair
}

// bytesKeyVal represents a single key-value pair.
type bytesKeyVal struct {
	key       string
	value     []byte
	prevValue []byte
	revision  int64
}

// GetNext returns the following key (+ revision) from the result set.
// When there are no more keys to get, <stop> is returned as *true*
// and <key> and <rev> are default values.
func (it *bytesKeyIterator) GetNext() (key string, rev int64, stop bool) {
	if it.index >= it.len {
		return "", 0, true
	}

	key = it.keys[it.index]
	if it.prefix != "" {
		key = strings.TrimPrefix(key, it.prefix)
	}
	it.index++

	return key, 0, false
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (it *bytesKeyIterator) Close() error {
	return nil
}

// GetNext returns the following item from the result set.
// When there are no more items to get, <stop> is returned as *true* and <val>
// is simply *nil*.
func (it *bytesKeyValIterator) GetNext() (val keyval.BytesKeyVal, stop bool) {
	if it.index >= it.len {
		return nil, true
	}

	key := it.pairs[it.index].Key
	if it.prefix != "" {
		key = strings.TrimPrefix(key, it.prefix)
	}
	data := it.pairs[it.index].Value

	var prevValue []byte
	if len(it.pairs) > 0 && it.index > 0 {
		prevValue = it.pairs[it.index-1].Value
	}

	it.index++

	return &bytesKeyVal{key, data, prevValue, 0}, false
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (it *bytesKeyValIterator) Close() error {
	return nil
}

// Close does nothing since db cursors are not needed.
// The method is required by the code since it implements Iterator API.
func (kv *bytesKeyVal) Close() error {
	return nil
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
	return kv.revision
}
