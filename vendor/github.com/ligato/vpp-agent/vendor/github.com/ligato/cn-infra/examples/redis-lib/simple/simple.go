package main

import (
	"time"

	"fmt"
	"strconv"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/redis"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/namsral/flag"
)

// SimpleRedis is base structure which holds together all items needed to run the example - logger, redis client,
// test prefix, channel for redis responses and channel to close the redis watcher
type SimpleRedis struct {
	log    *logrus.Logger
	client redis.Client

	prefix string

	respChan  chan keyval.BytesWatchResp
	closeChan chan string
}

func main() {
	var debug bool
	var redisConfigPath string

	// init example flags
	flag.BoolVar(&debug, "debug", false, "Enable debugging")
	flag.StringVar(&redisConfigPath, "redis-config", "", "Redis configuration file path")
	flag.Parse()

	log := logrus.DefaultLogger()
	if debug {
		log.SetLevel(logging.DebugLevel)
	}
	// load redis config file
	redisConfig, err := redis.LoadConfig(redisConfigPath)
	if err != nil {
		log.Errorf("Failed to load Redis config file %s: %v", redisConfigPath, err)
		return
	}

	example := &SimpleRedis{
		log:       log,
		prefix:    "/redis/test",
		respChan:  make(chan keyval.BytesWatchResp, 10),
		closeChan: make(chan string),
	}
	doneChan := make(chan struct{})
	if broker, err := example.init(redisConfig, doneChan); err != nil {
		example.log.Errorf("simple example error: %v", err)
	} else {
		example.start(broker)
	}

	// wait for watcher
	log.Info("Waiting for watcher... (if it takes long, please make sure redis watching is enabled with, see readme)")
	<-doneChan

	log.Info("Example done, closing")
}

func (sr *SimpleRedis) init(config interface{}, doneChan chan struct{}) (broker keyval.BytesBroker, err error) {
	sr.log.Info("Simple redis example. If you need more info about what is happening, run example with -debug=true")

	// prepare client to connect to the redis DB
	sr.client, err = redis.ConfigToClient(config)
	if err != nil {
		return broker, fmt.Errorf("failed to create redis client: %v", err)
	}
	connection, err := redis.NewBytesConnection(sr.client, sr.log)
	if err != nil {
		return broker, fmt.Errorf("failed to create connection from redis client: %v", err)
	}

	// start and register the redis watcher
	go sr.watch(doneChan)
	bytesWatcher := connection.NewWatcher(sr.prefix)
	if err := bytesWatcher.Watch(keyval.ToChan(sr.respChan), sr.closeChan, sr.prefix); err != nil {
		return broker, fmt.Errorf("failed to init redis watcher: %v", err)
	}

	// prepare the broker in order to put/delete key-value pairs
	return connection.NewBroker(sr.prefix), nil
}

// watch redis database for all the keys/values put during the example
func (sr *SimpleRedis) watch(done chan struct{}) {
	sr.log.Info("==> Redis DB watcher started")

	var count int8

	for {
		select {
		case r, ok := <-sr.respChan:
			count++
			if !ok {
				sr.log.Info("==> Redis DB watcher closed")
				return
			}
			switch r.GetChangeType() {
			case datasync.Put:
				sr.log.Debugf("==> Redis watcher: received 'put' event: key: %s, value: %s", r.GetKey(), string(r.GetValue()))
			case datasync.Delete:
				sr.log.Debugf("==> Redis watcher: received 'delete' event: key: %s", r.GetKey())
			}
			if count == 12 {
				sr.log.Info("All expected events were received by watcher")
				done <- struct{}{}
			}
		}
	}
}

// start the example, testing simple key/value put, put key with TTL and list all present items by key or by value. All
// changes are also reflected in watcher
func (sr *SimpleRedis) start(db keyval.BytesBroker) {
	sr.log.Info("Start putting data to Redis...")
	time.Sleep(2 * time.Second)

	// basic key-value entry
	if err := db.Put(sr.prefix+"/key1", []byte("data1")); err != nil {
		sr.log.Errorf("put key1 failed: %v", err)
	} else {
		sr.log.Info("key1 stored in DB")
	}
	if data, found, _, err := db.GetValue(sr.prefix + "/key1"); err != nil {
		sr.log.Errorf("get key1 failed: %v", err)
	} else if !found {
		sr.log.Errorf("expected key1 does not exist: %v", err)
	} else {
		sr.log.Infof("key1 read from DB (data: %v)", data)
	}

	// key-value entry with TTL
	if err := db.Put(sr.prefix+"/key2", []byte("data2"), datasync.WithTTL(2*time.Second)); err != nil {
		sr.log.Errorf("put key1 failed: %v", err)
	}
	if data, found, _, err := db.GetValue(sr.prefix + "/key2"); err != nil {
		sr.log.Errorf("get key2 failed: %v", err)
	} else if !found {
		sr.log.Errorf("expected key2 does not exist: %v", err)
	} else {
		sr.log.Infof("key2 read from DB (data: %v)", data)
	}
	sr.log.Info("waiting for key2 TTL...")
	time.Sleep(3 * time.Second)
	if _, found, _, err := db.GetValue(sr.prefix + "/key2"); err != nil {
		sr.log.Errorf("get key2 after TTL failed: %v", err)
	} else if !found {
		sr.log.Info("key2 does not exist (expired TTL)")
	} else {
		sr.log.Error("key2 should not exist")
	}

	// put several another keys as txn
	sr.log.Info("Put a few more keys as transaction...")
	txn := db.NewTxn()
	for i := 3; i <= 11; i++ {
		txn.Put(sr.prefix+"/key"+strconv.Itoa(i), []byte("data"+strconv.Itoa(i)))
	}
	if err := txn.Commit(); err != nil {
		sr.log.Errorf("failed to commit transaction: %v", err)
	}
	sr.log.Info("... done")

	// list keys
	sr.log.Info("Listing keys (expected 10 entries)")
	var count int8
	keys, err := db.ListKeys(sr.prefix)
	if err != nil {
		sr.log.Errorf("failed to list keys: %v", err)
	} else {
		for {
			key, _, done := keys.GetNext()
			if done {
				break
			}
			sr.log.Debugf("Found key %s", key)
			count++
		}
		if count == 10 {
			sr.log.Info("all expected keys were found")
		} else {
			sr.log.Errorf("failed to get all the keys (expected 10, got %d)", count)
		}
	}

	// list keys
	sr.log.Info("Listing values (expected 10 entries)")
	count = 0
	values, err := db.ListValues(sr.prefix)
	if err != nil {
		sr.log.Errorf("failed to list values: %v", err)
	} else {
		for {
			kv, done := values.GetNext()
			if done {
				break
			}
			sr.log.Debugf("Found value %s", string(kv.GetValue()))
			count++
		}
		if count == 10 {
			sr.log.Info("all expected values were found")
		} else {
			sr.log.Errorf("failed to get all the values (expected 10, got %d)", count)
		}
	}
}

// Close close the redis client and watcher channels
func (sr *SimpleRedis) Close() error {
	return safeclose.Close(sr.client, sr.respChan, sr.closeChan)
}
