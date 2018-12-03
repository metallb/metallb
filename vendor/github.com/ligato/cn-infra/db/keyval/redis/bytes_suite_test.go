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
	"reflect"
	"testing"
	"time"

	"strconv"

	"os"

	"fmt"
	"strings"

	"errors"

	"github.com/alicebob/miniredis"
	goredis "github.com/go-redis/redis"
	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/onsi/gomega"
)

var miniRedis *miniredis.Miniredis
var bytesConn *BytesConnectionRedis
var bytesBrokerWatcher *BytesBrokerWatcherRedis
var ttl = time.Second
var log logging.Logger

var keyValues = map[string]string{
	"keyWest": "a place",
	"keyMap":  "a map",
}

func TestMain(m *testing.M) {
	log = logrus.DefaultLogger()

	var err error
	miniRedis, err = miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer miniRedis.Close()
	createMiniRedisConnection()
	code := m.Run()

	os.Exit(code)
}

func createMiniRedisConnection() {
	clientConfig := ClientConfig{
		Password:     "",
		DialTimeout:  0,
		ReadTimeout:  0,
		WriteTimeout: 0,
		Pool: PoolConfig{
			PoolSize:           0,
			PoolTimeout:        0,
			IdleTimeout:        0,
			IdleCheckFrequency: 0,
		},
	}
	nodeConfig := NodeConfig{
		Endpoint: miniRedis.Addr(),
		DB:       0,
		EnableReadQueryOnSlave: false,
		TLS:          TLS{},
		ClientConfig: clientConfig,
	}
	var client Client
	client = goredis.NewClient(&goredis.Options{
		Network: "tcp",
		Addr:    nodeConfig.Endpoint,

		// Database to be selected after connecting to the server
		DB: nodeConfig.DB,

		// Enables read only queries on slave nodes.
		/*ReadOnly: nodeConfig.EnableReadQueryOnSlave,*/

		// TLS Config to use. When set, TLS will be negotiated.
		TLSConfig: nil,

		// Optional password. Must match the password specified in the requirepass server configuration option.
		Password: nodeConfig.Password,

		// Dial timeout for establishing new connections. Default is 5 seconds.
		DialTimeout: nodeConfig.DialTimeout,
		// Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.
		ReadTimeout: nodeConfig.ReadTimeout,
		// Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Default is ReadTimeout.
		WriteTimeout: nodeConfig.WriteTimeout,

		// Maximum number of socket connections. Default is 10 connections per every CPU as reported by runtime.NumCPU.
		PoolSize: nodeConfig.Pool.PoolSize,
		// Amount of time client waits for connection if all connections are busy before returning an error. Default is ReadTimeout + 1 second.
		PoolTimeout: nodeConfig.Pool.PoolTimeout,
		// Amount of time after which client closes idle connections. Should be less than server's timeout. Default is 5 minutes.
		IdleTimeout: nodeConfig.Pool.IdleTimeout,
		// Frequency of idle checks. Default is 1 minute. When negative value is set, then idle check is disabled.
		IdleCheckFrequency: nodeConfig.Pool.IdleCheckFrequency,

		// Dialer creates new network connection and has priority over Network and Addr options.
		// Dialer func() (net.Conn, error)
		// Hook that is called when new connection is established
		// OnConnect func(*Conn) error

		// Maximum number of retries before giving up. Default is to not retry failed commands.
		MaxRetries: 0,
		// Minimum backoff between each retry. Default is 8 milliseconds; -1 disables backoff.
		MinRetryBackoff: 0,
		// Maximum backoff between each retry. Default is 512 milliseconds; -1 disables backoff.
		MaxRetryBackoff: 0,
	})
	// client = &MockGoredisClient{}
	bytesConn, _ = NewBytesConnection(client, logrus.DefaultLogger())
	bytesBrokerWatcher = bytesConn.NewBrokerWatcher("unit_test-")

	for k, v := range keyValues {
		miniRedis.Set(k, v)
		bytesBrokerWatcher.Put(k, []byte(v))
	}
	miniRedis.Set("bytes", "bytes")
}

func TestConfig(t *testing.T) {
	gomega.RegisterTestingT(t)

	clientConfig := ClientConfig{
		Password:     "",
		DialTimeout:  1000000,
		ReadTimeout:  0,
		WriteTimeout: 0,
		Pool: PoolConfig{
			PoolSize:           0,
			PoolTimeout:        0,
			IdleTimeout:        0,
			IdleCheckFrequency: 0,
		},
	}
	nodeConfig := NodeConfig{
		Endpoint: "localhost:6379",
		DB:       0,
		EnableReadQueryOnSlave: false,
		TLS:          TLS{},
		ClientConfig: clientConfig,
	}
	sentinelConfig := SentinelConfig{
		Endpoints:    []string{"172.17.0.7:26379", "172.17.0.8:26379", "172.17.0.9:26379"},
		MasterName:   "mymaster",
		DB:           0,
		ClientConfig: clientConfig,
	}
	clusterConfig := ClusterConfig{
		Endpoints:              []string{"172.17.0.1:6379", "172.17.0.2:6379", "172.17.0.3:6379"},
		EnableReadQueryOnSlave: true,
		MaxRedirects:           0,
		RouteByLatency:         true,
		ClientConfig:           clientConfig,
	}
	configs := []interface{}{nodeConfig, sentinelConfig, clusterConfig}
	yamlFile := "./redis_client-unit_test.yaml"
	for _, c := range configs {
		client, err := ConfigToClient(c)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(client).ShouldNot(gomega.BeNil())

		config.SaveConfigToYamlFile(c, yamlFile, 0644, makeTypeHeader(c))
		c, err = LoadConfig(yamlFile)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(c).ShouldNot(gomega.BeNil())
		client, err = ConfigToClient(c)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(client).ShouldNot(gomega.BeNil())
	}
	os.Remove(yamlFile)
}

func TestBadConfig(t *testing.T) {
	gomega.RegisterTestingT(t)

	nodeConfig := NodeConfig{
		Endpoint: "localhost:6379",
		DB:       0,
		EnableReadQueryOnSlave: false,
		TLS:          TLS{},
		ClientConfig: ClientConfig{},
	}

	var cfg *NodeConfig
	client, err := ConfigToClient(cfg)
	gomega.Expect(err).Should(gomega.HaveOccurred())
	gomega.Expect(client).Should(gomega.BeNil())
	client, err = ConfigToClient(nil)
	gomega.Expect(err).Should(gomega.HaveOccurred())
	gomega.Expect(client).Should(gomega.BeNil())

	nodeConfig.TLS.Enabled = true
	nodeConfig.TLS.CAfile = "bad CA file"
	client, err = ConfigToClient(nodeConfig)
	gomega.Expect(err).Should(gomega.HaveOccurred())
	gomega.Expect(client).Should(gomega.BeNil())

	nodeConfig.TLS.Certfile = "bad cert file"
	nodeConfig.TLS.Keyfile = "bad key file"
	client, err = ConfigToClient(nodeConfig)
	gomega.Expect(err).Should(gomega.HaveOccurred())
	gomega.Expect(client).Should(gomega.BeNil())
}

func makeTypeHeader(i interface{}) string {
	t := reflect.TypeOf(i)
	tn := t.String()
	return fmt.Sprintf("# %s#%s", t.PkgPath(), tn[strings.Index(tn, ".")+1:])
}

func TestBrokerWatcher(t *testing.T) {
	gomega.RegisterTestingT(t)
	prefix := bytesBrokerWatcher.GetPrefix()
	gomega.Expect(prefix).ShouldNot(gomega.BeNil())

	broker := bytesConn.NewBroker("")
	gomega.Expect(broker).Should(gomega.BeAssignableToTypeOf(bytesBrokerWatcher))

	watcher := bytesConn.NewWatcher("")
	gomega.Expect(watcher).Should(gomega.BeAssignableToTypeOf(bytesBrokerWatcher))
}

func TestPut(t *testing.T) {
	gomega.RegisterTestingT(t)

	err := bytesBrokerWatcher.Put("abc", []byte("123"))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	err = bytesBrokerWatcher.Put("abcWithTTL", []byte("123"), datasync.WithTTL(ttl))
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

func TestGet(t *testing.T) {
	gomega.RegisterTestingT(t)

	val, found, _, err := bytesConn.GetValue("bytes")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeTrue())
	gomega.Expect(val).ShouldNot(gomega.BeNil())

	for k, v := range keyValues {
		val, found, _, err = bytesBrokerWatcher.GetValue(k)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(found).Should(gomega.BeTrue())
		gomega.Expect(val).Should(gomega.Equal([]byte(v)))

	}

	val, found, _, err = bytesBrokerWatcher.GetValue("nil")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeFalse())
	gomega.Expect(val).Should(gomega.BeEmpty())
}

func TestListKeys(t *testing.T) {
	gomega.RegisterTestingT(t)

	keys, err := bytesBrokerWatcher.ListKeys("key")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for {
		k, _, last := keys.GetNext()
		if last {
			break
		}
		gomega.Expect(k).Should(gomega.SatisfyAny(gomega.BeEquivalentTo("keyWest"), gomega.BeEquivalentTo("keyMap")))
	}
}

func TestListValues(t *testing.T) {
	gomega.RegisterTestingT(t)

	keyVals, err := bytesConn.ListValues("key")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for {
		kv, last := keyVals.GetNext()
		if last {
			break
		}
		gomega.Expect(kv.GetKey()).Should(gomega.SatisfyAny(gomega.BeEquivalentTo("keyWest"), gomega.BeEquivalentTo("keyMap")))
		gomega.Expect(kv.GetValue()).Should(gomega.SatisfyAny(gomega.BeEquivalentTo(keyValues["keyWest"]), gomega.BeEquivalentTo(keyValues["keyMap"])))
		gomega.Expect(kv.GetRevision()).ShouldNot(gomega.BeNil())
	}

	keyVals, err = bytesBrokerWatcher.ListValues("key")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for {
		kv, last := keyVals.GetNext()
		if last {
			break
		}
		gomega.Expect(kv.GetKey()).Should(gomega.SatisfyAny(gomega.BeEquivalentTo("keyWest"), gomega.BeEquivalentTo("keyMap")))
		gomega.Expect(kv.GetValue()).Should(gomega.SatisfyAny(gomega.BeEquivalentTo(keyValues["keyWest"]), gomega.BeEquivalentTo(keyValues["keyMap"])))
		gomega.Expect(kv.GetRevision()).ShouldNot(gomega.BeNil())
	}
}

func TestKeyIterator(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefix := "KeyIterator-"
	max := 100
	for i := 1; i <= max; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		bytesBrokerWatcher.Put(key, []byte(key))
	}
	iterator, err := bytesBrokerWatcher.ListKeys(prefix)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	count := 0
	for {
		_, _, last := iterator.GetNext()
		if last {
			gomega.Expect(count).Should(gomega.Equal(max))
			break
		}
		count++
	}

	// test it.err
	iterator, err = bytesBrokerWatcher.ListKeys(prefix)
	it := iterator.(*bytesKeyIterator)
	it.err = errors.New("unittest")
	_, _, last := it.GetNext()
	gomega.Expect(last).Should(gomega.BeTrue())
	err = it.Close()
	gomega.Expect(err).Should(gomega.HaveOccurred())

	// test it.index
	iterator, err = bytesBrokerWatcher.ListKeys(prefix)
	gomega.Expect(err).To(gomega.BeNil())
	it = iterator.(*bytesKeyIterator)
	it.index = max
	it.cursor = 1 // This only meant to trigger scan.  miniRedis, however, will not accept nonzero cursor.
	_, _, _ = it.GetNext()
}

func TestKeyValIterator(t *testing.T) {
	gomega.RegisterTestingT(t)

	prefix := "KeyValIterator-"
	max := 100
	for i := 1; i <= max; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		bytesBrokerWatcher.Put(key, []byte(key))
	}
	iterator, err := bytesBrokerWatcher.ListValues(prefix)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	count := 0
	for {
		_, last := iterator.GetNext()
		if last {
			gomega.Expect(count).Should(gomega.Equal(max))
			break
		}
		count++
	}

	// test it.err
	iterator, err = bytesBrokerWatcher.ListValues(prefix)
	it := iterator.(*bytesKeyValIterator)
	it.err = errors.New("unittest")
	_, last := it.GetNext()
	gomega.Expect(last).Should(gomega.BeTrue())
	err = it.Close()
	gomega.Expect(err).Should(gomega.HaveOccurred())

	// test it.index
	iterator, err = bytesBrokerWatcher.ListValues(prefix)
	gomega.Expect(err).To(gomega.BeNil())
	it = iterator.(*bytesKeyValIterator)
	it.index = max
	it.cursor = 1 // This only meant to trigger scan.  miniRedis, however, will not accept nonzero cursor.
	_, _ = it.GetNext()
}

func TestDel(t *testing.T) {
	gomega.RegisterTestingT(t)

	found, err := bytesBrokerWatcher.Delete("key")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeFalse())

	found, err = bytesBrokerWatcher.Delete("keyWest")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeTrue())

	found, err = bytesBrokerWatcher.Delete("key", datasync.WithPrefix())
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeTrue())
}

func TestTxn(t *testing.T) {
	gomega.RegisterTestingT(t)

	txn := bytesBrokerWatcher.NewTxn()
	txn.Put("keyWest", []byte(keyValues["keyWest"])).Put("keyMap", []byte(keyValues["keyMap"]))
	txn.Delete("keyWest")
	err := txn.Commit()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	val, found, _, err := bytesBrokerWatcher.GetValue("keyWest")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeFalse())
	gomega.Expect(val).Should(gomega.BeEmpty())

	val, found, _, err = bytesBrokerWatcher.GetValue("keyMap")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeTrue())
	gomega.Expect(val).Should(gomega.Equal([]byte(keyValues["keyMap"])))

	txn = bytesBrokerWatcher.NewTxn()
	txn.Put("{hashTag}key", []byte{}).Delete("key")
	checkCrossSlot(txn.(*Txn))
}

/* miniRedis does not support PSUBSCRIBE yet.
func TestWatcher(t *testing.T) {
	gomega.RegisterTestingT(t)
}
*/

func consumeEvent(respChan chan keyval.BytesWatchResp, eventCount int) {
	for {
		r, ok := <-respChan
		if ok {
			switch r.GetChangeType() {
			case datasync.Put:
				log.Debugf("KeyValProtoWatcher received %v: %s=%s prev=%s (rev %d)",
					r.GetChangeType(), r.GetKey(), string(r.GetValue()), string(r.GetPrevValue()), r.GetRevision())
			case datasync.Delete:
				log.Debugf("KeyValProtoWatcher received %v: %s=%s prev=%s(rev %d)",
					r.GetChangeType(), r.GetKey(), string(r.GetValue()), string(r.GetPrevValue()), r.GetRevision())

			}
		} else {
			log.Error("Something wrong with Watch channel... bail out")
			break
		}
		eventCount--
		if eventCount == 0 {
			return
		}
	}
}

func newSubscriptionResponse(kind string, chanName string, count int) []interface{} {
	values := []interface{}{}
	values = append(values, interface{}([]byte(kind)))
	values = append(values, interface{}([]byte(chanName)))
	values = append(values, interface{}([]byte(strconv.Itoa(count))))
	return values
}

func newPMessage(pattern string, chanName string, data string) []interface{} {
	values := []interface{}{}
	values = append(values, interface{}([]byte("pmessage")))
	values = append(values, interface{}([]byte(pattern)))
	values = append(values, interface{}([]byte(chanName)))
	values = append(values, interface{}([]byte(data)))
	return values
}

func TestGetShouldNotApplyWildcard(t *testing.T) {
	gomega.RegisterTestingT(t)

	val, found, _, err := bytesBrokerWatcher.GetValue("key")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(found).Should(gomega.BeFalse())
	gomega.Expect(val).Should(gomega.BeEmpty())
}

/* TODO: How to produce error with miniRedis?
func TestPutError(t *testing.T) {
	gomega.RegisterTestingT(t)
}

func TestGetError(t *testing.T) {
	gomega.RegisterTestingT(t)
}
*/

func TestBrokerClosed(t *testing.T) {
	gomega.RegisterTestingT(t)

	txn := bytesConn.NewTxn()
	txn2 := bytesBrokerWatcher.NewTxn()
	err := safeclose.Close(bytesConn)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	respChan := make(chan keyval.BytesWatchResp)

	// byteConn
	err = bytesConn.Put("any", []byte("any"))
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, _, _, err = bytesConn.GetValue("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesConn.ListValues("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesConn.ListKeys("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesConn.Delete("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())

	txn.Put("keyWest", []byte(keyValues["keyWest"])).Put("keyMap", []byte(keyValues["keyMap"]))
	txn.Delete("keyWest")
	err = txn.Commit()
	gomega.Expect(err).Should(gomega.HaveOccurred())

	txn = bytesConn.NewTxn()
	gomega.Expect(txn).Should(gomega.BeNil())

	bytesConn.Watch(keyval.ToChan(respChan), nil, "key")

	// bytesBrokerWatcher
	err = bytesBrokerWatcher.Put("any", []byte("any"))
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, _, _, err = bytesBrokerWatcher.GetValue("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesBrokerWatcher.ListValues("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesBrokerWatcher.ListKeys("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())
	_, err = bytesBrokerWatcher.Delete("any")
	gomega.Expect(err).Should(gomega.HaveOccurred())

	txn2.Put("keyWest", []byte(keyValues["keyWest"])).Put("keyMap", []byte(keyValues["keyMap"]))
	txn2.Delete("keyWest")
	err = txn2.Commit()
	gomega.Expect(err).Should(gomega.HaveOccurred())

	txn2 = bytesBrokerWatcher.NewTxn()
	gomega.Expect(txn2).Should(gomega.BeNil())

	bytesBrokerWatcher.Watch(keyval.ToChan(respChan), nil, "key")

	err = safeclose.Close(bytesConn)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

///////////////////////////////////////////////////////////////////////////////
// go-redis https://github.com/go-redis/redis

type MockGoredisClient struct {
}

func (c *MockGoredisClient) Close() error {
	return nil
}
func (c *MockGoredisClient) Del(keys ...string) *goredis.IntCmd {
	args := stringsToInterfaces(append([]string{"del"}, keys...)...)
	cmd := goredis.NewIntCmd(args...)
	//TODO: Manipulate command result here...
	return cmd
}
func (c *MockGoredisClient) Get(key string) *goredis.StringCmd {
	cmd := goredis.NewStringCmd("get", key)
	//TODO: Manipulate command result here...
	return cmd
}
func (c *MockGoredisClient) MGet(keys ...string) *goredis.SliceCmd {
	args := stringsToInterfaces(append([]string{"mget"}, keys...)...)
	cmd := goredis.NewSliceCmd(args...)
	//TODO: Manipulate command result here...
	return cmd
}
func (c *MockGoredisClient) Scan(cursor uint64, match string, count int64) *goredis.ScanCmd {
	args := []interface{}{"scan", cursor}
	if match != "" {
		args = append(args, "match", match)
	}
	if count > 0 {
		args = append(args, "count", count)
	}
	cmd := goredis.NewScanCmd(func(cmd goredis.Cmder) error {
		//TODO: Manipulate command result here...
		return nil
	}, args...)
	return cmd
}
func (c *MockGoredisClient) Set(key string, value interface{}, expiration time.Duration) *goredis.StatusCmd {
	args := make([]interface{}, 3, 4)
	args[0] = "set"
	args[1] = key
	args[2] = value
	if expiration > 0 {
		if expiration < time.Second || expiration%time.Second != 0 {
			args = append(args, "px", expiration/time.Millisecond)
		} else {
			args = append(args, "ex", expiration/time.Second)
		}
	}
	cmd := goredis.NewStatusCmd(args...)
	//TODO: Manipulate command result here...
	return cmd
}
func (c *MockGoredisClient) TxPipeline() goredis.Pipeliner {
	//TODO: Manipulate the pipeliner...
	return &goredis.Pipeline{}
}
func (c *MockGoredisClient) PSubscribe(channels ...string) *goredis.PubSub {
	pubSub := &goredis.PubSub{}
	//TODO: PubSub is a struct with all internal fields. Is it possible to manipulate?
	return pubSub
}

func stringsToInterfaces(ss ...string) []interface{} {
	args := make([]interface{}, len(ss))
	for i, s := range ss {
		args[i] = s
	}
	return args
}
