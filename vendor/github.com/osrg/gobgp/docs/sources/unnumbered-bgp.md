# Unnumbered BGP

BGP is not only for the Internet. Due to proven scalability and configuration
flexibility, large data center operators are using BGP for their data center
networking [[ietf-rtgwg-bgp-routing-large-dc](https://tools.ietf.org/html/rfc7938)].

In typical case, the topology of the network is CLOS network which can offer
multiple ECMP for ToR switches.
Each ToR switches run BGP daemon and peer to uplink switches connected with
P2P link.

In this case, since all switches are operated by single administrator and trusted,
we can skip tedious neighbor configurations like specifying neighbor address or
neighbor AS number by using unnumbered BGP feature.

Unnumbered BGP utilizes IPv6 link local address to automatically decide who
to connect. Also, when using unnumbered BGP, you don't need to specify neighbor AS number.
GoBGP will accept any AS number in the neighbor's open message.

## Prerequisites

To use unnumbered BGP feature, be sure the link between two BGP daemons is P2P
and IPv6 is enabled on interfaces connected to the link.

Also, check neighbor's IPv6 link local address is on the linux's neighbor table.

```bash
$ ip -6 neigh show
fe80::42:acff:fe11:5 dev eth0 lladdr 02:42:ac:11:00:05 REACHABLE
```

If neighbor's address doesn't exist, easiest way to fill the table is `ping6`.
Try the command below

```bash
$ ping6 -c 1 ff02::1%eth0
PING ff02::1%eth0 (ff02::1%eth0): 56 data bytes
64 bytes from fe80::42:acff:fe11:5%eth0: icmp_seq=0 ttl=64 time=0.312 ms
--- ff02::1%eth0 ping statistics ---
1 packets transmitted, 1 packets received, 0% packet loss
round-trip min/avg/max/stddev = 0.312/0.312/0.312/0.000 ms
```

More reliable method is to run [radvd](http://www.litech.org/radvd/) or
[zebra](http://www.nongnu.org/quagga/) to periodically send router
advertisement.

## Configuration via configuration file

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-interface = "eth0"
```

## Configuration via CLI

```bash
$ gobgp global as 64512 router-id 192.168.255.1
$ gobgp neighbor add interface eth0
$ gobgp neighbor eth0
BGP neighbor is fe80::42:acff:fe11:3%eth0, remote AS 65001
  BGP version 4, remote router ID 192.168.0.2
  BGP state = BGP_FSM_ESTABLISHED, up for 00:00:07
  BGP OutQ = 0, Flops = 0
  Hold time is 90, keepalive interval is 30 seconds
  Configured hold time is 90, keepalive interval is 30 seconds
  Neighbor capabilities:
    multi-protocol:
        ipv4-unicast:   advertised and received
        ipv6-unicast:   advertised and received
    route-refresh:      advertised and received
    extended-nexthop:   advertised and received
        Local:  nlri: ipv4-unicast, nexthop: ipv6
        Remote: nlri: ipv4-unicast, nexthop: ipv6
    four-octet-as:      advertised and received
  Message statistics:
                         Sent       Rcvd
    Opens:                  1          1
    Notifications:          0          0
    Updates:                1          0
    Keepalives:             1          1
    Route Refresh:          0          0
    Discarded:              0          0
    Total:                  3          2
  Route statistics:
    Advertised:             1
    Received:               0
    Accepted:               0
```
