# Ethernet VPN (EVPN)

This page explains an configuration for EVPN. Note that the feature is
still very experimental.

## Contents

- [CLI Syntax](#cli-syntax)
  - [Ethernet Segment Identifier](#ethernet-segment-identifier)
  - [Ethernet Auto-discovery Route](#ethernet-auto-discovery-route)
  - [MAC/IP Advertisement Route](#macip-advertisement-route)
  - [Inclusive Multicast Ethernet Tag Route](#inclusive-multicast-ethernet-tag-route)
  - [Ethernet Segment Route](#ethernet-segment-route)
  - [IP Prefix Route](#ip-prefix-route)
- [BaGPipe](#bagpipe)
  - [Configuration](#configuration)
  - [Advertising EVPN route](#advertising-evpn-route)
- [YABGP](#yabgp)
  - [Configuration](#configuration-1)
  - [Advertising EVPN route](#advertising-evpn-route-1)

## CLI Syntax

### Ethernet Segment Identifier

Some route types requires to specify Ethernet Segment Identifier (ESI) for its
argument. The supported ESI types and their formats are the following.

| Type | Format                                 | Description                                                                                                   |
| ---- | -------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| 0    | single-homed                           | Reserved keyword for arbitrary ESI type to denote a single-homed site.                                        |
| 0    | 0                                      | The same with "single-homed".                                                                                 |
| 0    | ARBITRARY \<Value>                     | Arbitrary ESI type with arbitrary value. Value should be colon separated hex values (similar to MAC address). |
| 1    | LACP \<MAC> \<Port Key>                | Type for LACP configured segment.                                                                             |
| 2    | MSTP \<MAC> \<Priority>                | Type for L2 bridge protocol (e.g., Multiple Spanning Tree Protocol) configured segment.                       |
| 3    | MAC \<MAC> \<Discriminator>            | Type for ESI based on MAC address.                                                                            |
| 4    | ROUTERID \<Router ID> \<Discriminator> | Type for ESI based on Router ID.                                                                              |
| 5    | AS \<AS> \<Discriminator>              | Type for ESI based on AS number.                                                                              |

### Example - Ethernet Segment Identifier

```bash
# single-homed
$ gobgp global rib -a evpn add a-d esi single-homed etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:single-homed][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# ARBITRARY <Value>
$ gobgp global rib -a evpn add a-d esi ARBITRARY 11:22:33:44:55:66:77:88:99 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                              Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_ARBITRARY | 11:22:33:44:55:66:77:88:99][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# LACP <MAC> <Port Key>
$ gobgp global rib -a evpn add a-d esi LACP aa:bb:cc:dd:ee:ff 10 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                                        Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_LACP | system mac aa:bb:cc:dd:ee:ff, port key 10][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# MSTP <MAC> <Priority>
$ gobgp global rib -a evpn add a-d esi MSTP aa:bb:cc:dd:ee:ff 10 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                                        Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_MSTP | bridge mac aa:bb:cc:dd:ee:ff, priority 10][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# MAC <MAC> <Discriminator>
$ gobgp global rib -a evpn add a-d esi MAC aa:bb:cc:dd:ee:ff 10 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                                                  Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_MAC | system mac aa:bb:cc:dd:ee:ff, local discriminator 10][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# ROUTERID <Router ID> <Discriminator>
$ gobgp global rib -a evpn add a-d esi ROUTERID 1.1.1.1 10 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                                            Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_ROUTERID | router id 1.1.1.1, local discriminator 10][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]

# AS <AS> <Discriminator>
$ gobgp global rib -a evpn add a-d esi AS 65000 10 etag 100 label 200 rd 1.1.1.1:100
$ gobgp global rib -a evpn
   Network                                                                             Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:100][esi:ESI_AS | as 65000, local discriminator 10][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]
```

### Ethernet Auto-discovery Route

```bash
# Add a route
$ gobgp global rib -a evpn add a-d esi <esi> etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [esi-label <esi-label> [single-active | all-active]]

# Show routes
$ gobgp global rib -a evpn [a-d]

# Delete route
$ gobgp global rib -a evpn del a-d esi <esi> etag <etag> label <label> rd <rd>
```

#### Example - Ethernet Auto-discovery Route

```bash
# Simple case
$ gobgp global rib -a evpn add a-d esi 0 etag 100 label 200 rd 1.1.1.1:65000
$ gobgp global rib -a evpn
   Network                                                  Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:65000][esi:single-homed][etag:100] [200]      0.0.0.0                                   00:00:00   [{Origin: ?}]
$ gobgp global rib -a evpn del a-d esi 0 etag 100 label 200 rd 1.1.1.1:65000

# With optionals
$ gobgp global rib -a evpn add a-d esi LACP aa:bb:cc:dd:ee:ff 100 etag 200 label 300 rd 1.1.1.1:65000 rt 65000:200 encap vxlan esi-label 400 single-active
$ gobgp global rib -a evpn a-d
   Network                                                                                           Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:A-D][rd:1.1.1.1:65000][esi:ESI_LACP | system mac aa:bb:cc:dd:ee:ff, port key 100][etag:200] [300]      0.0.0.0                                   00:00:00   [{Origin: ?} {Extcomms: [65000:200], [VXLAN], [esi-label: 400, single-active]}]
$ gobgp global rib -a evpn del a-d esi LACP aa:bb:cc:dd:ee:ff 100 etag 200 label 300 rd 1.1.1.1:65000
```

### MAC/IP Advertisement Route

```bash
# Add a route
$ gobgp global rib -a evpn add macadv <mac address> <ip address> [esi <esi>] etag <etag> label <label> rd <rd> [rt <rt>...] [encap <encap type>] [default-gateway]

# Show routes
$ gobgp global rib -a evpn [macadv]

# Delete route
$ gobgp global rib -a evpn del macadv <mac address> <ip address> [esi <esi>] etag <etag> label <label> rd <rd>
```

#### Example - MAC/IP Advertisement Route

```bash
# Simple case
$ gobgp global rib -a evpn add macadv aa:bb:cc:dd:ee:ff 10.0.0.1 etag 100 label 200,300 rd 1.1.1.1:65000
$ gobgp global rib -a evpn
   Network                                                                       Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:macadv][rd:1.1.1.1:65000][etag:100][mac:aa:bb:cc:dd:ee:ff][ip:10.0.0.1] [200,300]  0.0.0.0                                   00:00:00   [{Origin: ?} [ESI: single-homed]]
$ gobgp global rib -a evpn del macadv aa:bb:cc:dd:ee:ff 10.0.0.1 etag 100 label 200,300 rd 1.1.1.1:65000

# With optionals
$ gobgp global rib -a evpn add macadv aa:bb:cc:dd:ee:ff 10.0.0.1 esi AS 65000 100 etag 200 label 300 rd 1.1.1.1:65000 rt 65000:400 encap vxlan default-gateway
$ gobgp global rib -a evpn macadv
   Network                                                                       Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:macadv][rd:1.1.1.1:65000][etag:200][mac:aa:bb:cc:dd:ee:ff][ip:10.0.0.1] [300]      0.0.0.0                                   00:00:00   [{Origin: ?} {Extcomms: [65000:400], [VXLAN], [default-gateway]} [ESI: ESI_AS | as 65000, local discriminator 100]]
$ gobgp global rib -a evpn del macadv aa:bb:cc:dd:ee:ff 10.0.0.1 esi AS 65000 100 etag 200 label 300 rd 1.1.1.1:65000
```

### Inclusive Multicast Ethernet Tag Route

```bash
# Add a route
$ gobgp global rib -a evpn add multicast <ip address> etag <etag> rd <rd> [rt <rt>...] [encap <encap type>]

# Show routes
$ gobgp global rib -a evpn [multicast]

# Delete route
$ gobgp global rib -a evpn del multicast <ip address> etag <etag> rd <rd>
```

#### Example - Inclusive Multicast Ethernet Tag Route

```bash
# Simple case
$ gobgp global rib -a evpn add multicast 10.0.0.1 etag 100 rd 1.1.1.1:65000
$ gobgp global rib -a evpn
   Network                                                   Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:multicast][rd:1.1.1.1:65000][etag:100][ip:10.0.0.1]            0.0.0.0                                   00:00:00   [{Origin: ?}]
$ gobgp global rib -a evpn del multicast 10.0.0.1 etag 100 rd 1.1.1.1:65000

# With optionals
$ gobgp global rib -a evpn add multicast 10.0.0.1 etag 100 rd 1.1.1.1:65000 rt 65000:200 encap vxlan
$ gobgp global rib -a evpn multicast
   Network                                                   Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:multicast][rd:1.1.1.1:65000][etag:100][ip:10.0.0.1]            0.0.0.0                                   00:00:00   [{Origin: ?} {Extcomms: [65000:200], [VXLAN]}]
$ gobgp global rib -a evpn del multicast 10.0.0.1 etag 100 rd 1.1.1.1:65000
```

### Ethernet Segment Route

```bash
# Add a route
$ gobgp global rib -a evpn add esi <ip address> esi <esi> rd <rd> [rt <rt>...] [encap <encap type>]

# Show routes
$ gobgp global rib -a evpn [esi]

# Delete route
$ gobgp global rib -a evpn del esi <ip address> esi <esi> rd <rd>
```

#### Example - Ethernet Segment Route

```bash
# Simple case
$ gobgp global rib -a evpn add esi 10.0.0.1 esi 0 rd 1.1.1.1:65000
$ gobgp global rib -a evpn
   Network                                                     Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:esi][rd:1.1.1.1:65000][esi:single-homed][ip:10.0.0.1]            0.0.0.0                                   00:00:00   [{Origin: ?}]
$ gobgp global rib -a evpn del esi 10.0.0.1 esi 0 rd 1.1.1.1:65000

# With optionals
$ gobgp global rib -a evpn add esi 10.0.0.1 esi MAC aa:bb:cc:dd:ee:ff 100 rd 1.1.1.1:65000 rt 65000:200 encap vxlan
$ gobgp global rib -a evpn esi
   Network                                                                                                        Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:esi][rd:1.1.1.1:65000][esi:ESI_MAC | system mac aa:bb:cc:dd:ee:ff, local discriminator 100][ip:10.0.0.1]            0.0.0.0                                   00:00:00   [{Origin: ?} {Extcomms: [65000:200], [VXLAN], [es-import rt: aa:bb:cc:dd:ee:ff]}]
$ gobgp global rib -a evpn del esi 10.0.0.1 esi MAC aa:bb:cc:dd:ee:ff 100 rd 1.1.1.1:65000
```

### IP Prefix Route

```bash
# Add a route
$ gobgp global rib -a evpn add prefix <ip prefix> [gw <gateway>] [esi <esi>] etag <etag> [label <label>] rd <rd> [rt <rt>...] [encap <encap type>] [router-mac <mac address>]

# Show routes
$ gobgp global rib -a evpn [prefix]

# Delete route
$ gobgp global rib -a evpn del prefix <ip prefix> [gw <gateway>] [esi <esi>] etag <etag> [label <label>] rd <rd>
```

#### Example - IP Prefix Route

```bash
# Simple case
$ gobgp global rib -a evpn add prefix 10.0.0.0/24 etag 100 rd 1.1.1.1:65000
$ gobgp global rib -a evpn
   Network                                                       Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:Prefix][rd:1.1.1.1:65000][etag:100][prefix:10.0.0.0/24] [0]        0.0.0.0                                   00:00:00   [{Origin: ?} [ESI: single-homed] [GW: 0.0.0.0]]
$ gobgp global rib -a evpn del prefix 10.0.0.0/24 etag 100 rd 1.1.1.1:65000

# With optionals
$ gobgp global rib -a evpn add prefix 10.0.0.0/24 172.16.0.1 esi MSTP aa:aa:aa:aa:aa:aa 100 etag 200 label 300 rd 1.1.1.1:65000 rt 65000:200 encap vxlan router-mac bb:bb:bb:bb:bb:bb
$ gobgp global rib -a evpn prefix
   Network                                                       Labels     Next Hop             AS_PATH              Age        Attrs
*> [type:Prefix][rd:1.1.1.1:65000][etag:200][prefix:10.0.0.0/24] [300]      0.0.0.0                                   00:00:00   [{Origin: ?} {Extcomms: [65000:200], [VXLAN], [router's mac: bb:bb:bb:bb:bb:bb]} [ESI: ESI_MSTP | bridge mac aa:aa:aa:aa:aa:aa, priority 100] [GW: 0.0.0.0]]
$ gobgp global rib -a evpn del prefix 10.0.0.0/24 172.16.0.1 esi MSTP aa:aa:aa:aa:aa:aa 100 etag 200 label 300 rd 1.1.1.1:65000
```

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

**NOTE:** The following supposes to use YABGP version "0.4.0".

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
            "esi-label:0:500"
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
            "mac-mobility:1:500"
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
            "es-import:00-11-22-33-44-55"
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
