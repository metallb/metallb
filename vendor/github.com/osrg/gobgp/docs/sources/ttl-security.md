# TTL Security

This page explains how to configure TTL Security in accordance with
[RFC3682](https://tools.ietf.org/html/rfc3682): The Generalized TTL Security
Mechanism (GTSM).

## Prerequisites

Assume you finished [Getting Started](getting-started.md).

## Contents

- [Configuration](#configuration)
- [Verification](#verification)

## Configuration

If the BGP neighbor "10.0.0.2" is directly connected and the "malicious" BGP
router is 2 hops away, you can block the connection from the malicious BGP
router with `ttl-min >= 254` in `[neighbors.ttl-security.config]` section.
If specify `ttl-min = 255`, this allows only directly connected neighbor, and
`ttl-min = 254` allows also the neighbor on 1 hop away.

```toml
[global.config]
router-id = "10.0.0.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
  [neighbors.ttl-security.config]
    enabled = true
    ttl-min = 255
```

**NOTE:** TTL Security feature is mututally exclusive with
[eBGP Multihop](ebgp-multihop.md).
These features cannot be configured for the same neighbor.

## Verification

With TTL Security configuration, GoBGP will set TTL of all BGP messages to
255 and set the minimal acceptable TTL to the given `ttl-min` value.
Then, with the above configuration, only directly connected neighbor
"10.0.0.2" is acceptable and the malicious BGP router will be blocked.

For the connection from the proper neighbor:

```bash
$ gobgpd -f gobgpd.toml
{"level":"info","msg":"gobgpd started","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Config","level":"info","msg":"Finished reading the config file","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"level":"info","msg":"Peer 10.0.0.2 is added","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Peer","level":"info","msg":"Add a peer configuration for:10.0.0.2","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Key":"10.0.0.2","State":"BGP_FSM_OPENCONFIRM","Topic":"Peer","level":"info","msg":"Peer Up","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
...(snip)...
```

```bash
$ tcpdump -i ethXX tcp -v
tcpdump: listening on ethXX, link-type EN10MB (Ethernet), capture size 262144 bytes
hh:mm:ss IP (tos 0x0, ttl 255, id 51126, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [S], cksum 0x7df2 (correct), seq 889149897, win 29200, options [mss 1460,sackOK,TS val 4431487 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [S.], cksum 0x8382 (incorrect -> 0x12ac), seq 2886345048, ack 889149898, win 28960, options [mss 1460,sackOK,TS val 4431487 ecr 4431487,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 51127, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [.], cksum 0x837a (incorrect -> 0xb260), ack 1, win 58, options [nop,nop,TS val 4431487 ecr 4431487], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 51128, offset 0, flags [DF], proto TCP (6), length 103)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [P.], cksum 0x83ad (incorrect -> 0x8860), seq 1:52, ack 1, win 58, options [nop,nop,TS val 4431487 ecr 4431487], length 51: BGP
    Open Message (1), length: 51
      Version 4, my AS 65002, Holdtime 90s, ID 2.2.2.2
      Optional parameters, length: 22
        Option Capabilities Advertisement (2), length: 20
          Route Refresh (2), length: 0
          Multiprotocol Extensions (1), length: 4
        AFI IPv4 (1), SAFI Unicast (1)
          Multiprotocol Extensions (1), length: 4
        AFI IPv6 (2), SAFI Unicast (1)
          32-Bit AS Number (65), length: 4
         4 Byte AS 65002
hh:mm:ss IP (tos 0x0, ttl 255, id 48934, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [.], cksum 0x837a (incorrect -> 0xb22e), ack 52, win 57, options [nop,nop,TS val 4431487 ecr 4431487], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 48935, offset 0, flags [DF], proto TCP (6), length 103)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [P.], cksum 0x83ad (incorrect -> 0x8b31), seq 1:52, ack 52, win 57, options [nop,nop,TS val 4431487 ecr 4431487], length 51: BGP
    Open Message (1), length: 51
      Version 4, my AS 65001, Holdtime 90s, ID 1.1.1.1
      Optional parameters, length: 22
        Option Capabilities Advertisement (2), length: 20
          Route Refresh (2), length: 0
          Multiprotocol Extensions (1), length: 4
        AFI IPv4 (1), SAFI Unicast (1)
          Multiprotocol Extensions (1), length: 4
        AFI IPv6 (2), SAFI Unicast (1)
          32-Bit AS Number (65), length: 4
         4 Byte AS 65001
hh:mm:ss IP (tos 0x0, ttl 255, id 51129, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [.], cksum 0x837a (incorrect -> 0xb1fa), ack 52, win 58, options [nop,nop,TS val 4431487 ecr 4431487], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 51131, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [.], cksum 0x837a (incorrect -> 0xb1ca), ack 71, win 58, options [nop,nop,TS val 4431497 ecr 4431487], length 0
...(snip)...
```

For the connection from the malicious BGP router:

```bash
$ gobgpd -f gobgpd.toml
{"level":"info","msg":"gobgpd started","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Config","level":"info","msg":"Finished reading the config file","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"level":"info","msg":"Peer 10.0.0.2 is added","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
{"Topic":"Peer","level":"info","msg":"Add a peer configuration for:10.0.0.2","time":"YYYY-MM-DDTHH:mm:ss+09:00"}
...(No connection)...
```

```bash
$ tcpdump -i ethXX tcp -v
tcpdump: listening on ethXX, link-type EN10MB (Ethernet), capture size 262144 bytes
hh:mm:ss IP (tos 0x0, ttl 253, id 396, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [S], cksum 0xf680 (correct), seq 1704340403, win 29200, options [mss 1460,sackOK,TS val 4270655 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [S.], cksum 0x8382 (incorrect -> 0x1e1a), seq 2916417775, ack 1704340404, win 28960, options [mss 1460,sackOK,TS val 4270656 ecr 4270655,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 253, id 397, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [S], cksum 0x8382 (incorrect -> 0xf586), seq 1704340403, win 29200, options [mss 1460,sackOK,TS val 4270905 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [S.], cksum 0x8382 (incorrect -> 0x1d21), seq 2916417775, ack 1704340404, win 28960, options [mss 1460,sackOK,TS val 4270905 ecr 4270655,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [S.], cksum 0x8382 (incorrect -> 0x1c27), seq 2916417775, ack 1704340404, win 28960, options [mss 1460,sackOK,TS val 4271155 ecr 4270655,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 253, id 398, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.2.xxx > 10.0.0.1.bgp: Flags [S], cksum 0x8382 (incorrect -> 0xf391), seq 1704340403, win 29200, options [mss 1460,sackOK,TS val 4271406 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 255, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.bgp > 10.0.0.2.xxx: Flags [S.], cksum 0x8382 (incorrect -> 0x1b2c), seq 2916417775, ack 1704340404, win 28960, options [mss 1460,sackOK,TS val 4271406 ecr 4270655,nop,wscale 9], length 0
...(snip)...
```
