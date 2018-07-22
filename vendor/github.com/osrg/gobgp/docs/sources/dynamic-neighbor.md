# Dynamic Neighbor

This page explains how to configure the Dynamic Neighbor features.
Dynamic Neighbor enables GoBGP to accept connections from the peers in specific prefix.

## Contents

- [Prerequisite](#prerequisite)
- [Configuration](#configuration)
- [Verification](#verification)

## Prerequisite

Assumed that you finished [Getting Started](getting-started.md) and learned [Peer Group](peer-group.md).

## Configuration

The Dynamic Neighbor feature requires a peer group setting for its configuration.

```toml
[global.config]
  as = 65001
  router-id = "172.40.1.2"

[[peer-groups]]
  [peer-groups.config]
    peer-group-name = "sample-group"
    peer-as = 65002
  [[peer-groups.afi-safis]]
    [peer-groups.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[peer-groups.afi-safis]]
    [peer-groups.afi-safis.config]
      afi-safi-name = "ipv4-flowspec"

[[dynamic-neighbors]]
  [dynamic-neighbors.config]
    prefix = "172.40.0.0/16"
    peer-group = "sample-group"
```

By this configuration, peers in `172.40.0.0/16` will be accepted by this GoBGP,
and the `sample-group` configuration is used as the configuration of members of this dynamic neighbor.

Note that GoBGP will be passive mode to members of dynamic neighbors.
So if both peers listen to each other as dynamic neighbors, the connection will never be established.

## Verification

Dynamic neighbors are not shown by `gobgp neighbor` command until the connection is established.

```shell
$ gobgp neighbor
Peer AS Up/Down State       |#Received  Accepted
```

After the connection is established, the neighbor will appear by `gobgp neighbor` command.
You can see the neighbor config is inherited from the peer group config.

```shell
$ gobgp neighbor
Peer          AS  Up/Down State       |#Received  Accepted
172.40.1.3 65001 00:00:23 Establ      |        0         0
$ gobgp neighbor 172.40.1.3
BGP neighbor is 172.40.1.3, remote AS 65002
  BGP version 4, remote router ID 172.40.1.3
  BGP state = established, up for 00:00:07
  BGP OutQ = 0, Flops = 0
  Hold time is 90, keepalive interval is 30 seconds
  Configured hold time is 90, keepalive interval is 30 seconds

  Neighbor capabilities:
    multiprotocol:
        ipv4-unicast:	advertised and received
        ipv4-flowspec:	advertised and received
    route-refresh:	advertised and received
    4-octet-as:	advertised and received
  Message statistics:
                         Sent       Rcvd
    Opens:                  1          1
    Notifications:          0          0
    Updates:                0          0
    Keepalives:             1          1
    Route Refresh:          0          0
    Discarded:              0          0
    Total:                  2          2
  Route statistics:
    Advertised:             0
    Received:               0
    Accepted:               0
```