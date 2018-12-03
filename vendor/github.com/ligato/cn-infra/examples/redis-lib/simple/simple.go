package main

import (
	"reflect"
	"time"

	"fmt"
	"strconv"
	"strings"

	"os"

	"github.com/ligato/cn-infra/config"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/redis"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/namsral/flag"
)

var log = logrus.DefaultLogger()

var redisConn *redis.BytesConnectionRedis
var broker keyval.BytesBroker
var watcher keyval.BytesWatcher

var prefix string
var debug bool
var debugIterator bool
var redisConfig string

func main() {
	//generateSampleConfigs()

	cfg := loadConfig()
	if cfg == nil {
		return
	}
	fmt.Printf("config: %T:\n%v\n", cfg, cfg)
	fmt.Printf("prefix: %s\n", prefix)

	redisConn = createConnection(cfg)
	broker = redisConn.NewBroker(prefix)
	watcher = redisConn.NewWatcher(prefix)

	runSimpleExmple()
}

func loadConfig() interface{} {
	flag.StringVar(&prefix, "prefix", "",
		"Specifies key prefix")
	flag.BoolVar(&debug, "debug", false,
		"Specifies whether to enable debugging; default to false")
	flag.BoolVar(&debugIterator, "debug-iterator", false,
		"Specifies whether to enable debugging; default to false")
	flag.StringVar(&redisConfig, "redis-config", "",
		"Specifies configuration file path")
	flag.Parse()

	flag.Usage = func() {
		flag.VisitAll(func(f *flag.Flag) {
			var format string
			if f.Name == "redis-config" || f.Name == "prefix" {
				// put quotes around string
				format = "  -%s=%q: %s\n"
			} else {
				if f.Name != "debug" && f.Name != "debug-iterator" {
					return
				}
				format = "  -%s=%s: %s\n"
			}
			fmt.Fprintf(os.Stderr, format, f.Name, f.DefValue, f.Usage)
		})

	}

	if debug {
		log.SetLevel(logging.DebugLevel)
	}
	cfgFlag := flag.Lookup("redis-config")
	if cfgFlag == nil {
		flag.Usage()
		return nil
	}
	cfgFile := cfgFlag.Value.String()
	if cfgFile == "" {
		flag.Usage()
		return nil
	}
	cfg, err := redis.LoadConfig(cfgFile)
	if err != nil {
		log.Panicf("LoadConfig(%s) failed: %s", cfgFile, err)
	}
	return cfg
}

func createConnection(cfg interface{}) *redis.BytesConnectionRedis {
	client, err := redis.ConfigToClient(cfg)
	if err != nil {
		log.Panicf("CreateNodeClient() failed: %s", err)
	}
	conn, err := redis.NewBytesConnection(client, log)
	if err != nil {
		safeclose.Close(client)
		log.Panicf("NewBytesConnection() failed: %s", err)
	}
	return conn
}

func runSimpleExmple() {
	var err error

	keyPrefix := "key"
	keys3 := []string{
		keyPrefix + "1",
		keyPrefix + "2",
		keyPrefix + "3",
	}

	respChan := make(chan keyval.BytesWatchResp, 10)
	err = watcher.Watch(keyval.ToChan(respChan), make(chan string), keyPrefix)
	if err != nil {
		log.Error(err.Error())
	}
	go func() {
		for {
			select {
			case r, ok := <-respChan:
				if ok {
					switch r.GetChangeType() {
					case datasync.Put:
						log.Infof("KeyValProtoWatcher received %v: %s=%s", r.GetChangeType(), r.GetKey(), string(r.GetValue()))
					case datasync.Delete:
						log.Infof("KeyValProtoWatcher received %v: %s", r.GetChangeType(), r.GetKey())
					}
				} else {
					log.Error("Something wrong with respChan... bail out")
					return
				}
			default:
				break
			}
		}
	}()
	time.Sleep(2 * time.Second)
	put(keys3[0], "val 1")
	put(keys3[1], "val 2")
	put(keys3[2], "val 3", datasync.WithTTL(time.Second))

	time.Sleep(2 * time.Second)
	get(keys3[0])
	get(keys3[1])
	fmt.Printf("==> NOTE: %s should have expired\n", keys3[2])
	get(keys3[2]) // key3 should've expired
	fmt.Printf("==> NOTE: get(%s) should return false\n", keyPrefix)
	get(keyPrefix) // keyPrefix shouldn't find anything
	listKeys(keyPrefix)
	listVal(keyPrefix)

	doKeyInterator()
	doKeyValInterator()

	del(keyPrefix, datasync.WithPrefix())

	fmt.Println("==> NOTE: All keys should have been deleted")
	get(keys3[0])
	get(keys3[1])
	listKeys(keyPrefix)
	listVal(keyPrefix)

	txn(keyPrefix)

	log.Info("Sleep for 5 seconds")
	time.Sleep(5 * time.Second)

	// Done watching.  Close the channel.
	log.Infof("Closing connection")
	//close(respChan)
	safeclose.Close(redisConn)

	fmt.Println("==> NOTE: Call on a closed connection should fail.")
	del(keyPrefix)

	log.Info("Sleep for 10 seconds")
	time.Sleep(30 * time.Second)
}

func put(key, value string, opts ...datasync.PutOption) {
	err := broker.Put(key, []byte(value), opts...)
	if err != nil {
		//log.Panicf(err.Error())
		log.Error(err.Error())
	}
}

func get(key string) {
	var val []byte
	var found bool
	var revision int64
	var err error

	val, found, revision, err = broker.GetValue(key)
	if err != nil {
		log.Error(err.Error())
	} else if found {
		log.Infof("GetValue(%s) = %t ; val = %s ; revision = %d", key, found, val, revision)
	} else {
		log.Infof("GetValue(%s) = %t", key, found)
	}
}

func listKeys(keyPrefix string) {
	var keys keyval.BytesKeyIterator
	var err error

	keys, err = broker.ListKeys(keyPrefix)
	if err != nil {
		log.Error(err.Error())
	} else {
		var count int32
		for {
			key, rev, done := keys.GetNext()
			if done {
				break
			}
			log.Infof("ListKeys(%s):  %s (rev %d)", keyPrefix, key, rev)
			count++
		}
		log.Infof("ListKeys(%s): count = %d", keyPrefix, count)
	}
}

func listVal(keyPrefix string) {
	var keyVals keyval.BytesKeyValIterator
	var err error

	keyVals, err = broker.ListValues(keyPrefix)
	if err != nil {
		log.Error(err.Error())
	} else {
		var count int32
		for {
			kv, done := keyVals.GetNext()
			if done {
				break
			}
			log.Infof("ListValues(%s):  %s = %s (rev %d)", keyPrefix, kv.GetKey(), kv.GetValue(), kv.GetRevision())
			count++
		}
		log.Infof("ListValues(%s): count = %d", keyPrefix, count)
	}
}

func doKeyInterator() {
	prefix := "k_iter-"
	max := 100
	for i := 1; i <= max; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		broker.Put(key, []byte(key))
	}
	var level logging.LogLevel
	if debugIterator {
		level = log.GetLevel()
		log.SetLevel(logging.DebugLevel)
	}
	iterator, err := broker.ListKeys(prefix)
	if err != nil {
		log.Error(err.Error())
	}
	count := 0
	for {
		_, _, last := iterator.GetNext()
		if last {
			if count == max {
				log.Infof("doKeyInterator(): Expected %d keys; Found %d", max, count)
			} else {
				log.Errorf("doKeyInterator(): Expected %d keys; Found %d", max, count)
			}
			break
		}
		if debug || debugIterator {
			time.Sleep(200 * time.Millisecond)
		}
		count++
	}
	if debugIterator {
		log.SetLevel(level)
	}
	broker.Delete(prefix, datasync.WithPrefix())
}

func doKeyValInterator() {
	prefix := "kv_iter-"
	max := 100
	for i := 1; i <= max; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		broker.Put(key, []byte(key))
	}
	var level logging.LogLevel
	if debugIterator {
		level = log.GetLevel()
		log.SetLevel(logging.DebugLevel)
	}
	iterator, err := broker.ListValues(prefix)
	if err != nil {
		log.Error(err.Error())
	}
	count := 0
	for {
		_, last := iterator.GetNext()
		if last {
			if count == max {
				log.Infof("doKeyValInterator(): Expected %d keyVals; Found %d", max, count)
			} else {
				log.Errorf("doKeyValInterator(): Expected %d keyVals; Found %d", max, count)
			}
			break
		}
		if debug || debugIterator {
			time.Sleep(200 * time.Millisecond)
		}
		count++
	}
	if debugIterator {
		log.SetLevel(level)
	}
	broker.Delete(prefix, datasync.WithPrefix())
}

func del(keyPrefix string, opt ...datasync.DelOption) {
	var found bool
	var err error

	found, err = broker.Delete(keyPrefix, opt...)
	if err != nil {
		log.Error(err.Error())
		return
	}
	log.Infof("Delete(%s): found = %t", keyPrefix, found)
}

func txn(keyPrefix string) {
	keys := []string{
		keyPrefix + "101",
		keyPrefix + "102",
		keyPrefix + "103",
		keyPrefix + "104",
	}
	var txn keyval.BytesTxn

	log.Infof("txn(): keys = %v", keys)
	txn = broker.NewTxn()
	for i, k := range keys {
		txn.Put(k, []byte(strconv.Itoa(i+1)))
	}
	txn.Delete(keys[0])
	err := txn.Commit()
	if err != nil {
		log.Errorf("txn(): %s", err)
	}
	listVal(keyPrefix)
}

func generateSampleConfigs() {
	clientConfig := redis.ClientConfig{
		Password:     "",
		DialTimeout:  0,
		ReadTimeout:  0,
		WriteTimeout: 0,
		Pool: redis.PoolConfig{
			PoolSize:           0,
			PoolTimeout:        0,
			IdleTimeout:        0,
			IdleCheckFrequency: 0,
		},
	}
	var cfg interface{}

	cfg = redis.NodeConfig{
		Endpoint: "localhost:6379",
		DB:       0,
		EnableReadQueryOnSlave: false,
		TLS:          redis.TLS{},
		ClientConfig: clientConfig,
	}
	config.SaveConfigToYamlFile(cfg, "./node-client.yaml", 0644, makeTypeHeader(cfg))

	cfg = redis.SentinelConfig{
		Endpoints:    []string{"172.17.0.7:26379", "172.17.0.8:26379", "172.17.0.9:26379"},
		MasterName:   "mymaster",
		DB:           0,
		ClientConfig: clientConfig,
	}
	config.SaveConfigToYamlFile(cfg, "./sentinel-client.yaml", 0644, makeTypeHeader(cfg))

	cfg = redis.ClusterConfig{
		Endpoints:              []string{"172.17.0.1:6379", "172.17.0.2:6379", "172.17.0.3:6379"},
		EnableReadQueryOnSlave: true,
		MaxRedirects:           0,
		RouteByLatency:         true,
		ClientConfig:           clientConfig,
	}
	config.SaveConfigToYamlFile(cfg, "./cluster-client.yaml", 0644, makeTypeHeader(cfg))
}

func makeTypeHeader(i interface{}) string {
	t := reflect.TypeOf(i)
	tn := t.String()
	return fmt.Sprintf("# %s#%s", t.PkgPath(), tn[strings.Index(tn, ".")+1:])
}
