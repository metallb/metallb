# Ethernet VPN (EVPN)

This page explains an configuration for EVPN. Note that the feature is
still very experimental.

## Contents

- [BaGPipe](#bagpipe)
    - [Configuration](#configuration)
    - [Advertising EVPN route](#advertising-evpn-route)
- [YABGP](#yabgp)
    - [Configuration](#configuration-1)
    - [Advertising EVPN route](#advertising-evpn-route-1)

## BaGPipe

This example uses [BaGPipe](https://github.com/openstack/networking-bagpipe). GoBGP receives
routes from one BaGPipe peer and advertises it to another BaGPipe peer.

**NOTE:** The following supposes to use BaGPipe version "7.0.0".

### Configuration

Please note BaGPipe supports only iBGP.
So here supposes a topology that GoBGP is configured as Route Reflector.
Two BaGPipe peers are Route Reflector clients and not connected to each other.
Then the following example shows two OSS BGP implementations can interchange EVPN messages.

Topology:

```
           +------------+
           | GoBGP (RR) |
     +-----| AS 65000   |-----+
     |     | 10.0.0.254 |     |
     |     +------------+     |
     |                        |
   (iBGP)                  (iBGP)
     |                        |
+----------+            +----------+
| BaGPipe  |            | BaGPipe  |
| AS 65000 |            | AS 65000 |
| 10.0.0.1 |            | 10.0.0.2 |
+----------+            +----------+
```

The following shows the sample configuration for GoBGP.
The point is that "l2vpn-evpn" families to be advertised need to be specified.

GoBGP on "10.0.0.254": `gobgpd.toml`

```toml
[global.config]
  as = 65000
  router-id = "10.0.0.254"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.1"
    peer-as = 65000
  [neighbors.route-reflector.config]
    route-reflector-client = true
    route-reflector-cluster-id = "10.0.0.254"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l2vpn-evpn"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
    peer-as = 65000
  [neighbors.route-reflector.config]
    route-reflector-client = true
    route-reflector-cluster-id = "10.0.0.254"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l2vpn-evpn"
```

If you are not familiar with BaGPipe, the following shows our configuration files.

BaGPipe peer on "10.0.0.1": `/etc/bagpipe-bgp/bgp.conf`

```ini
[BGP]
local_address=10.0.0.1
peers=10.0.0.254
my_as=65000
enable_rtc=True

[API]
host=localhost
port=8082

[DATAPLANE_DRIVER_IPVPN]
dataplane_driver = DummyDataplaneDriver

[DATAPLANE_DRIVER_EVPN]
dataplane_driver = DummyDataplaneDriver
```

BaGPipe peer on "10.0.0.2": `/etc/bagpipe-bgp/bgp.conf`

```ini
[BGP]
local_address=10.0.0.2
peers=10.0.0.254
my_as=65000
enable_rtc=True

[API]
api_host=localhost
api_port=8082

[DATAPLANE_DRIVER_IPVPN]
dataplane_driver = DummyDataplaneDriver

[DATAPLANE_DRIVER_EVPN]
dataplane_driver = DummyDataplaneDriver
```

Then, run GoBGP and BaGPipe peers.

```bash
# GoBGP
$ gobgpd -f gobgpd.toml

# BaGPipe
# If bgp.conf does not locate on the default path, please specify the config file as following.
$ bagpipe-bgp --config-file /etc/bagpipe-bgp/bgp.conf
```

### Advertising EVPN route

As you expect, the RIBs at BaGPipe peer on "10.0.0.2" has nothing.

```bash
# BaGPipe peer on "10.0.0.2"
$ bagpipe-looking-glass bgp routes
l2vpn/evpn,*: -
ipv4/mpls-vpn,*: -
ipv4/rtc,*: -
ipv4/flow-vpn,*: -
```

Let's advertise EVPN routes from BaGPipe peer on "10.0.0.1".

```bash
# BaGPipe peer on "10.0.0.1"
$ bagpipe-rest-attach --attach --network-type evpn --port tap-dummy --mac 00:11:22:33:44:55 --ip 11.11.11.1 --gateway-ip 11.11.11.254 --rt 65000:77 --vni 100
request: {"import_rt": ["65000:77"], "lb_consistent_hash_order": 0, "vpn_type": "evpn", "vni": 100, "vpn_instance_id": "evpn-bagpipe-test", "ip_address": "11.11.11.1/24", "export_rt": ["65000:77"], "local_port": {"linuxif": "tap-dummy"}, "advertise_subnet": false, "attract_traffic": {}, "gateway_ip": "11.11.11.254", "mac_address": "00:11:22:33:44:55", "readvertise": null}
response: 200 null
```

Now the RIBs at GoBGP and BaGPipe peer "10.0.0.2" has the advertised routes. The route was interchanged via GoBGP peer.

```bash
# GoBGP
$ gobgp global rib -a evpn
   Network                                                                      Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:macadv][rd:10.0.0.1:118][etag:0][mac:00:11:22:33:44:55][ip:11.11.11.1] [1601]     10.0.0.1                                  hh:mm:ss   [{Origin: i} {LocalPref: 100} {Extcomms: [VXLAN], [65000:77]} [ESI: single-homed]]
*> [type:multicast][rd:10.0.0.1:118][etag:0][ip:10.0.0.1]            10.0.0.1                                  hh:mm:ss   [{Origin: i} {LocalPref: 100} {Extcomms: [VXLAN], [65000:77]} {Pmsi: type: ingress-repl, label: 1600, tunnel-id: 10.0.0.1}]

# BaGPipe peer on "10.0.0.2"
$ bagpipe-looking-glass bgp routes
l2vpn/evpn,*:
  * evpn:macadv::10.0.0.1:118:-:0:00:11:22:33:44:55/48:11.11.11.1: label [ 100 ]:
      attributes:
        originator-id: 10.0.0.1
        cluster-list: [ 10.0.0.254 ]
        extended-community: [ target:65000:77 encap:VXLAN ]
      next_hop: 10.0.0.1
      afi-safi: l2vpn/evpn
      source: BGP-10.0.0.254 (...)
      route_targets:
        * target:65000:77
  * evpn:multicast::10.0.0.1:118:0:10.0.0.1:
      attributes:
        cluster-list: [ 10.0.0.254 ]
        originator-id: 10.0.0.1
        pmsi-tunnel: pmsi:ingressreplication:-:100:10.0.0.1
        extended-community: [ target:65000:77 encap:VXLAN ]
      next_hop: 10.0.0.1
      afi-safi: l2vpn/evpn
      source: BGP-10.0.0.254 (...)
      route_targets:
        * target:65000:77
ipv4/mpls-vpn,*: -
ipv4/rtc,*: -
ipv4/flow-vpn,*: -
```

## YABGP

Just like the example using BaGPipe, this example uses [YABGP](https://github.com/smartbgp/yabgp).
GoBGP receives EVPN routes from one YABGP peer and re-advertises it to another YABGP peer.

**NOTE:** The following supposes to use YABGP version "0.3.1".

### Configuration

YABGP supports eBGP peering. The following example shows GoBGP and two YABGP peers are connected
with eBGP and GoBGP interchanges EVPN routes from one YABGP peer to another.

Topology:

```
           +------------+
           | GoBGP      |
     +-----| AS 65254   |-----+
     |     | 10.0.0.254 |     |
     |     +------------+     |
     |                        |
   (eBGP)                  (eBGP)
     |                        |
+----------+            +----------+
| YABGP    |            | YABGP    |
| AS 65001 |            | AS 65002 |
| 10.0.0.1 |            | 10.0.0.2 |
+----------+            +----------+
```

GoBGP on "10.0.0.254": `gobgpd.toml`

```toml
[global.config]
  as = 65254
  router-id = "10.0.0.254"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.1"
    peer-as = 65001
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l2vpn-evpn"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.0.2"
    peer-as = 65002
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l2vpn-evpn"
```

You can start YABGP with the following CLI options:

```bash
# YABGP peer on "10.0.0.1"
$ yabgpd --bgp-local_as=65001 --bgp-local_addr=10.0.0.1 --bgp-remote_addr=10.0.0.254 --bgp-remote_as=65254 --bgp-afi_safi=evpn

# YABGP peer on "10.0.0.2"
$ yabgpd --bgp-local_as=65002 --bgp-local_addr=10.0.0.2 --bgp-remote_addr=10.0.0.254 --bgp-remote_as=65254 --bgp-afi_safi=evpn
```

Then, you can see GoBGP can connect to two YABGP peers by using gobgp command:

``` bash
# GoBGP
$ gobgpd -f gobgpd.toml
...(snip)...

$ gobgp neighbor
Peer        AS  Up/Down State       |#Received  Accepted
10.0.0.1 65001 hh:mm:ss Establ      |        0         0
10.0.0.2 65002 hh:mm:ss Establ      |        0         0
```

### Advertising EVPN route

We can advertise EVPN routes from YABGP 10.0.0.1 through its [REST
API](http://yabgp.readthedocs.io/en/latest/restapi.html).
In the REST request, you need to specify the `Authorization` header is `admin/admin`, and the
`Content-Type` is `application/json`.

Request URL for sending UPDATE messages:

```
POST http://10.0.0.1:8801/v1/peer/10.0.0.254/send/update
```

We will run this API four times to advertise four EVPN route types.
The following example use "curl" command for sending POST request.

EVPN type 1:

```bash
curl -X POST -u admin:admin -H 'Content-Type: application/json' http://10.0.0.1:8801/v1/peer/10.0.0.254/send/update -d '{
    "attr": {
        "1": 0,
        "2": [],
        "5": 100,
        "14": {
            "afi_safi": [
                25,
                70
            ],
            "nexthop": "10.75.44.254",
            "nlri": [
                {
                    "type": 1,
                    "value": {
                        "esi": 0,
                        "eth_tag_id": 100,
                        "label": [
                            10
                        ],
                        "rd": "1.1.1.1:32867"
                    }
                }
            ]
        },
        "16": [
            [
                1537,
                0,
                500
            ]
        ]
    }
}'
```

EVPN type 2:

```bash
curl -X POST -u admin:admin -H 'Content-Type: application/json' http://10.0.0.1:8801/v1/peer/10.0.0.254/send/update -d '{
    "attr": {
        "1": 0,
        "2": [],
        "5": 100,
        "14": {
            "afi_safi": [
                25,
                70
            ],
            "nexthop": "10.75.44.254",
            "nlri": [
                {
                    "type": 2,
                    "value": {
                        "esi": 0,
                        "eth_tag_id": 108,
                        "ip": "11.11.11.1",
                        "label": [
                            0
                        ],
                        "mac": "00-11-22-33-44-55",
                        "rd": "172.17.0.3:2"
                    }
                }
            ]
        },
        "16": [
            [
                1536,
                1,
                500
            ]
        ]
    }
}'
```

EVPN type 3:

```bash
curl -X POST -u admin:admin -H 'Content-Type: application/json' http://10.0.0.1:8801/v1/peer/10.0.0.254/send/update -d '{
    "attr": {
        "1": 0,
        "2": [],
        "5": 100,
        "14": {
            "afi_safi": [
                25,
                70
            ],
            "nexthop": "10.75.44.254",
            "nlri": [
                {
                    "type": 3,
                    "value": {
                        "eth_tag_id": 100,
                        "ip": "192.168.0.1",
                        "rd": "172.16.0.1:5904"
                    }
                }
            ]
        }
    }
}'
```
EVPN type 4:

```bash
curl -X POST -u admin:admin -H 'Content-Type: application/json' http://10.0.0.1:8801/v1/peer/10.0.0.254/send/update -d '{
    "attr": {
        "1": 0,
        "2": [],
        "5": 100,
        "14": {
            "afi_safi": [
                25,
                70
            ],
            "nexthop": "10.75.44.254",
            "nlri": [
                {
                    "type": 4,
                    "value": {
                        "esi": 0,
                        "ip": "192.168.0.1",
                        "rd": "172.16.0.1:8888"
                    }
                }
            ]
        },
        "16": [
            [
                1538,
                "00-11-22-33-44-55"
            ]
        ]
    }
}'
```

GoBGP will receive these four routes and re-advertise them to YABGP peer on "10.0.0.2"

```bash
# GoBGP
$ gobgp global rib -a evpn
   Network                                                  Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:32867][esi:single-homed][etag:100] [161]      10.75.44.254                              hh:mm:ss   [{Extcomms: [esi-label: 8001]} {Origin: i} {LocalPref: 100}]
*> [type:esi][rd:172.16.0.1:8888][esi:single-homed][ip:192.168.0.1]            10.75.44.254                              hh:mm:ss   [{Extcomms: [es-import rt: 00:11:22:33:44:55]} {Origin: i} {LocalPref: 100}]
*> [type:macadv][rd:172.17.0.3:2][etag:108][mac:00:11:22:33:44:55][ip:11.11.11.1] [0]        10.75.44.254                              hh:mm:ss   [{Extcomms: [mac-mobility: 500, sticky]} {Origin: i} {LocalPref: 100} [ESI: single-homed]]
*> [type:multicast][rd:172.16.0.1:5904][etag:100][ip:192.168.0.1]            10.75.44.254                              hh:mm:ss   [{Origin: i} {LocalPref: 100}]
```

Then, check statistics of neighbors for confirming the number of re-advertised routes.

```bash
# GoBGP
$ gobgp neighbor
Peer        AS  Up/Down State       |#Received  Accepted
10.0.0.1 65001 hh:mm:ss Establ      |        4         4
10.0.0.2 65002 hh:mm:ss Establ      |        0         0

$ gobgp neighbor 10.0.0.2
BGP neighbor is 10.0.0.2, remote AS 65002
  BGP version 4, remote router ID 10.0.0.2
  BGP state = established, up for hh:mm:ss
  BGP OutQ = 0, Flops = 0
  Hold time is 90, keepalive interval is 30 seconds
  Configured hold time is 90, keepalive interval is 30 seconds

  Neighbor capabilities:
    multiprotocol:
        l2vpn-evpn:	advertised and received
    route-refresh:	advertised and received
    4-octet-as:	advertised and received
    enhanced-route-refresh:	received
    cisco-route-refresh:	received
  Message statistics:
                         Sent       Rcvd
    Opens:                  2          2
    Notifications:          0          0
    Updates:                4          0
    Keepalives:             2          2
    Route Refresh:          0          0
    Discarded:              0          0
    Total:                  8          4
  Route statistics:
    Advertised:             4
    Received:               0
    Accepted:               0
```
