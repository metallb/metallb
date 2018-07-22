# Peer Group

This page explains how to configure the Peer Group features.
With Peer Group, you can set the same configuration to multiple peers.

## Contents

- [Prerequisite](#prerequisite)
- [Configuration](#configuration)
- [Verification](#verification)

## Prerequisite

Assumed that you finished [Getting Started](getting-started.md).

## Configuration

Below is the configuration to create a peer group.

```toml
[[peer-groups]]
  [peer-groups.config]
    peer-group-name = "sample-group"
    peer-as = 65001
  [[peer-groups.afi-safis]]
    [peer-groups.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[peer-groups.afi-safis]]
    [peer-groups.afi-safis.config]
      afi-safi-name = "ipv4-flowspec"
```

The configurations in this peer group will be inherited to the neighbors which is the member of this peer group.
In addition, you can add additional configurations to each member.

Below is the configuration to create a neighbor which belongs this peer group.

```toml
[[neighbors]]
  [neighbors.config]
    neighbor-address = "172.40.1.3"
    peer-group = "sample-group"
  [neighbors.timers.config]
    hold-time = 99
```

This neighbor belongs to the peer group, so the peer-as is 65001, and ipv4-unicast and ipv4-flowspec are enabled.
Furthermore, an additional configuration is set, the hold timer is 99 secs.

## Verification

You can see the neighbor configuration inherits the peer group config by running `gobgp neighbor` command.

```shell
$ gobgp neighbor 172.40.1.3
BGP neighbor is 172.40.1.3, remote AS 65001
  BGP version 4, remote router ID 172.40.1.3
  BGP state = established, up for 00:00:05
  BGP OutQ = 0, Flops = 0
  Hold time is 99, keepalive interval is 33 seconds
  Configured hold time is 99, keepalive interval is 33 seconds

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
