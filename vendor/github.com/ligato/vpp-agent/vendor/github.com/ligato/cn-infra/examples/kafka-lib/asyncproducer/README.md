# kafka-lib Async-producer

A simple command line tool for sending messages to Kafka using
an asynchronous Kafka producer.

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
  `go run asyncproducer.go -brokers=localhost:9092`

- In order to configure addresses of kafka brokers, an environment
  variable `KAFKA_PEERS` can be used instead:
  ```
  export KAFKA_PEERS=kafka1:9092,kafka2:9092,kafka3:9092
  go run consumer.go --topics=test -groupid=mygroup
  ```

- By default, the producer will choose the destination partition based
  on the message hash. This can be overridden using the `-partitioner`
  argument, available options are
    - `manual`: destination partition selected by the user through
                the option `-partition`
    - `hash`: partition calculated using a hash function applied
              on the message key
    - `random`: randomly selected partition

- On startup, a prompt will be displayed:
  Enter command [quit|message]:
    - enter `quit` to exit
    - or enter `message` to send a message

- If a message is entered, then the following prompts will be displayed:
    - `enter topic`: enter the destination topic name
    - `enter message`: enter the message text
    - `enter key`: enter the message key or skip
    - `enter meta`: enter the message meta data or skip

- To terminate this producer press `ctrl-c`.
  The message `closing producer ...` will be displayed.

- When a message is successfully sent,
  `message sent successfully - <msg>` is displayed.

- When the message delivery fails,
  `message errored - <error>` is displayed.

- If `quit` is entered, the consumer will be closed
  and `ended successfully` displayed.

- To display all command line options, type:
  ```
  go run asyncproducer.go -help
  ```

### Options

- brokers
: A comma separated list of brokers in the Kafka cluster.
  Alternatively, you can set broker addresses via the `KAFKA_PEERS`
  environment variable.

- partitioner
: The partitioning scheme to use.
  Can be `hash`, `manual`, or `random`.
  Default is **hash**.

- partition
: The partition to produce to. Only used if `partitioner=manual`.
  If the partition is > -1, then the partitioner will be automatically
  set to **manual**.
 
- debug
: Turns-on debug logging.

- silent
: Turns-off printing the message's topic, partition, and offset
   to stdout.

