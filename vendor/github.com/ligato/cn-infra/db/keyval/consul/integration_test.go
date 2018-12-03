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

package consul

import (
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	. "github.com/onsi/gomega"
)

func init() {
	logrus.DefaultLogger().SetLevel(logging.DebugLevel)
}

type testCtx struct {
	client  *Client
	testSrv *testutil.TestServer
}

func setupTest(t *testing.T) *testCtx {
	RegisterTestingT(t)

	srv, err := testutil.NewTestServer()
	if err != nil {
		t.Fatal("setting up test server failed:", err)
	}

	cfg := api.DefaultConfig()
	cfg.Address = srv.HTTPAddr

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal("connecting to consul failed:", err)
	}

	return &testCtx{client: client, testSrv: srv}
}

func (ctx *testCtx) teardownTest() {
	ctx.client.Close()
	ctx.testSrv.Stop()
}

func TestPut(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	err := ctx.client.Put("key", []byte("val"))
	Expect(err).ToNot(HaveOccurred())

	Expect(ctx.testSrv.GetKVString(t, "key")).To(Equal("val"))
}

func TestGetValue(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.SetKV(t, "key", []byte("val"))

	data, found, rev, err := ctx.client.GetValue("key")
	Expect(err).ToNot(HaveOccurred())
	Expect(data).To(Equal([]byte("val")))
	Expect(found).To(BeTrue())
	Expect(rev).NotTo(BeZero())
}

func TestDelete(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.SetKV(t, "key", []byte("val"))

	existed, err := ctx.client.Delete("key")
	Expect(err).ToNot(HaveOccurred())
	Expect(existed).To(BeTrue())

	Expect(ctx.testSrv.ListKV(t, "")).To(BeEmpty())
}

func TestListKeys(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"key/1": []byte("val1"),
		"key/2": []byte("val2"),
	})

	kvi, err := ctx.client.ListKeys("key/")
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

func TestListKeysPrefixed(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"myprefix/key/1": []byte("val1"),
		"myprefix/key/2": []byte("val2"),
		"key/x":          []byte("valx"),
	})

	client := ctx.client.NewBroker("myprefix/")
	kvi, err := client.ListKeys("key")
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
		// verify that prefix of BytesBrokerWatcherEtcd is trimmed
		Expect(key).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestListKeysPrefixedSlash(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"myprefix/key/1": []byte("val1"),
		"myprefix/key/2": []byte("val2"),
		"myprefix/anb/7": []byte("xxx"),
		"key/x":          []byte("valx"),
	})

	client := ctx.client.NewBroker("/myprefix/")
	kvi, err := client.ListKeys("key/")
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
		// verify that prefix of BytesBrokerWatcherEtcd is trimmed
		Expect(key).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestListValues(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"key/1":  []byte("val1"),
		"key/2":  []byte("val2"),
		"foo/22": []byte("bar33"),
	})

	kvi, err := ctx.client.ListValues("key")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/1", "key/2"}
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

func TestListValuesSlash(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"key/1":  []byte("val1"),
		"key/2":  []byte("val2"),
		"foo/22": []byte("bar33"),
	})

	kvi, err := ctx.client.ListValues("/key")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/1", "key/2"}
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

func TestListValuesPrefixed(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"myprefix/key/at/1": []byte("val1"),
		"myprefix/key/at/2": []byte("val2"),
		"myprefix/key/bt/3": []byte("val3"),
		"key/x":             []byte("valx"),
	})

	client := ctx.client.NewBroker("myprefix/")
	kvi, err := client.ListValues("key/at/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/at/1", "key/at/2"}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		t.Logf("%+v", kv.GetKey())
		Expect(kv).NotTo(BeNil())
		Expect(all).To(BeFalse())
		// verify that prefix of BytesBrokerWatcherEtcd is trimmed
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestListValuesPrefixedSlash(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	ctx.testSrv.PopulateKV(t, map[string][]byte{
		"myprefix/key/at/1": []byte("val1"),
		"myprefix/key/at/2": []byte("val2"),
		"myprefix/key/bt/3": []byte("val3"),
		"key/x":             []byte("valx"),
	})

	client := ctx.client.NewBroker("/myprefix/")
	kvi, err := client.ListValues("key/at/")
	Expect(err).ToNot(HaveOccurred())
	Expect(kvi).NotTo(BeNil())

	expectedKeys := []string{"key/at/1", "key/at/2"}
	for i := 0; i <= len(expectedKeys); i++ {
		kv, all := kvi.GetNext()
		if i == len(expectedKeys) {
			Expect(all).To(BeTrue())
			break
		}
		t.Logf("%+v", kv.GetKey())
		Expect(kv).NotTo(BeNil())
		Expect(all).To(BeFalse())
		// verify that prefix of BytesBrokerWatcherEtcd is trimmed
		Expect(kv.GetKey()).To(BeEquivalentTo(expectedKeys[i]))
	}
}

func TestWatch(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	watchKey := "key/"

	closeCh := make(chan string)
	watchCh := make(chan keyval.BytesWatchResp)
	err := ctx.client.Watch(keyval.ToChan(watchCh), closeCh, watchKey)
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

	ctx.client.Put("/something/else/val1", []byte{0, 0, 7})
	ctx.client.Put(watchKey+"val1", []byte{1, 2, 3})

	wg.Wait()
}
