# CN-infra examples

The examples folder contains several executable examples (built from their 
respective `main.go` files) used to illustrate the cn-infra functionality. 
While most of the examples show a very simple use case, they still often
need to connect to ETCD/Redis and/or Kafka. Therefore, you need to have
instances of Etcd, Redis and Kafka running prior to starting examples.

Examples with the suffix `-lib` demonstrate the usage of CN-Infra APIs in
generic Go programs. You can simply import the CN-Infra library where the
API is declared into your program and start using the API.

Examples with the suffix `-plugin` demonstrate the usage of CN-Infra APIs
within the context of plugins. Plugins are the basic building blocks
of any given CN-Infra application.  The CN-Infra plugin framework
provides plugin initialization and graceful shutdown and supports
uniform dependency injection mechanism to manage dependencies between
plugins.

Current examples:
* **[cassandra-lib](cassandra-lib)** shows how to use the Cassandra data
  broker API to access the Cassandra database,
* **[datasync-plugin](datasync-plugin)** demonstrates the usage
  of the data synchronization APIs of the datasync package inside
  an example plugin,
* **[etcd-lib](etcd-lib)** shows how to use the ETCD data broker API
  to write data into ETCD and catch this change as an event by the watcher,
* **[flags-lib](flags-lib/main.go)** registers flags and shows their
  runtime values in an example plugin,
* **[kafka-lib](kafka-lib)** shows how to use the Kafka messaging library
  on a set of individual tools (sync and async producer, consumer, mux),
* **[kafka-plugin (hash-partitioner)](kafka-plugin/hash-partitioner/main.go)**
  contains a simple plugin which registers a Kafka consumer on specific
  topics and sends multiple test messages,
* **[kafka-plugin (manual-partitioner)](kafka-plugin/manual-partitioner/main.go)**
  contains a simple plugin which registers a Kafka consumer watching
  on specific topics/partitions/offsets and sends multiple test messages,
* **[logs-lib](logs-lib)** shows how to use the logger library and switch
  between the log levels,
* **[logs-plugin](logs-plugin)** shows how to use the logger library
  in a simple plugin,
* **[redis-lib](redis-lib)** contains several examples that use
  the Redis data broker API,
* **[model](model)** shows how to define a custom data model using
  Protocol Buffers and how to integrate it into an application,
* **[simple-agent](simple-agent)** demonstrates how easily a set of
  CN-infra based plugins can be turned into an application.