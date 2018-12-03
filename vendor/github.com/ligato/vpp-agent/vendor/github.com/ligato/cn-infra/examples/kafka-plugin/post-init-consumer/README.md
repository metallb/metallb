# kafka-plugin Post-init consumer

A simple example demonstrating the usage of Kafka plugin API 
where consumer is started after the message producer.

### Requirements

To start the example you have to have Kafka broker running first.
Furthermore, the example assumes that each auto-created topic has
at least 3 partitions (`num.partitions` >= 3 in `server.properties`).

If you don't have Kafka installed locally, you can use the following
docker image in combination with a prepared `server.properties` config
file:
```
sudo docker create -p 2181:2181 -p 9092:9092 --name kafka --rm \
   --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 spotify/kafka
KAFKA_VERSION=$(docker inspect -f '{{ .Config.Env }}' kafka |  tr ' ' '\n' | grep KAFKA_VERSION | sed 's/^.*=//')
SCALA_VERSION=$(docker inspect -f '{{ .Config.Env }}' kafka |  tr ' ' '\n' | grep SCALA_VERSION | sed 's/^.*=//')
sudo docker cp server.properties kafka:/opt/kafka_${SCALA_VERSION}-${KAFKA_VERSION}/config/server.properties
sudo docker start kafka
```

It will bring up Kafka broker listening on port 9092 for client
communication.


### Usage

To run the example, type:
```
go run main.go deps.go [-kafka-config <config-filepath>]
```

If `kafka-config` is unspecified, the application will try to search
for `kafka.conf` in the current working directory.
If the configuration file cannot be loaded or is not defined, default 
configuration will be used.
