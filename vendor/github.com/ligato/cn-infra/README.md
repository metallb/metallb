# CN-Infra

[![Build Status](https://travis-ci.org/ligato/cn-infra.svg?branch=master)](https://travis-ci.org/ligato/cn-infra)
[![Coverage Status](https://coveralls.io/repos/github/ligato/cn-infra/badge.svg?branch=master)](https://coveralls.io/github/ligato/cn-infra?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/ligato/cn-infra)](https://goreportcard.com/report/github.com/ligato/cn-infra)
[![GoDoc](https://godoc.org/github.com/ligato/cn-infra?status.svg)](https://godoc.org/github.com/ligato/cn-infra)
[![GitHub license](https://img.shields.io/badge/license-Apache%20license%202.0-blue.svg)](https://github.com/ligato/cn-infra/blob/master/LICENSE.md)

CN-Infra (cloud-native infrastructure) is a Golang platform for building
cloud-native microservices. Although it was originally intended for 
development/implementation of custom management/control plane agents
for [cloud-native Virtual Network Functions][4] (VNFs), it can be used to
develop any microservice. 

## Architecture

Each management/control plane app built on top of the CN-Infra platform is 
basically a set of modules called "plugins" in CN-Infra lingo, where each 
plugin provides a very specific/focused functionality. Some plugins are 
provided by the CN-Infra platform itself, some are written by the app's 
implementors. In other words, the CN-Infra platform itself is implemented
as a set of plugins that together provide the platform's functionality, 
such as logging, health checks, messaging (e.g. Kafka), a common front-end
API and back-end connectivity to various KV data stores (Etcd, Cassandra, 
Redis, ...), and REST and gRPC APIs. 

The architecture of the CN-Infra platform is shown in the following figure.

![arch](docs/imgs/high_level_arch_cninfra.png "High Level Architecture of cn-infra")

The CN-Infra platform consists of a **[Agent](agent)** that provides plugin
lifecycle management (initialization and graceful shutdown of plugins) 
and a set of platform plugins. Note that the figure shows not only 
CN-Infra plugins that are a part of the CN-Infra platform, but also 
app plugins that use the platform. CN-Infra platform plugins provide 
APIs that are consumed by app plugins. App plugins themselves may 
provide their own APIs consumed by external clients.

The platform is modular and extensible. Plugins supporting new functionality
(e.g. another KV store or another message bus) can be easily added to the
existing set of CN-Infra platform plugins. Moreover, CN-Infra based apps
can be built in layers: a set of app plugins together with CN-Infra plugins
can form a new platform providing APIs/services to higher layer apps. 
This approach was used in the [VPP Agent][3] - a management/control agent
for [VPP][2] based software data planes.,

Extending the code base does not mean that all plugins end up in all 
apps - app writers can pick and choose only those platform plugins that 
are required by their app; for example, if an app does not need a KV 
store, the CN-Infra platform KV data store plugins would not be included
in the app. All plugins used in an app are statically linked into the 
app.

## CN-Infra Plugins

A CN-Infra plugin is typically implemented as a library providing the 
plugin's functionality/APIs wrapped in a plugin wrapper. A CN-Infra 
library can also be used standalone in 3rd party apps that do not use
the CN-Infra platform. The plugin wrapper provides lifecycle management 
for the plugin component.

Platform plugins in the current CN-Infra release provide functionality
in one of the following functional areas:

* **RPC** - allows to expose application's API:
  - [GRPC](rpc/grpc) - handles GRPC requests and allows app plugins to define
    their own GRPC services
  - [REST](rpc/rest) - handles HTTP requests and allows app plugins to define
    their own REST APIs
  - [Prometheus](rpc/prometheus) - serves Prometheus metrics via HTTP and allows
    app plugins to register their own collectors
        
* **Data Stores** - provides a common data store API for app plugins (the 
    Data Broker) and back-end clients. The data store related plugins are:
  - [Consul](db/keyval/consul) - implements key-value plugin providing access to Consul
  - [Etcd](db/keyval/etcd) - implements key-value plugin providing access to Etcd
  - [Redis](db/keyval/redis) - implements key-value plugin providing access to Redis
  - [Casssandra](db/sql/cassandra) - implements sql plugin providing access to Cassandra
    
* **Messaging** - provides a common API and connectivity to message buses:
  - [Kafka](messaging/kafka) - provides access to a Kafka broker (Sarama)
    
* **Logging**:
  - [Logrus wrapper](logging/logrus) - implements logging skeleton 
    using the Logrus library. An app writer can create multiple loggers -
    for example, each app plugin can have its own logger. Log level
    for each logger can be controlled individually at run time through
    the Log Manager REST API.
  - [Log Manager](logging/logmanager) - allows the operator to set log
    level for each logger using a REST API.
    
* **Health** - Self health check mechanism between plugins 
    plus RPCs:
  - [StatusCheck](health/statuscheck) - allows to monitor the status of plugins
    and exposes it via HTTP
  - [Probe](health/probe) - callable remotely from K8s
  
* **Miscellaneous** - value-add plugins supporting the operation of a 
    CN-Infra based application: 
  - [Config](config) - helpers for loading plugin configuration.
  - [Datasync](datasync/resync) - provides data resynchronization after HA 
    events (restart or connectivity restoration after an outage) for data
    stores, gRPC and REST.
  - [IDX Map](idxmap) - reusable thread-safe map with advanced features:
    * multiple subscribers for watching changes in the map
    * secondary indexes
  - [ServiceLabel](servicelabel) - provides setting and retrieval of a 
      unique identifier for a CN-Infra based app. A cloud app typically needs
      a unique identifier so that it can differentiated from other instances 
      of the same app or from other apps (e.g. to have its own space in a kv 
      data store).
   
## Quickstart

The following code shows the initialization/start of a simple agent 
application built on the CN-Infra platform. The code for this example
can be found [here](examples/simple-agent/agent.go).
```
func main() {
	flavor := &rpc.FlavorRPC{}
	agent := core.NewAgent(flavor)

	err := core.EventLoopWithInterrupt(agent, nil)
	if err != nil {
		os.Exit(1)
	}
}
```

You can run this example code by using pre-build Docker images:

For quick start with the VPP Agent, you can use pre-build Docker images with the Agent and VPP
on [Dockerhub](https://hub.docker.com/r/ligato/dev-cn-infra/).

1. Run ETCD and Kafka on your host (e.g. in Docker 
  [using this procedure](examples/simple-agent/README.md)).

2. Run cn-infra example [simple-agent](examples/simple-agent/agent.go).
```
docker pull ligato/dev-cn-infra
docker run -it --name dev-cn-infra --rm ligato/dev-cn-infra
```

## Documentation

GoDoc can be browsed [online](https://godoc.org/github.com/ligato/cn-infra).

## Contributing

If you are interested in contributing, please see the [contribution guidelines](CONTRIBUTING.md).

[1]: https://12factor.net/
[2]: https://fd.io
[3]: https://github.com/ligato/vpp-agent
[4]: docs/readmes/cn_virtual_function.md
