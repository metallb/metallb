# Simple agent

This application demonstrates how easily a set of CN-infra based plugins
can be turned into an application.

### Usage

To run the example, simply type:
```
go run agent.go [-kafka-config <config-filepath>] [-etcd-config <config-filepath>] \
 [-cassandra-config <config-filepath>] [-redis-config <config-filepath>]
```

All four data sources (kafka, etcd, redis, cassandra) are optional.
If a particular config file path is left unspecified, the application
will first try to look for the default configuration filename
in the current working directory before skipping the initialization
of the associated plugin.