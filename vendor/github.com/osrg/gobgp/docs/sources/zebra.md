# FIB manipulation

This page explains how to perform FIB manipulation; kernel routing
table updates, interface lookups, and redistribution of routes between
different routing protocols. GoBGP uses zebra included in
[Quagga](http://www.nongnu.org/quagga/) or [FRRouting](https://frrouting.org/).

## Prerequisites

Assume you finished [Getting Started](getting-started.md)
and installing Quagga or FRRouting on the same host with GoBGP.

**Note:** For the integration with FRRouting, ONLY version 3.0.x is supported,
because the API (using Zebra protocol) of FRRouging is updated so fast and its
backward compatibility is not been kept.

## Contents

- [Configuration](#configuration)
- [Check routes from zebra](#check-routes-from-zebra)

## Configuration

You need to enable the zebra feature in the Global configuration as follows.

```toml
[zebra]
    [zebra.config]
        enabled = true
        url = "unix:/var/run/quagga/zserv.api"
        redistribute-route-type-list = ["connect"]
        version = 2
```

- `url` specifies the path to the unix domain socket or the TCP port for
  connecting to Zebra API.
  If omitted, GoBGP will use `"unix:/var/run/quagga/zserv.api"` by the default.
  Please note that with FRRouting, the path to the unix domain socket would be
  like `"unix:/var/run/frr/zserv.api"`.
  To specify the TCP port, `url` value would be like `"tcp:192.168.24.1:2600"`.

- `redistribute-route-type-list` specifies which route types you want to
  receive from Zebra daemon.
  For example, with `["connect"]`, GoBGP will receive the connected routes and
  redistribute them.

- `version` specifies Zebra API version.
  `2` is the version used by Quagga on Ubuntu 16.04 LTS.
  To enable the Next-Hop Tracking features, please specify `3` or later.
  For connecting to FRRouting, please specify `4`.

## Check Routes from zebra

Zebra has 3 connected routes in this example's environment.

- 172.16.1.100/30
- 172.16.6.100/30
- 192.168.31.0/24

Let's check these routes with GoBGP cli.

```bash
$ gobgp global rib
    Network              Next Hop             AS_PATH              Age        Attrs
*>  172.16.1.100/30      0.0.0.0                                   00:00:02   [{Origin: i} {Med: 1}]
*>  172.16.6.100/30      0.0.0.0                                   00:00:02   [{Origin: i} {Med: 1}]
*>  192.168.31.0/24      0.0.0.0                                   00:00:02   [{Origin: i} {Med: 1}]
```

You can see connected routes stored in the GoBGP global rib.
