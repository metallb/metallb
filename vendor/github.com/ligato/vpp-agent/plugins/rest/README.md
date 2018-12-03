# REST API Plugin

The `restplugin` is a core Agent Plugin used to expose REST API for the following:
* Run VPP CLI commands
* Exposes existing Northbound objects
* Provides logging mechanism so that the VPPCLI command and response can be searched in elastic search

## VPP CLI commands
```
curl -H "Content-Type: application/json" -X POST -d '{"vppclicommand":"show interface"}' http://0.0.0.0:9191/
```

## Exposing existing Northbound objects

Here is the list of supported REST URLs. If configuration dump URL is used, the output is based on proto model
structure for given data type together with VPP-specific data which are not a part of the model (indexes for
interfaces or ACLs, internal names, etc.). Those data are in separate section labeled as `Meta`.

**Access lists**

URLs to obtain ACL IP/MACIP configuration are as follows.

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/acl/ip
curl GET http://0.0.0.0:9191/vpp/dump/v1/acl/macip 
```

**VPP Interfaces**

REST plugin exposes configured VPP interfaces, which can be show all together, or only interfaces
of specific type.
 
```
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/loopback
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/ethernet
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/memif
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/tap
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/vxlan
curl GET http://0.0.0.0:9191/vpp/dump/v1/interfaces/afpacket
``` 
 
**Linux Interfaces**

REST plugin exposes configured Linux interfaces. All configured interfaces are dumped, together
with all interfaces in default namespace 

```
curl GET https://0.0.0.0:9191/linux/dump/v1/interfaces
```
 
**BFD**

REST plugin allows to dump bidirectional forwarding detection sessions, authentication keys, 
or the whole configuration. 

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/bfd
curl GET http://0.0.0.0:9191/vpp/dump/v1/bfd/sessions
curl GET http://0.0.0.0:9191/vpp/dump/v1/bfd/authkeys
``` 

**NAT**

REST plugin allows to dump NAT44 global configuration, DNAT configuration or both of them together.
SNAT is currently not supported in the model, so REST dump is not available as well.

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/nat
curl GET http://0.0.0.0:9191/vpp/dump/v1/nat/global
curl GET http://0.0.0.0:9191/vpp/dump/v1/nat/dnat
``` 

**STN**

Steal the NIC feature REST API contains one uri returning the list of STN rules.

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/stn
``` 

**L2 plugin**

Support for bridge domains, FIBs and cross connects. It is also possible to get all 
the bridge domain IDs.

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/bdid
curl GET http://0.0.0.0:9191/vpp/dump/v1/bd
curl GET http://0.0.0.0:9191/vpp/dump/v1/fib
curl GET http://0.0.0.0:9191/vpp/dump/v1/xc
```

**L3 plugin**

ARPs, proxy ARP interfaces/ranges and static routes exposed via REST:

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/arps
curl GET http://0.0.0.0:9191/vpp/dump/v1/proxyarp/interfaces
curl GET http://0.0.0.0:9191/vpp/dump/v1/proxyarp/ranges
curl GET http://0.0.0.0:9191/vpp/dump/v1/routes
```

**Linux L3 plugin**

Linux ARP and linux routes exposed via REST:

```
curl GET http://0.0.0.0:9191/linux/dump/v1/arps
curl GET http://0.0.0.0:9191/linux/dump/v1/routes
```

**L4 plugin**

L4 plugin exposes session configuration:

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/sessions
```

**Telemetry**

REST allows to get all the telemetry data, or selective using specific key:

```
curl GET http://0.0.0.0:9191/vpp/dump/v1/telemetry
curl GET http://0.0.0.0:9191/vpp/dump/v1/telemetry/memory
curl GET http://0.0.0.0:9191/vpp/dump/v1/telemetry/runtime
curl GET http://0.0.0.0:9191/vpp/dump/v1/telemetry/nodecount
```

**CLI command**

Allows to use VPP CLI command via REST. Commands are defined as a map as following:

```
curl POST http://0.0.0.0:9191/vpp/command -d '{"vppclicommand":"<command>"}'
```

**Index**

REST to get index page. Configuration items are sorted by type (ifplugin, telemetry, etc.)

```
curl GET http://0.0.0.0:9191/
```

## Logging mechanism
The REST API request is logged to stdout. The log contains VPPCLI command and VPPCLI response. It is searchable in elastic search using "VPPCLI".
