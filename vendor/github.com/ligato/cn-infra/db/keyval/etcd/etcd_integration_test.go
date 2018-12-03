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
	"sync"
	"testing"
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd/mocks"
	"github.com/ligato/cn-infra/logging/logrus"

	"github.com/coreos/etcd/etcdserver/api/v3client"
	. "github.com/onsi/gomega"
)

const (
	prefix   = "/my/prefix/"
	key      = "key"
	watchKey = "vals/"
)

var (
	broker          *BytesConnectionEtcd
	prefixedBroker  keyval.BytesBroker
	prefixedWatcher keyval.BytesWatcher
	embd            mocks.Embedded
)

func TestDataBroker(t *testing.T) {
	embd.Start(t)
	defer embd.Stop()
	RegisterTestingT(t)

	t.Run("putGetValue", testPutGetValuePrefixed)
	embd.CleanDs()
	t.Run("simpleWatcher", testPrefixedWatcher)
	embd.CleanDs()
	t.Run("listValues", testPrefixedListValues)
	embd.CleanDs()
	t.Run("txn", testPrefixedTxn)
	embd.CleanDs()
	t.Run("testDelWithPrefix", testDelWithPrefix)
	embd.CleanDs()
	t.Run("testPutIfNotExist", testPutIfNotExists)
	embd.CleanDs()
	t.Run("compact", testCompact)
}

func setupBrokers(t *testing.T) {
	RegisterTestingT(t)

	var err error
	broker, err = NewEtcdConnectionUsingClient(v3client.New(embd.ETCD.Server), logrus.DefaultLogger())

	Expect(err).To(BeNil())
	Expect(broker).NotTo(BeNil())
	// Create BytesBrokerWatcherEtcd with prefix.
	prefixedBroker = broker.NewBroker(prefix)
	prefixedWatcher = broker.NewWatcher(prefix)
	Expect(prefixedBroker).NotTo(BeNil())
	Expect(prefixedWatcher).NotTo(BeNil())
}

func teardownBrokers() {
	broker.Close()
	broker = nil
	prefixedBroker = nil
	prefixedWatcher = nil
}

func testPutGetValuePrefixed(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	data := []byte{1, 2, 3}

	// Insert key-value pair using databroker.
	err := broker.Put(prefix+key, data)
	Expect(err).To(BeNil())

	returnedData, found, _, err := prefixedBroker.GetValue(key)
	Expect(returnedData).NotTo(BeNil())
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())

	// not existing value
	returnedData, found, _, err = prefixedBroker.GetValue("unknown")
	Expect(returnedData).To(BeNil())
	Expect(found).To(BeFalse())
	Expect(err).To(BeNil())

}

func testPrefixedWatcher(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	closeCh := make(chan string)
	watchCh := make(chan keyval.BytesWatchResp)
	err := prefixedWatcher.Watch(keyval.ToChan(watchCh), closeCh, watchKey)
	Expect(err).To(BeNil())

	var wg sync.WaitGroup
	wg.Add(1)

	go func(expectedKey string) {
		select {
		case resp := <-watchCh:
			Expect(resp).NotTo(BeNil())
			Expect(resp.GetKey()).To(BeEquivalentTo(expectedKey))
		case <-time.After(time.Second):
			t.Error("Watch resp not received")
			t.FailNow()
		}
		close(closeCh)
		wg.Done()
	}(watchKey + "val1")

	// Insert kv that doesn't match the watcher subscription.
	broker.Put(prefix+"/something/else/val1", []byte{0, 0, 7})

	// Insert kv for watcher.
	broker.Put(prefix+watchKey+"val1", []byte{0, 0, 7})

	wg.Wait()
}

func testPrefixedTxn(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	tx := prefixedBroker.NewTxn()
	Expect(tx).NotTo(BeNil())

	tx.Put("b/val1", []byte{0, 1})
	tx.Put("b/val2", []byte{0, 1})
	tx.Put("b/val3", []byte{0, 1})
	tx.Commit()

	kvi, err := broker.ListValues(prefix + "b")
	Expect(err).To(BeNil())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{prefix + "b/val1", prefix + "b/val2", prefix + "b/val3"}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(kv).NotTo(BeNil())
		Expect(all).To(BeFalse())
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func testPrefixedListValues(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	var err error
	// Insert values using databroker.
	err = broker.Put(prefix+"a/val1", []byte{0, 0, 7})
	Expect(err).To(BeNil())
	err = broker.Put(prefix+"a/val2", []byte{0, 0, 7})
	Expect(err).To(BeNil())
	err = broker.Put(prefix+"a/val3", []byte{0, 0, 7})
	Expect(err).To(BeNil())

	// List values using pluginDatabroker.
	kvi, err := prefixedBroker.ListValues("a")
	Expect(err).To(BeNil())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"a/val1", "a/val2", "a/val3"}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(kv).NotTo(BeNil())
		Expect(all).To(BeFalse())
		// verify that prefix of BytesBrokerWatcherEtcd is trimmed
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func testDelWithPrefix(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	err := broker.Put("something/a/val1", []byte{0, 0, 7})
	Expect(err).To(BeNil())
	err = broker.Put("something/a/val2", []byte{0, 0, 7})
	Expect(err).To(BeNil())
	err = broker.Put("something/a/val3", []byte{0, 0, 7})
	Expect(err).To(BeNil())

	_, found, _, err := broker.GetValue("something/a/val1")
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())

	_, found, _, err = broker.GetValue("something/a/val2")
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())

	_, found, _, err = broker.GetValue("something/a/val3")
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())

	_, err = broker.Delete("something/a", datasync.WithPrefix())
	Expect(err).To(BeNil())

	_, found, _, err = broker.GetValue("something/a/val1")
	Expect(found).To(BeFalse())
	Expect(err).To(BeNil())

	_, found, _, err = broker.GetValue("something/a/val2")
	Expect(found).To(BeFalse())
	Expect(err).To(BeNil())

	_, found, _, err = broker.GetValue("something/a/val3")
	Expect(found).To(BeFalse())
	Expect(err).To(BeNil())

}

func testPutIfNotExists(t *testing.T) {
	RegisterTestingT(t)

	conn, err := NewEtcdConnectionUsingClient(v3client.New(embd.ETCD.Server), logrus.DefaultLogger())

	Expect(err).To(BeNil())
	Expect(conn).NotTo(BeNil())

	const key = "myKey"
	var (
		intialValue  = []byte("abcd")
		changedValue = []byte("modified")
	)

	_, found, _, err := conn.GetValue(key)
	Expect(err).To(BeNil())
	Expect(found).To(BeFalse())

	inserted, err := conn.PutIfNotExists(key, intialValue)
	Expect(err).To(BeNil())
	Expect(inserted).To(BeTrue())

	data, found, _, err := conn.GetValue(key)
	Expect(err).To(BeNil())
	Expect(found).To(BeTrue())
	Expect(string(data)).To(BeEquivalentTo(string(intialValue)))

	inserted, err = conn.PutIfNotExists(key, changedValue)
	Expect(err).To(BeNil())
	Expect(inserted).To(BeFalse())

	data, found, _, err = conn.GetValue(key)
	Expect(err).To(BeNil())
	Expect(found).To(BeTrue())
	Expect(string(data)).To(BeEquivalentTo(string(intialValue)))

	_, err = conn.Delete(key)
	Expect(err).To(BeNil())

	inserted, err = conn.PutIfNotExists(key, changedValue)
	Expect(err).To(BeNil())
	Expect(inserted).To(BeTrue())

	data, found, _, err = conn.GetValue(key)
	Expect(err).To(BeNil())
	Expect(found).To(BeTrue())
	Expect(string(data)).To(BeEquivalentTo(string(changedValue)))

}

func testCompact(t *testing.T) {
	setupBrokers(t)
	defer teardownBrokers()

	/*
		This test runs following scenario:
		- store some data to key
		- overwrite with new data 		=> expect mod revision to increment
		- get previous revision			=> expect original data to return
		- compact to current revision
		- try to retrieve original data	=> expect to fail
	*/

	mykey := "mykey"
	data := []byte{1, 2, 3}
	data2 := []byte{4, 5, 6}

	//broker.etcdClient.Maintenance.Status(context.TODO())
	revision, err := broker.GetRevision()
	Expect(err).To(BeNil())
	t.Log("current revision:", revision)

	// insert some data
	err = broker.Put(prefix+mykey, data)
	Expect(err).To(BeNil())

	// retrieve the data
	retData, found, modRev, err := prefixedBroker.GetValue(mykey)
	Expect(retData).NotTo(BeNil())
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())
	//Expect(rev).To(Equal(1))
	t.Log("data:", retData, "modrev:", modRev)

	// store its mod revision
	firsRev := modRev

	// overwrite the data with new data
	err = broker.Put(prefix+mykey, data2)
	Expect(err).To(BeNil())

	// retrieve the new data
	retData, found, modRev, err = prefixedBroker.GetValue(mykey)
	Expect(retData).NotTo(BeNil())
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())
	//Expect(rev).To(Equal(1))
	t.Log("data:", retData, "modrev:", modRev)

	// retrieve the previous revision
	retData, found, modRev, err = broker.GetValueRev(prefix+mykey, firsRev)
	Expect(retData).NotTo(BeNil())
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())
	//Expect(rev).To(Equal(1))
	t.Log("data:", retData, "modrev:", modRev)

	// get current revision
	revision, err = broker.GetRevision()
	Expect(err).To(BeNil())
	t.Log("current revision:", revision)

	// compact to current revision
	toRev, err := broker.Compact()
	Expect(err).To(BeNil())
	t.Log("compacted to revision:", toRev)

	// try retrieving previous revision
	retData, found, modRev, err = broker.GetValueRev(prefix+mykey, firsRev)
	Expect(retData).To(BeNil())
	Expect(found).NotTo(BeTrue())
	Expect(err).NotTo(BeNil())
}
