# Equal Cost Multipath Routing with Zebra

This page explains how GoBGP handles Equal Cost Multipath (ECMP) routes with
Zebra daemon included in [Quagga](http://www.nongnu.org/quagga/) or
[FRRouting](https://frrouting.org/).

## Prerequisites

Assume you finished [Getting Started](getting-started.md) and
[FIB manipulation](zebra.md).

## Contents

- [Configuration](#configuration)
- [Verification](#verification)

## Configuration

**Note:** Before constructing your environment, please confirm your Zebra is
built with "--enable-multipath=ARG" configure option. The APT packaged Quagga
on Ubuntu 16.04 is configured with this option as following.

```bash
$ /usr/lib/quagga/zebra --version
zebra version 0.99.24.1
Copyright 1996-2005 Kunihiro Ishiguro, et al.
configured with:
	--build=x86_64-linux-gnu ...(snip)... --enable-multipath=64 ...(snip)...
```

Here supposes the following topology and demonstrates two ECMP routes which
advertised from R2 and R3 are installed to R1's Kernel routing table via Zebra.

```text
R1: GoBGP + Zebra
R2: GoBGP
R3: GoBGP

    +-------------+                     +-------------+
    | R1          | .1/24         .2/24 | R2          |
    | ID: 1.1.1.1 |---------------------| ID: 2.2.2.2 |
    | AS: 65000   |   192.168.12.0/24   | AS: 65000   |
    +-------------+                     +-------------+
        | .1/24
        |
        | 192.168.13.0/24
        |
        | .3/24
    +-------------+
    | R3          |
    | ID: 3.3.3.3 |
    | AS: 65000   |
    +-------------+
```

To enables ECMP features at GoBGP on R1, please confirm "use-multiple-paths"
option is configured as following. With this option, GoBGP will redistribute BGP
multipath routes to Zebra and Zebra will install them into Kernel routing table.

```toml
# gobgpd.toml on R1

[global.config]
  as = 65000
  router-id = "1.1.1.1"

[global.use-multiple-paths.config]
  enabled = true

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.12.2"
    peer-as = 65000
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv6-unicast"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.13.3"
    peer-as = 65000
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv6-unicast"

[zebra.config]
  enabled = true
  url = "unix:/var/run/quagga/zserv.api"
  redistribute-route-type-list = ["connect"]
  version = 2
```

```toml
# gobgpd.toml on R2

[global.config]
  as = 65000
  router-id = "2.2.2.2"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.12.1"
    peer-as = 65000
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv6-unicast"
```

```toml
# gobgpd.toml on R3

[global.config]
  as = 65000
  router-id = "3.3.3.3"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "192.168.13.1"
    peer-as = 65000
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-unicast"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv6-unicast"
```

## Verification

When connections established between each routers, all routers have only
"connected" routes from Zebra on R1.

```bash
R1> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 1.1.1.1/32           0.0.0.0                                   00:00:00   [{Origin: i} {Med: 0}]
*> 192.168.12.0/24      0.0.0.0                                   00:00:00   [{Origin: i} {Med: 0}]
*> 192.168.13.0/24      0.0.0.0                                   00:00:00   [{Origin: i} {Med: 0}]

R2> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 1.1.1.1/32           192.168.12.1                              00:00:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 192.168.12.0/24      192.168.12.1                              00:00:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 192.168.13.0/24      192.168.12.1                              00:00:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
```

And only these routes are installed on R1's Kernel routing table.

```bash
R1> ip route
192.168.12.0/24 dev r1-eth1  proto kernel  scope link  src 192.168.12.1
192.168.13.0/24 dev r1-eth2  proto kernel  scope link  src 192.168.13.1
```

Then, let's add new routes destinated to "10.23.1.0/24" on R2 and R3 routes.
These routes should be treated as Multipath routes which have the same cost.

```bash
R2> gobgp global rib -a ipv4 add 10.23.1.0/24
R2> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 1.1.1.1/32           192.168.12.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 10.23.1.0/24         0.0.0.0                                   00:00:00   [{Origin: ?}]
*> 192.168.12.0/24      192.168.12.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 192.168.13.0/24      192.168.12.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]

R3> gobgp global rib -a ipv4 add 10.23.1.0/24
R3> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 1.1.1.1/32           192.168.13.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 10.23.1.0/24         0.0.0.0                                   00:00:00   [{Origin: ?}]
*> 192.168.12.0/24      192.168.13.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
*> 192.168.13.0/24      192.168.13.1                              00:10:00   [{Origin: i} {Med: 0} {LocalPref: 100}]
```

GoBGP on R1 will receive these routes and install them into R1's Kernel routing
table via Zebra. The following shows that traffic to "10.23.1.0/24" will be
forwarded through the interface r1-eth1 (nexthop is R2) or the interface r1-eth2
(nexthop is R3) with the same weight.

```bash
R1> gobgp global rib -a ipv4
   Network              Next Hop             AS_PATH              Age        Attrs
*> 1.1.1.1/32           0.0.0.0                                   00:15:00   [{Origin: i} {Med: 0}]
*> 10.23.1.0/24         192.168.12.2                              00:05:00   [{Origin: ?} {LocalPref: 100}]
*  10.23.1.0/24         192.168.13.3                              00:05:00   [{Origin: ?} {LocalPref: 100}]
*> 192.168.12.0/24      0.0.0.0                                   00:15:00   [{Origin: i} {Med: 0}]
*> 192.168.13.0/24      0.0.0.0                                   00:15:00   [{Origin: i} {Med: 0}]

R1> ip route
10.23.1.0/24  proto zebra
	nexthop via 192.168.12.2  dev r1-eth1 weight 1
	nexthop via 192.168.13.3  dev r1-eth2 weight 1
192.168.12.0/24 dev r1-eth1  proto kernel  scope link  src 192.168.12.1
192.168.13.0/24 dev r1-eth2  proto kernel  scope link  src 192.168.13.1
```
