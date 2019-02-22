package filedb

import (
	"path/filepath"
	"strings"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/filedb/decoder"
)

// BrokerWatcher implements CoreBrokerWatcher and provides broker/watcher constructors with client
type BrokerWatcher struct {
	*Client
	prefix string
}

// Put calls client's 'Put' method
func (pdb *BrokerWatcher) Put(key string, data []byte, opts ...datasync.PutOption) error {
	return pdb.Client.Put(pdb.prefixKey(key), data, opts...)
}

// NewTxn calls client's 'NewTxn' method
func (pdb *BrokerWatcher) NewTxn() keyval.BytesTxn {
	return pdb.Client.NewTxn()
}

// GetValue calls client's 'GetValue' method
func (pdb *BrokerWatcher) GetValue(key string) (data []byte, found bool, revision int64, err error) {
	return pdb.Client.GetValue(pdb.prefixKey(key))
}

// Delete calls client's 'Delete' method
func (pdb *BrokerWatcher) Delete(key string, opts ...datasync.DelOption) (existed bool, err error) {
	return pdb.Client.Delete(pdb.prefixKey(key), opts...)
}

// ListValues returns a list of all database values for given key
func (pdb *BrokerWatcher) ListValues(key string) (keyval.BytesKeyValIterator, error) {
	keyValues := pdb.db.GetDataForPrefix(pdb.prefixKey(key))
	data := make([]*decoder.FileDataEntry, 0, len(keyValues))
	for _, entry := range keyValues {
		data = append(data, &decoder.FileDataEntry{
			Key:   strings.TrimPrefix(entry.Key, pdb.prefix),
			Value: entry.Value,
		})
	}
	return &bytesKeyValIterator{len: len(data), data: data}, nil
}

// ListKeys returns a list of all database keys for given prefix
func (pdb *BrokerWatcher) ListKeys(prefix string) (keyval.BytesKeyIterator, error) {
	entries := pdb.Client.db.GetDataForPrefix(prefix)
	var keys []string
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return &bytesKeyIterator{len: len(keys), keys: keys, prefix: pdb.prefix}, nil
}

// Watch augments watcher's response and removes prefix from it
func (pdb *BrokerWatcher) Watch(resp func(keyval.BytesWatchResp), closeChan chan string, keys ...string) error {
	var prefixedKeys []string
	for _, key := range keys {
		prefixedKeys = append(prefixedKeys, pdb.prefixKey(key))
	}
	return pdb.Client.Watch(func(origResp keyval.BytesWatchResp) {
		r := origResp.(*watchResp)
		r.Key = strings.TrimPrefix(r.Key, pdb.prefix)
		resp(r)
	}, closeChan, prefixedKeys...)
}

func (pdb *BrokerWatcher) prefixKey(key string) string {
	return filepath.Join(pdb.prefix, key)
}
