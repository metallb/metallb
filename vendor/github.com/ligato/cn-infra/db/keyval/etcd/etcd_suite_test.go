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
	"errors"
	"testing"
	"time"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging/logrus"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

// MockKV mocks KV for Etcd client.
type MockKV struct {
	mem        map[string]string
	shouldFail bool
}

func (mock *MockKV) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if mock.shouldFail {
		return nil, errors.New("test-error")
	}

	mock.mem[key] = val

	return &clientv3.PutResponse{}, nil
}

func (mock *MockKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if mock.shouldFail {
		return nil, errors.New("test-error")
	}

	var kvs []*mvccpb.KeyValue

	if val, ok := mock.mem[key]; ok {
		kvs = append(kvs, &mvccpb.KeyValue{
			Key:   []byte(key),
			Value: []byte(val),
		})
	}

	return &clientv3.GetResponse{Kvs: kvs}, nil
}

func (mock *MockKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	if mock.shouldFail {
		return nil, errors.New("test-error")
	}

	var prevKvs []*mvccpb.KeyValue

	if prevVal, ok := mock.mem[key]; ok {
		prevKvs = append(prevKvs, &mvccpb.KeyValue{
			Key:   []byte(key),
			Value: []byte(prevVal),
		})
		delete(mock.mem, key)
	}

	return &clientv3.DeleteResponse{PrevKvs: prevKvs}, nil
}

func (mock *MockKV) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return nil, nil
}

func (mock *MockKV) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (mock *MockKV) Txn(ctx context.Context) clientv3.Txn {
	return &MockTxn{}
}

func (mock *MockKV) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return nil
}

func (mock *MockKV) Close() error {
	return nil
}

// Mock Txn
type MockTxn struct {
}

func (mock *MockTxn) If(cs ...clientv3.Cmp) clientv3.Txn {
	return &MockTxn{}
}

func (mock *MockTxn) Then(ops ...clientv3.Op) clientv3.Txn {
	return &MockTxn{}
}

func (mock *MockTxn) Else(ops ...clientv3.Op) clientv3.Txn {
	return &MockTxn{}
}

func (mock *MockTxn) Commit() (*clientv3.TxnResponse, error) {
	return nil, nil
}

// Tests

type testCtx struct {
	mockKV     *MockKV
	dataBroker *BytesConnectionEtcd
}

func setupTest(t *testing.T) *testCtx {
	RegisterTestingT(t)

	mockKV := &MockKV{
		mem: make(map[string]string),
	}
	dataBroker := &BytesConnectionEtcd{
		Logger: logrus.DefaultLogger(),
		etcdClient: &clientv3.Client{
			KV:      mockKV,
			Watcher: mockKV,
		},
	}

	return &testCtx{
		mockKV:     mockKV,
		dataBroker: dataBroker,
	}
}

func (ctx *testCtx) teardownTest() {

}

func TestPut(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	// regular case
	err := ctx.dataBroker.Put("key", []byte("data"))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(ctx.mockKV.mem["key"]).To(Equal("data"))

	// error case
	ctx.mockKV.shouldFail = true
	err = ctx.dataBroker.Put("key", []byte("data"))
	Expect(err).Should(HaveOccurred())
	Expect(err.Error()).To(BeEquivalentTo("test-error"))
}

func TestGetValue(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	// regular case
	ctx.mockKV.mem["key"] = "data"
	result, found, _, err := ctx.dataBroker.GetValue("key")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(found).To(BeTrue())
	Expect(result).To(Equal([]byte("data")))

	// error case
	ctx.mockKV.shouldFail = true
	result, found, _, err = ctx.dataBroker.GetValue("key")
	Expect(err).Should(HaveOccurred())
	Expect(err.Error()).To(BeEquivalentTo("test-error"))
}

func TestListValues(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	// regular case
	ctx.mockKV.mem["key"] = "data"
	iter, err := ctx.dataBroker.ListValues("key")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(iter).ToNot(BeNil())
	var val []byte
	for {
		kv, stop := iter.GetNext()
		if stop {
			break
		}
		val = kv.GetValue()
	}
	Expect(val).To(Equal([]byte("data")))

	// error case
	ctx.mockKV.shouldFail = true
	_, err = ctx.dataBroker.ListValues("key")
	Expect(err).Should(HaveOccurred())
	Expect(err.Error()).To(BeEquivalentTo("test-error"))
}

func TestDelete(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	// regular case
	ctx.mockKV.mem["key"] = "data"
	response, err := ctx.dataBroker.Delete("key")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(response).To(BeTrue())

	// error case
	ctx.mockKV.shouldFail = true
	response, err = ctx.dataBroker.Delete("key")
	Expect(err).Should(HaveOccurred())
	Expect(err.Error()).To(BeEquivalentTo("test-error"))
}

func TestListValuesRange(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	// regular case
	result, err := ctx.dataBroker.ListValuesRange("AKey", "ZKey")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(result).ToNot(BeNil())

	// error case
	ctx.mockKV.shouldFail = true
	result, err = ctx.dataBroker.ListValuesRange("AKey", "ZKey")
	Expect(err).Should(HaveOccurred())
	Expect(err.Error()).To(BeEquivalentTo("test-error"))
}

func TestNewBroker(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	pdb := ctx.dataBroker.NewBroker("/pluginname")
	Expect(pdb).NotTo(BeNil())
}

func TestNewWatcher(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	pdb := ctx.dataBroker.NewWatcher("/pluginname")
	Expect(pdb).NotTo(BeNil())
}

func TestWatch(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	err := ctx.dataBroker.Watch(func(keyval.BytesWatchResp) {}, nil, "key")
	Expect(err).ShouldNot(HaveOccurred())
}

func TestWatchPutResp(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	rev := int64(1)
	value := []byte("data")
	prevVal := []byte("prevData")
	key := "key"

	createResp := NewBytesWatchPutResp(key, value, prevVal, rev)
	Expect(createResp).NotTo(BeNil())
	Expect(createResp.GetChangeType()).To(BeEquivalentTo(datasync.Put))
	Expect(createResp.GetKey()).To(BeEquivalentTo(key))
	Expect(createResp.GetValue()).To(BeEquivalentTo(value))
	Expect(createResp.GetPrevValue()).To(BeEquivalentTo(prevVal))
	Expect(createResp.GetRevision()).To(BeEquivalentTo(rev))
}

func TestWatchDeleteResp(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	rev := int64(1)
	key := "key"
	prevVal := []byte("prevVal")

	createResp := NewBytesWatchDelResp(key, prevVal, rev)
	Expect(createResp).NotTo(BeNil())
	Expect(createResp.GetChangeType()).To(BeEquivalentTo(datasync.Delete))
	Expect(createResp.GetKey()).To(BeEquivalentTo(key))
	Expect(createResp.GetValue()).To(BeNil())
	Expect(createResp.GetPrevValue()).To(BeEquivalentTo(prevVal))
	Expect(createResp.GetRevision()).To(BeEquivalentTo(rev))
}

func TestConfig(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	cfg := &Config{DialTimeout: time.Second, OpTimeout: time.Second}
	etcdCfg, err := ConfigToClient(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(etcdCfg).NotTo(BeNil())
	Expect(etcdCfg.OpTimeout).To(BeEquivalentTo(time.Second))
	Expect(etcdCfg.DialTimeout).To(BeEquivalentTo(time.Second))
	Expect(etcdCfg.TLS).To(BeNil())
}

func TestTxnPut(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	txn := ctx.dataBroker.NewTxn()
	Expect(txn).NotTo(BeNil())
	txn = txn.Put("key", []byte("data"))
	Expect(txn).NotTo(BeNil())
	err := txn.Commit()
	Expect(err).ToNot(HaveOccurred())
}

func TestTxnDelete(t *testing.T) {
	ctx := setupTest(t)
	defer ctx.teardownTest()

	txn := ctx.dataBroker.NewTxn()
	Expect(txn).NotTo(BeNil())
	txn = txn.Delete("key")
	Expect(txn).NotTo(BeNil())
	err := txn.Commit()
	Expect(err).ToNot(HaveOccurred())
}
