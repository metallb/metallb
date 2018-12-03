# kafka-lib Multiplexer

A simple command line tool demonstrating the usage of Kafka multiplexer
API which allows to create multiple producers and consumers sharing
the minimal number of connections needed.

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

Quick-start:
```
go run main.go [-config=<config-file>]
```

By default, it is assumed that Kafka broker is running on 127.0.0.1:9092.
You may change the broker address or even specify multiple (clustered)
brokers in a YAML configuration file, selected with the CLI option
`-config`. See the attached `config` file with an example configuration.