# VPP Agent

[![Latest release](https://img.shields.io/github/release/ligato/vpp-agent.svg)](https://github.com/ligato/vpp-agent/releases/latest)
[![Build Status](https://travis-ci.org/ligato/vpp-agent.svg?branch=master)](https://travis-ci.org/ligato/vpp-agent)
[![Coverage Status](https://coveralls.io/repos/github/ligato/vpp-agent/badge.svg?branch=master)](https://coveralls.io/github/ligato/vpp-agent?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/ligato/vpp-agent)](https://goreportcard.com/report/github.com/ligato/vpp-agent)
[![GoDoc](https://godoc.org/github.com/ligato/vpp-agent?status.svg)](https://godoc.org/github.com/ligato/vpp-agent)
[![GitHub license](https://img.shields.io/badge/license-Apache%20license%202.0-blue.svg)](https://github.com/ligato/vpp-agent/blob/master/LICENSE)

###### Please note that the content of this repository is currently **WORK IN PROGRESS**.

The VPP Agent is a Go implementation of a control/management plane for
[VPP][1] based cloud-native [Virtual Network Functions][2] (VNFs). The VPP
Agent is built on top of the [CN-Infra platform][16] for developing 
cloud-native VNFs.

The VPP Agent can be used as-is as a management/control agent for VNFs 
based on off-the-shelf VPP (for example, a VPP-based vswitch), or as a
platform for developing customized VNFs with customized VPP-based data.

## Architecture

The VPP Agent is basically a set of VPP-specific plugins that use the 
CN-Infra platform to interact with other services/microservices in the
cloud (e.g. a KV data store, messaging, log warehouse, etc.). The VPP Agent
exposes VPP functionality to client apps via a higher-level model-driven 
API. Clients that consume this API may be either external (connecting to 
the VPP Agent via REST, gRPC API, Etcd or message bus transport), or local
Apps and/or Extension plugins running on the same CN-Infra platform in the 
same Linux process. 

The VNF Agent architecture is shown in the following figure: 

![vpp agent](docs/imgs/vpp_agent.png "VPP Agent & its Plugins on top of cn-infra")

Each (northbound) VPP API - L2, L3, ACL, ... - is implemented by a specific
VNF Agent plugin, which translates northbound API calls/operations into 
(southbound) low level VPP Binary API calls. Northbound APIs are defined 
using [protobufs][3], which allow for the same functionality to be accessible
over multiple transport protocols (HTTP, gRPC, Etcd, ...). Plugins use the 
[GoVPP library][4] to interact with the VPP.

The following figure shows the VPP Agent in context of a cloud-native VNF, 
where the VNF's data plane is implemented using VPP/DPDK and its management/control planes are implemented using the VNF agent:

![context](docs/imgs/context.png "VPP Agent & its Plugins on top of cn-infra")

## Plugins
 
The set of plugins in the VPP Agent is as follows:
* [Default VPP Plugins][5] - plugins providing northbound APIs to "default" 
  VPP functionality (i.e. VPP functions available "out-of-the-box"): 
  * [Interfaces][6] - network interface configuration (PCI Ethernet, MEMIF,
    AF_Packet, VXLAN, Loopback...) + BFD
  * [L2][7] - Bridge Domains, L2 cross-connects
  * [L3][8] - IP Routes, VRFs...
  * [ACL][9] - VPP access lists (VPP ACL plugin)
* [GOVPPmux][10] - plugin wrapper around GoVPP. Multiplexes plugins' access to
  VPP on a single connection.
* [Linux][11] (VETH) - allows optional configuration of Linux virtual ethernet 
  interfaces
* [CN-Infra datasync][12] - data synchronization after HA events
* [CN-Infra core][13] - lifecycle management of plugins (loading, 
  initialization, unloading)
* [RESTAPI](plugins/rest) - allows to run VPP CLI commands and also provides API to expose existing Northbound objects

## Tools

The VPP agent repository also contains tools for building and troubleshooting 
of VNFs based on the VPP Agent:

* [agentctl](cmd/agentctl) - a CLI tool that shows the state of a set of 
   VPP agents can configure the agents
* [vpp-agent-ctl](cmd/vpp-agent-ctl) - a utility for testing VNF Agent 
  configuration. It contains a set of pre-defined configurations that can 
  be sent to the VPP Agent either interactively or in a script. 
* [docker](docker) - container-based development environment for the VPP
  agent and for app/extension plugins.

## Quickstart

For a quick start with the VPP Agent, you can use pre-built Docker images with
the Agent and VPP on [Dockerhub][14] (or this for [ARM64][17]).

0. Start ETCD and Kafka on your host (e.g. in Docker as described [here][15]).
   Note: **The Agent in the pre-built Docker image will not start if it can't 
   connect to both Etcd and Kafka**.

   Note: **For ARM64 see the information for [kafka][18] and for [etcd][19]**.

1. Run VPP + VPP Agent in a Docker image:
```
docker pull ligato/vpp-agent
docker run -it --name vpp --rm ligato/vpp-agent
```

2. Configure the VPP agent using agentctl:
```
docker exec -it vpp agentctl -h
```

3. Check the configuration (using agentctl or directly using VPP console):
```
docker exec -it vpp agentctl -e 172.17.0.1:2379 show
docker exec -it vpp vppctl -s localhost:5002
```

## Documentation
GoDoc can be browsed [online](https://godoc.org/github.com/ligato/vpp-agent).

## Next Steps
Read the README for the [Development Docker Image](docker/dev/README.md) for more details.

#### Deployment:
[![K8s integration](docs/imgs/k8s_deployment_thumb.png "VPP Agent - K8s integration")](docs/Deployment.md)

#### Design & architecture:
[![VPP agent 10.000 feet](docs/imgs/vpp_agent_10K_feet_thumb.png "VPP Agent - 10.000 feet view on the architecture")](docs/Design.md)

## Contribution:
If you are interested in contributing, please see the [contribution guidelines](CONTRIBUTING.md).

[1]: https://fd.io/
[2]: https://github.com/ligato/cn-infra/blob/master/docs/readmes/cn_virtual_function.md
[3]: https://developers.google.com/protocol-buffers/
[4]: https://wiki.fd.io/view/GoVPP
[5]: plugins/vpp
[6]: plugins/vpp/ifplugin
[7]: plugins/vpp/l2plugin
[8]: plugins/vpp/l3plugin
[9]: plugins/vpp/aclplugin
[10]: plugins/govppmux
[11]: plugins/linux
[12]: https://github.com/ligato/cn-infra/tree/master/datasync
[13]: https://github.com/ligato/cn-infra/tree/master/core
[14]: https://hub.docker.com/r/ligato/vpp-agent/
[15]: docker/dev/README.md#running-etcd-server-on-local-host
[16]: https://github.com/ligato/cn-infra
[17]: https://hub.docker.com/r/ligato/vpp-agent-arm64/
[18]: docs/arm64/kafka.md
[19]: docs/arm64/etcd.md
