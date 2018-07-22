# eBGP Multihop

This page explains how to configure eBGP multihop feature when external
BGP (eBGP) peers are not directly connected and multiple IP hops away.

## Prerequisites

Assume you finished [Getting Started](getting-started.md).

## Contents

- [Configuration](#configuration)
- [Verification](#verification)

## Configuration

If eBGP neighbor "10.0.0.2" is 2 hops away, you need to configure
`[neighbors.ebgp-multihop.config]` with `multihop-ttl >= 3` in
`[[neighbors]]` section.

```toml
[global.config]
as = 65001
router-id = "10.0.0.1"

[[neighbors]]
  [neighbors.config]
    peer-as = 65002
    neighbor-address = "10.0.0.2"
  [neighbors.ebgp-multihop.config]
    enabled = true
    multihop-ttl = 3
```

**NOTE:** eBGP Multihop feature is mututally exclusive with
[TTL Security](ttl-security.md).
These features cannot be configured for the same neighbor.

## Verification

Without eBGP multihop configuration, the default TTL for eBGP session is 1,
and GoBGP cannot reach the neighbor on 2 hops away.

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
hh:mm:ss IP (tos 0x0, ttl 1, id 19110, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [S], cksum 0x8382 (incorrect -> 0x540e), seq 31213082, win 29200, options [mss 1460,sackOK,TS val 2231484 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 1, id 19111, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [S], cksum 0x8382 (incorrect -> 0x5314), seq 31213082, win 29200, options [mss 1460,sackOK,TS val 2231734 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 1, id 19112, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [S], cksum 0x8382 (incorrect -> 0x511f), seq 31213082, win 29200, options [mss 1460,sackOK,TS val 2232235 ecr 0,nop,wscale 9], length 0
...(snip)...
```

With eBGP multihop configuration, GoBGP will set the given TTL for eBGP
session and successfully connect to the neighbor on 2 hops away.

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
hh:mm:ss IP (tos 0x0, ttl 3, id 31155, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [S], cksum 0x8382 (incorrect -> 0x42a8), seq 3226540591, win 29200, options [mss 1460,sackOK,TS val 3302300 ecr 0,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 253, id 0, offset 0, flags [DF], proto TCP (6), length 60)
    10.0.0.2.bgp > 10.0.0.1.xxx: Flags [S.], cksum 0x5dd6 (correct), seq 2536172214, ack 3226540592, win 28960, options [mss 1460,sackOK,TS val 3302301 ecr 3302300,nop,wscale 9], length 0
hh:mm:ss IP (tos 0x0, ttl 3, id 31156, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [.], cksum 0x837a (incorrect -> 0xfd89), ack 1, win 58, options [nop,nop,TS val 3302301 ecr 3302301], length 0
hh:mm:ss IP (tos 0x0, ttl 3, id 31157, offset 0, flags [DF], proto TCP (6), length 103)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [P.], cksum 0x83ad (incorrect -> 0xd68c), seq 1:52, ack 1, win 58, options [nop,nop,TS val 3302301 ecr 3302301], length 51: BGP
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
hh:mm:ss IP (tos 0x0, ttl 1, id 35114, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.2.bgp > 10.0.0.1.xxx: Flags [.], cksum 0xfd57 (correct), ack 52, win 57, options [nop,nop,TS val 3302301 ecr 3302301], length 0
hh:mm:ss IP (tos 0x0, ttl 1, id 35115, offset 0, flags [DF], proto TCP (6), length 103)
    10.0.0.2.bgp > 10.0.0.1.xxx: Flags [P.], cksum 0xd357 (correct), seq 1:52, ack 52, win 57, options [nop,nop,TS val 3302301 ecr 3302301], length 51: BGP
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
hh:mm:ss IP (tos 0x0, ttl 3, id 31158, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.1.xxx > 10.0.0.2.bgp: Flags [.], cksum 0x837a (incorrect -> 0xfd23), ack 52, win 58, options [nop,nop,TS val 3302301 ecr 3302301], length 0
hh:mm:ss IP (tos 0x0, ttl 1, id 35117, offset 0, flags [DF], proto TCP (6), length 52)
    10.0.0.2.bgp > 10.0.0.1.xxx: Flags [.], cksum 0x837a (incorrect -> 0xfcf4), ack 71, win 57, options [nop,nop,TS val 3302311 ecr 3302301], length 0
...(snip)...
```
