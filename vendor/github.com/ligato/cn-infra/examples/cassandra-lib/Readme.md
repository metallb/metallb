# Simple Cassandra example

To start the example, you need to connect to a Cassandra database.
If you don't have one running, you can start the following docker image
on your localhost:

```
sudo docker run -p 9042:9042 --name cassandra01 -d cassandra:latest
```

In the example, the configuration for the connection to the cassandra 
is configured in a yaml-formatted config file. The config file is specified
as the first CLI argument:

```
go run main.go <config-file-name>
```

To run the example with the provided default configuration, type:

```
go run main.go ./client-config.yaml
```
