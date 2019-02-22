package filedb

import (
	"strings"

	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"

	"github.com/ligato/cn-infra/db/keyval"
)

// File system DB BytesKeyValIterator implementation
type bytesKeyValIterator struct {
	index int
	len   int
	rev   int
	data  []*decoder.FileDataEntry
}

// File system DB BytesKeyIterator implementation
type bytesKeyIterator struct {
	index  int
	len    int
	keys   []string
	prefix string
}

// File system DB BytesKeyVal implementation
type bytesKeyVal struct {
	key       string
	value     []byte
	prevValue []byte
}

func (it *bytesKeyValIterator) GetNext() (val keyval.BytesKeyVal, stop bool) {
	if it.index >= it.len {
		return nil, true
	}

	key := it.data[it.index].Key
	data := it.data[it.index].Value

	var prevValue []byte
	if len(it.data) > 0 && it.index > 0 {
		prevValue = it.data[it.index-1].Value
	}

	it.index++

	return &bytesKeyVal{key, data, prevValue}, false
}

func (it *bytesKeyIterator) GetNext() (key string, rev int64, stop bool) {
	if it.index >= it.len {
		return "", 0, true
	}

	key = string(it.keys[it.index])
	if !strings.HasPrefix(key, "/") && strings.HasPrefix(it.prefix, "/") {
		key = "/" + key
	}
	if it.prefix != "" {
		key = strings.TrimPrefix(key, it.prefix)
	}
	it.index++

	return key, rev, false
}

func (kv *bytesKeyVal) GetValue() []byte {
	return kv.value
}

func (kv *bytesKeyVal) GetPrevValue() []byte {
	return kv.prevValue
}

func (kv *bytesKeyVal) GetKey() string {
	return kv.key
}

func (kv *bytesKeyVal) GetRevision() (rev int64) {
	return 0
}
