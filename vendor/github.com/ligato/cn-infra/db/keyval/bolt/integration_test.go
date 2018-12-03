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

package bolt

import (
	"os"
	"testing"

	"bytes"

	"github.com/boltdb/bolt"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	. "github.com/onsi/gomega"
)

func init() {
	logrus.DefaultLogger().SetLevel(logging.DebugLevel)
	boltLogger.SetLevel(logging.DebugLevel)
}

type testCtx struct {
	*testing.T
	client *Client
}

const testDbPath = "/tmp/bolt.db"

func setupTest(t *testing.T, newDB bool) *testCtx {
	RegisterTestingT(t)

	if newDB {
		err := os.Remove(testDbPath)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
			return nil
		}
	}

	client, err := NewClient(&Config{
		DbPath:   testDbPath,
		FileMode: 432,
	})
	Expect(err).ToNot(HaveOccurred())
	if err != nil {
		return nil
	}

	return &testCtx{T: t, client: client}
}

func (tc *testCtx) teardownTest() {
	Expect(tc.client.Close()).To(Succeed())
}

func (tc *testCtx) isInDB(key string, expectedVal []byte) (exists bool) {
	err := tc.client.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(rootBucket)
		if val := b.Get([]byte(key)); val != nil {
			exists = true
		}
		return nil
	})
	if err != nil {
		tc.Fatal(err)
	}
	return
}

func (tc *testCtx) populateDB(data map[string][]byte) {
	err := tc.client.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(rootBucket)
		for key, val := range data {
			if err := b.Put([]byte(key), val); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		tc.Fatal(err)
	}
	return
}

func TestPut(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	var key = "/agent/agent1/config/interface/iface0"
	var val = []byte("val")

	err := tc.client.Put(key, val)
	Expect(err).ToNot(HaveOccurred())
	Expect(tc.isInDB(key, val)).To(BeTrue())
}

func TestGet(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	var key = "/agent/agent1/config/interface/iface0"
	var val = []byte("val")

	err := tc.client.Put(key, val)
	Expect(err).ToNot(HaveOccurred())
	Expect(tc.isInDB(key, val)).To(BeTrue())
}

func TestListKeys(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	tc.populateDB(map[string][]byte{
		"/my/key/1":    []byte("val1"),
		"/my/key/2":    []byte("val2"),
		"/other/key/0": []byte("val0"),
	})

	kvi, err := tc.client.ListKeys("/my/key/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"/my/key/1", "/my/key/2"}
	for i := 0; i <= len(expectedKeys); i++ {
		key, _, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(all).To(BeFalse())
		Expect(key).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestListValues(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	tc.populateDB(map[string][]byte{
		"/my/key/1":    []byte("val1"),
		"/my/key/2":    []byte("val2"),
		"/other/key/0": []byte("val0"),
	})

	kvi, err := tc.client.ListValues("/my/key/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"/my/key/1", "/my/key/2"}
	expectedValues := [][]byte{[]byte("val1"), []byte("val2")}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(all).To(BeFalse())
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
		Expect(bytes.Compare(kv.GetValue(), expectedValues[i])).To(BeZero())
	}
}

func TestListKeysBroker(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	tc.populateDB(map[string][]byte{
		"/my/key/1":    []byte("val1"),
		"/my/key/2":    []byte("val2"),
		"/my/keyx/xx":  []byte("x"),
		"/my/xkey/xx":  []byte("x"),
		"/other/key/0": []byte("val0"),
	})

	broker := tc.client.NewBroker("/my/")
	kvi, err := broker.ListKeys("key/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/1", "key/2"}
	for i := 0; i <= len(expectedKeys); i++ {
		key, _, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(all).To(BeFalse())
		Expect(key).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestListValuesBroker(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	tc.populateDB(map[string][]byte{
		"/my/key/1":    []byte("val1"),
		"/my/key/2":    []byte("val2"),
		"/my/keyx/xx":  []byte("x"),
		"/my/xkey/xx":  []byte("x"),
		"/other/key/0": []byte("val0"),
	})

	broker := tc.client.NewBroker("/my/")
	kvi, err := broker.ListValues("key/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/1", "key/2"}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		Expect(all).To(BeFalse())
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestDelete(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	var key = "/agent/agent1/config/interface/iface0"
	var val = []byte("val")

	err := tc.client.Put(key, val)
	Expect(err).ToNot(HaveOccurred())
	existed, err := tc.client.Delete(key)
	Expect(err).ToNot(HaveOccurred())
	Expect(existed).To(BeTrue())

	existed, err = tc.client.Delete("/this/key/does/not/exists")
	Expect(err).To(HaveOccurred())
	Expect(existed).To(BeFalse())
	Expect(tc.isInDB(key, val)).To(BeFalse())
}

func TestPutInTxn(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	txn := tc.client.NewTxn()
	Expect(txn).ToNot(BeNil())

	var key1 = "/agent/agent1/config/interface/iface0"
	var val1 = []byte("iface0")
	var key2 = "/agent/agent1/config/interface/iface1"
	var val2 = []byte("iface1")
	var key3 = "/agent/agent1/config/interface/iface2"
	var val3 = []byte("iface2")

	txn.Put(key1, val1).
		Put(key2, val2).
		Put(key3, val3)
	Expect(txn.Commit()).To(Succeed())
	Expect(tc.isInDB(key1, val1)).To(BeTrue())
	Expect(tc.isInDB(key2, val2)).To(BeTrue())
	Expect(tc.isInDB(key3, val3)).To(BeTrue())
}

func TestDeleteInTxn(t *testing.T) {
	tc := setupTest(t, true)
	defer tc.teardownTest()

	txn := tc.client.NewTxn()
	Expect(txn).ToNot(BeNil())

	var key1 = "/agent/agent1/config/interface/iface0"
	var val1 = []byte("iface0")
	var key2 = "/agent/agent1/config/interface/iface1"
	var val2 = []byte("iface1")
	var key3 = "/agent/agent1/config/interface/iface2"
	var val3 = []byte("iface2")

	txn.Put(key1, val1).
		Put(key2, val2).
		Put(key3, val3).
		Delete(key2)
	Expect(txn.Commit()).To(Succeed())
	Expect(tc.isInDB(key1, val1)).To(BeTrue())
	Expect(tc.isInDB(key2, val2)).To(BeFalse())
	Expect(tc.isInDB(key3, val3)).To(BeTrue())
}
