# kafka-lib Consumer

A simple command line tool for consuming Kafka topic and printing
the received messages to the stdout.

### Requirements

To start the example you have to have Kafka broker running first.
if you don't have it installed locally you can use the following docker
image.
```
sudo docker run -p 2181:2181 -p 9092:9092 --name kafka --rm \
   --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 spotify/kafka
```

It will bring up Kafka broker listening on port 9092 for client
communication (suitable for quickstart).

### Usage

- Quick-start:
  ```
  go run consumer.go -topics=test -groupid=mygroup -brokers=localhost:9092
  ```

- In order to configure addresses of kafka brokers, an environment
  variable `KAFKA_PEERS` can be used instead:
  ```
  export KAFKA_PEERS=kafka1:9092,kafka2:9092,kafka3:9092
  go run consumer.go --topics=test -groupid=mygroup
  ```

- In order to configure kafka brokers an environment variable can be used:
  ```
  export KAFKA_PEERS=kafka1:9092,kafka2:9092,kafka3:9092
  go run consumer.go --topics=test -groupid=mygroup
  ```

- You can specify the offset you want to start consuming at.
  It can be either `oldest` or `newest`. The default is `newest`.
  ```
  go run consumer.go -topics=test -groupid=mygroup -offset=oldest
  go run consumer.go -topics=test -groupid=mygroup -offset=newest
  ```

- You can specify the partition(s) you want to consume as a comma-separated
  list. The default is `all`.
  ```
  go run consumer.go -topic=test -groupid=mygroup -partitions=1,2,3
  ```

- To display all command line options, type:
  ```
  go run consumer.go -help
  ```