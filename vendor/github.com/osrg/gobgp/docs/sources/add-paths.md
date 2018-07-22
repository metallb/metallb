# Additional Paths

This page explains how to configure BGP Additional Paths features on GoBGP.
GoBGP supports to advertise ADD-PATH capability according to
[RFC7911](https://tools.ietf.org/html/rfc7911) and advertise paths with
the "Advertise N Paths" mode described in
[draft-ietf-idr-add-paths-guidelines](https://tools.ietf.org/html/draft-ietf-idr-add-paths-guidelines).

## Prerequisites

Assumed that you finished [Getting Started](getting-started.md).

## Contents

- [Configuration](#configuration)
- [Verification](#verification)
  - [Example Topology and Configuration](#example-topology-and-configuration)
  - [Advertise Multiple Paths](#advertise-multiple-paths)

## Configuration

In order to advertise multiple paths to the specific neighbors, you need to
configure `[neighbors.add-paths.config]` section for each neighbor.
In the following example, `send-max = 8` means GoBGP will advertise up to 8
paths per prefix towards this neighbor and `receive = true` enables to
receive multiple paths from this neighbor.

```toml
[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
    [neighbors.add-paths.config]
      send-max = 8
      receive = true
```

Also, BGP Additional Paths features are configurable per AFI-SAFI and the per
AFI-SAFI configuration overrides the per neighbor configuration.
The following example enables BGP Additional Paths features for only IPv4
unicast family.

```toml
[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
  [neighbors.add-paths.config]
    receive = false
    send-max = 0
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
    [neighbors.afi-safis.add-paths.config]
      receive = true
      send-max = 8
```

## Verification

### Example Topology and Configuration

To test BGP Additional Paths features, this page supposes the following
topology.

```text
+----------+                    +----------+          +----------+
| r1       |                    | r2       |          | r3       |
| AS 65001 |  ADD-PATH enabled  | AS 65002 |          | AS 65003 |
| 10.0.0.1 |--------------------| 10.0.0.2 |----------| 10.0.0.3 |
+----------+                    +----------+          +----------+
                                     |
                                     |
                                     |
                                +----------+
                                | r4       |
                                | AS 65004 |
                                | 10.0.0.4 |
                                +----------+
```

Configuration on r1:

```toml
[global.config]
  as = 65001
  router-id = "10.0.0.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
    peer-as = 65002
  [neighbors.add-paths.config]
    receive = true
    send-max = 8
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
```

Configuration on r2:

```toml
[global.config]
  as = 65002
  router-id = "10.0.0.2"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.1"
    peer-as = 65001
  [neighbors.add-paths.config]
    receive = true
    send-max = 8
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.3"
    peer-as = 65003
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.4"
    peer-as = 65004
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
```

### Advertise Multiple Paths

Start GoBGP on r1, r2, r3 and r4, and confirm the establishment of each BGP
session.

e.g.:

```bash
r1> gobgpd -f gobgpd.toml
{"level":"info","msg":"gobgpd started","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Config","level":"info","msg":"Finished reading the config file","time":""YYYY-MM-DDTHH:mm:ss+09:00"}
{"level":"info","msg":"Peer 10.0.0.2 is added","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Peer","level":"info","msg":"Add a peer configuration for:10.0.0.2","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Key":"10.0.0.2","State":"BGP_FSM_OPENCONFIRM","Topic":"Peer","level":"info","msg":"Peer Up","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
```

Advertise a prefix "192.168.1.0/24" on r3 and r4.

```bash
r3> gobgp global rib -a ipv4 add 192.168.1.0/24
```

```bash
r4> gobgp global rib -a ipv4 add 192.168.1.0/24
```

Then confirm 2 paths (from r3 and r4) are advertised to r1 from r2.
In the following output shows the path with AS_PATH 65002 65003 (r3->r2->r1)
and the path with AS_PATH 65002 65004 (r4->r2->r1).

```bash
r1> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 192.168.1.0/24       10.0.0.2             65002 65003          HH:mm:ss   [{Origin: ?}]
*  192.168.1.0/24       10.0.0.2             65002 65004          HH:mm:ss   [{Origin: ?}]
```
