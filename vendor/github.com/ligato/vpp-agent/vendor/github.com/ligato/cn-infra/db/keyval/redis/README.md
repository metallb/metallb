Redis is the implementation of the key-value Data Broker client
API for the Redis key-value data store.
See [cn-infra/db/keyval](../../../db/keyval) for the definition
of the key-value Data Broker client API.

The entity BytesConnectionRedis provides access to CRUD as well as event
subscription API's.
```
   +-----+   (Broker)   +------------------------+ -->  CRUD      +-------+ -->
   | app |                   |  BytesConnectionRedis  |                 | Redis |
   +-----+    <-- (KeyValProtoWatcher)  +------------------------+  <--  events    +-------+
```

## How to use Redis
The code snippets below provide examples to help you get started.
For simplicity, error handling is omitted.

#### Need to import following dependencies
```
    import "github.com/ligato/cn-infra/db/keyval/kvproto"
    import "github.com/ligato/cn-infra/db/keyval/redis"
    import "github.com/ligato/cn-infra/utils/config"
    import "github.com/ligato/cn-infra/logging/logrus"
```
#### Define client configuration based on your Redis installation.
- Single Node
var cfg redis.NodeConfig
- Sentinel Enabled Cluster
var cfg redis.SentinelConfig
- Redis Cluster
var cfg redis.ClusterConfig
- See sample YAML configurations [(*.yaml files)](../../../examples/redis-lib)

You can initialize any of the above configuration instances in memory,
or load the settings from file using
```
   err = config.ParseConfigFromYamlFile(configFile, &cfg)
```
You can also load any of the three configuration files using
```
   var cfg interface{}
   cfg, err := redis.LoadConfig(configFile)
```
#### Create connection from configuration
```
   client, err := redis.CreateClient(cfg)
   db, err := redis.NewBytesConnection(client, logrus.DefaultLogger())
```
#### Create Brokers / Watchers from connection
```
   //create broker/watcher that share the same connection pools.
   bytesBroker := db.NewBroker("some-prefix")
   bytesWatcher := db.NewWatcher("some-prefix")

   // create broker/watcher that share the same connection pools,
   // capable of processing protocol-buffer generated data.
   wrapper := kvproto.NewProtoWrapper(db)
   protoBroker := wrapper.NewBroker("some-prefix")
   protoWatcher := wrapper.NewWatcher("some-prefix")
```
#### Perform CRUD operations
```
   // put
   err = db.Put("some-key", []byte("some-value"))
   err = db.Put("some-temp-key", []byte("valid for 20 seconds"),
                 datasync.WithTTL(20*time.Second))

   // get
   value, found, revision, err := db.GetValue("some-key")
   if found {
       ...
   }

   // Note: flight.Info implements proto.Message.
   f := flight.Info{
           Airline:  "UA",
           Number:   1573,
           Priority: 1,
        }
   err = protoBroker.Put("some-key-prefix", &f)
   f2 := flight.Info{}
   found, revision, err = protoBroker.GetValue("some-key-prefix", &f2)

   // list
   keyPrefix := "some"
   kv, err := db.ListValues(keyPrefix)
   for {
       kv, done := kv.GetNext()
       if done {
           break
       }
        key := kv.GetKey()
       value := kv.GetValue()
   }

   // delete
   found, err := db.Delete("some-key")
   // or, delete all keys matching the prefix "some-key".
   found, err := db.Delete("some-key", datasync.WithPrefix())

   // transaction
   var txn keyval.BytesTxn = db.NewTxn()
   txn.Put("key101", []byte("val 101")).Put("key102", []byte("val 102"))
   txn.Put("key103", []byte("val 103")).Put("key104", []byte("val 104"))
   err := txn.Commit()
```
#### Subscribe to key space events
```
   watchChan := make(chan keyval.BytesWatchResp, 10)
   err = db.Watch(watchChan, "some-key")
   for {
       select {
        case r := <-watchChan:
           switch r.GetChangeType() {
           case datasync.Put:
               log.Infof("KeyValProtoWatcher received %v: %s=%s", r.GetChangeType(),
                         r.GetKey(), string(r.GetValue()))
           case datasync.Delete:
               ...
           }
       ...
       }
  }
```
 NOTE: You must configure Redis for it to publish key space events.
```
   config SET notify-keyspace-events KA
```
See [EVENT NOTIFICATION](https://raw.githubusercontent.com/antirez/redis/3.2/redis.conf)
for more details.

You can find detailed examples in
- [simple](../../../examples/redis-lib/simple)
- [airport](../../../examples/redis-lib/airport)

#### Resiliency
Connection/read/write time-outs, failover, reconnection and recovery
are validated by running the airport example against a Redis Sentinel
Cluster. Redis nodes are paused selectively to simulate server down:
```
$ docker-compose ps
```
|Name   |Command  |State|Ports|
|-------|---------|-----|-----|
|dockerredissentinel_master_1 | docker-entrypoint.sh redis ... | Paused | 6379/tcp |
|dockerredissentinel_slave_1 | docker-entrypoint.sh redis ... | Up | 6379/tcp |
|dockerredissentinel_slave_2 | docker-entrypoint.sh redis ... | Up | 6379/tcp |
|dockerredissentinel_sentinel_1 | sentinel-entrypoint.sh | Up | 26379/tcp, 6379/tcp |
|dockerredissentinel_sentinel_2 | sentinel-entrypoint.sh | Up | 26379/tcp, 6379/tcp |
|dockerredissentinel_sentinel_3 | sentinel-entrypoint.sh | Up | 26379/tcp, 6379/tcp |

