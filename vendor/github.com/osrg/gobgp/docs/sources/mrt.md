# MRT

This page explains how to play with GoBGP's MRT feature.

## Prerequisites

Assume you finished [Getting Started](getting-started.md).

## Contents

- [Inject routes from MRT table v2 records](#inject-routes-from-mrt-table-v2-records)
- [Dump updates in MRT BGP4MP format](#dump-updates-in-mrt-bgp4mp-format)
- [Dump the RIB in MRT TABLE_DUMPv2 format](#dump-the-rib-in-mrt-table_dumpv2-format)

## Inject routes from MRT table v2 records

Route injection can be done by

```bash
$ gobgp mrt inject global <dumpfile> [<number of prefix to inject>]
```

## Dump updates in MRT BGP4MP format

### Configuration

With the following configuration, gobgpd continuously dumps BGP update
messages to `/tmp/updates.dump` file in the BGP4MP format.

```toml
[[mrt-dump]]
  [mrt-dump.config]
    dump-type = "updates"
    file-name = "/tmp/updates.dump"
```

Also gobgpd supports log rotation; a new dump file is created
periodically, and the old file is renamed to a different name.  With
the following configuration, gobgpd creates a new dump file every 180
seconds such as `/tmp/20160510.1546.dump`. The format of a name can be
specified in golang's
[time](https://golang.org/pkg/time/#pkg-constants) package's format.

```toml
[[mrt-dump]]
  [mrt-dump.config]
    dump-type = "updates"
    file-name = "/tmp/log/20060102.1504.dump"
    rotation-interval = 180
```

## Dump the RIB in MRT TABLE_DUMPv2 format

### Configuration

With the following configuration, gobgpd continuously dumps routes in
the global rib to `/tmp/table.dump` file in the TABLE_DUMPv2 format
every 60 seconds.

```toml
[[mrt-dump]]
  [mrt-dump.config]
    dump-type = "table"
    file-name = "/tmp/table.dump"
    dump-interval = 60
```

With a route server configuration, gobgpd can dump routes in each
peer's RIB.

```toml
[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
  # ...(snip)...
  [neighbors.route-server.config]
    route-server-client = true

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.2"
  # ...(snip)...
  [neighbors.route-server.config]
    route-server-client = true

[[mrt-dump]]
  [mrt-dump.config]
    dump-type = "table"
    file-name = "/tmp/table-1.dump"
    table-name = "10.0.255.1"
    dump-interval = 60

[[mrt-dump]]
  [mrt-dump.config]
    dump-type = "table"
    file-name = "/tmp/table-2.dump"
    table-name = "10.0.255.2"
    dump-interval = 60
```
