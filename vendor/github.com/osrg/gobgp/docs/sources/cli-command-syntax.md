# CLI command syntax

This page explains gobgp client command syntax.



## basic command pattern
gobgp \<subcommand> \<object>  opts...

gobgp has six subcommands.
- [global](#global)
- [neighbor](#neighbor)
- [policy](#policy)
- [vrf](#vrf)
- [monitor](#monitor)
- [mrt](#mrt)


## 1. <a name="global"> global subcommand
### 1.1 Global Configuration
#### syntax
```shell
# configure global setting and start acting as bgp daemon
% gobgp global as <VALUE> router-id <VALUE> [listen-port <VALUE>] [listen-addresses <VALUE>...] [mpls-label-min <VALUE>] [mpls-label-max <VALUE>]
# delete global setting and stop acting as bgp daemon (all peer sessions will be closed)
% gobgp global del all
# show global setting
% gobgp global
```

### 1.2. Operations for Global-Rib - add/del/show -
#### - syntax
```shell
# add Route
% gobgp global rib add <prefix> [-a <address family>]
# delete a specific Route
% gobgp global rib del <prefix> [-a <address family>]
# delete all locally generated routes
% gobgp global rib del all [-a <address family>]
# show all Route information
% gobgp global rib [-a <address family>]
# show a specific route information
% gobgp global rib [<prefix>|<host>] [longer-prefixes|shorter-prefixes] [-a <address family>]
# show table summary
% gobgp global rib summary [-a <address family>]
```

#### - example
If you want to add routes with the address of the ipv4 to global rib：
```shell
% gobgp global rib add 10.33.0.0/16 -a ipv4
```
If you want to remove routes with the address of the ipv6 from global rib：
```shell
% gobgp global rib del 2001:123:123:1::/64 -a ipv6
```

#### more examples
```shell
% gobgp global rib add -a ipv4 10.0.0.0/24 origin igp
% gobgp global rib add -a ipv4 10.0.0.0/24 origin egp
% gobgp global rib add -a ipv4 10.0.0.0/24 aspath 10,20,100.100
% gobgp global rib add -a ipv4 10.0.0.0/24 aspath "10 20 {30,40} 50"
% gobgp global rib add -a ipv4 10.0.0.0/24 nexthop 20.20.20.20
% gobgp global rib add -a ipv4 10.0.0.0/24 med 10
% gobgp global rib add -a ipv4 10.0.0.0/24 local-pref 110
% gobgp global rib add -a ipv4 10.0.0.0/24 community 100:100
% gobgp global rib add -a ipv4 10.0.0.0/24 community 100:100,200:200
% gobgp global rib add -a ipv4 10.0.0.0/24 community no-export
% gobgp global rib add -a ipv4 10.0.0.0/24 community blackhole
% gobgp global rib add -a ipv4 10.0.0.0/24 aigp metric 200
% gobgp global rib add -a ipv4 10.0.0.0/24 large-community 100:100:100
% gobgp global rib add -a ipv4 10.0.0.0/24 large-community 100:100:100,200:200:200
% gobgp global rib add -a ipv4 10.0.0.0/24 identifier 10
% gobgp global rib add -a ipv4-mpls 10.0.0.0/24 100
% gobgp global rib add -a ipv4-mpls 10.0.0.0/24 100/200
% gobgp global rib add -a ipv4-mpls 10.0.0.0/24 100 nexthop 20.20.20.20
% gobgp global rib add -a ipv4-mpls 10.0.0.0/24 100 med 10
% gobgp global rib add -a vpnv4 10.0.0.0/24 label 10 rd 100:100
% gobgp global rib add -a vpnv4 10.0.0.0/24 label 10 rd 100.100:100
% gobgp global rib add -a vpnv4 10.0.0.0/24 label 10 rd 10.10.10.10:100
% gobgp global rib add -a vpnv4 10.0.0.0/24 label 10 rd 100:100 rt 100:200
% gobgp global rib add -a opaque key hello value world
```

#### - option
The following options can be specified in the global subcommand:

| short  |long           | description                                | default |
|--------|---------------|--------------------------------------------|---------|
|a       |address-family |specify any one from among `ipv4`, `ipv6`, `vpnv4`, `vpnv6`, `ipv4-labeled`, `ipv6-labeled`, `evpn`, `encap`, `rtc`, `ipv4-flowspec`, `ipv6-flowspec`, `l2vpn-flowspec`, `opaque` | `ipv4` |

Also, refer to the following for the detail syntax of each address family.

- `evpn` address family: [CLI Syntax for EVPN](evpn.md#cli-syntax)
- `*-flowspec` address family: [CLI Syntax for Flow Specification](flowspec.md#cli-syntax)

## 2. <a name="neighbor"> neighbor subcommand
### 2.1. Show Neighbor Status
#### - syntax
```shell
# show neighbor's status as list
% gobgp neighbor
# show status of a specific neighbor
% gobgp neighbor <neighbor address>
```

### 2.2. Operations for neighbor - shutdown/reset/softreset/enable/disable -
#### - syntax
```shell
# add neighbor
% gobgp neighbor add { <neighbor address> | interface <ifname> } as <as number> [ vrf <vrf-name> | route-reflector-client [<cluster-id>] | route-server-client | allow-own-as <num> | remove-private-as (all|replace) | replace-peer-as ]
# delete neighbor
% gobgp neighbor delete { <neighbor address> | interface <ifname> }
% gobgp neighbor <neighbor address> softreset [-a <address family>]
% gobgp neighbor <neighbor address> softresetin [-a <address family>]
% gobgp neighbor <neighbor address> softresetout [-a <address family>]
% gobgp neighbor <neighbor address> enable
% gobgp neighbor <neighbor address> disable
% gobgp neighbor <neighbor address> reset
```
#### - option
  The following options can be specified in the neighbor subcommand:

| short  |long           | description                                | default |
|--------|---------------|--------------------------------------------|---------|
|a       |address-family |specify any one from among `ipv4`, `ipv6`, `vpnv4`, `vpnv6`, `ipv4-labeled`, `ipv6-labeld`, `evpn`, `encap`, `rtc`, `ipv4-flowspec`, `ipv6-flowspec`, `l2vpn-flowspec`, `opaque` | `ipv4` |

### 2.3. Show Rib - local-rib/adj-rib-in/adj-rib-out -
#### - syntax
```shell
# show all routes in [local|adj-in|adj-out] table
% gobgp neighbor <neighbor address> [local|adj-in|adj-out] [-a <address family>]
# show a specific route in [local|adj-in|adj-out] table
% gobgp neighbor <neighbor address> [local|adj-in|adj-out] [<prefix>|<host>] [longer-prefixes|shorter-prefixes] [-a <address family>]
# show table summary
% gobgp neighbor <neighbor address> [local|adj-in|adj-out] summary [-a <address family>]
# show RPKI detailed information in adj-in table
% gobgp neighbor <neighbor address> adj-in <prefix> validation
```

#### - example
If you want to show the local rib of ipv4 that neighbor(10.0.0.1) has：
```shell
% gobgp neighbor 10.0.0.1 local -a ipv4
```

#### - option
The following options can be specified in the neighbor subcommand:

| short  |long           | description                                | default |
|--------|---------------|--------------------------------------------|---------|
|a       |address-family |specify any one from among `ipv4`, `ipv6`, `vpnv4`, `vpnv6`, `ipv4-labeled`, `ipv6-labeld`, `evpn`, `encap`, `rtc`, `ipv4-flowspec`, `ipv6-flowspec`, `l2vpn-flowspec`, `opaque` | `ipv4` |


### 2.4. Operations for Policy  - add/del/show -
#### Syntax
```shell
# show neighbor policy assignment
% gobgp neighbor <neighbor address> policy { in | import | export }
# add policies to specific neighbor policy
% gobgp neighbor <neighbor address> policy { in | import | export } add <policy name>... [default { accept | reject }]
# set policies to specific neighbor policy
% gobgp neighbor <neighbor address> policy { in | import | export } set <policy name>... [default { accept | reject }]
# remove attached policies from specific neighbor policy
% gobgp neighbor <neighbor address> policy { in | import | export } del <policy name>...
# remove all policies from specific neighbor policy
% gobgp neighbor <neighbor address> policy { in | import | export } del
```

#### Example
If you want to add the import policy to neighbor(10.0.0.1)：
```shell
% gobgp neighbor 10.0.0.1 policy import add policy1 policy2 default accept
```
You can specify multiple policy to neighbor separated by commas.

\<default policy action> means the operation(accept | reject) in the case where the route does not match the conditions of the policy.


<br>

## 3. <a name="policy"> policy subcommand
### 3.1. Operations for PrefixSet - add/del/show -
#### Syntax
```shell
# add PrefixSet
% gobgp policy prefix add <prefix set name> <prefix> [<mask length range>]
# delete a PrefixSet
% gobgp policy prefix del <prefix set name>
# delete a prefix from specific PrefixSet
% gobgp policy prefix del <prefix set name> <prefix> [<mask length range>]
# show all PrefixSet information
% gobgp policy prefix
# show a specific PrefixSet
% gobgp policy prefix <prefix set name>
```

#### Example
If you want to add the PrefixSet：
```shell
% gobgp policy prefix add ps1 10.33.0.0/16 16..24
```
A PrefixSet it is possible to have multiple prefix, if you want to remove the PrefixSet to specify only PrefixSet name.
```shell
% gobgp policy prefix del ps1
```
If you want to remove one element(prefix) of PrefixSet, to specify a prefix in addition to the PrefixSet name.
```shell
% gobgp policy prefix del ps1 10.33.0.0/16
```

### 3.2. Operations for NeighborSet - add/del/show -
#### Syntax
```shell
# add NeighborSet
% gobgp policy neighbor add <neighbor set name> <neighbor address/prefix>
# delete a NeighborSet
% gobgp policy neighbor del <neighbor set name>
# delete a neighbor from a NeighborSet
% gobgp policy neighbor del <neighbor set name> <address>
# show all NeighborSet information
% gobgp policy neighbor
# show a specific NeighborSet information
% gobgp policy neighbor <neighbor set name>
```

#### Example
If you want to add the NeighborSet：
```shell
% gobgp policy neighbor add ns1 10.0.0.1
```
You can also specify a neighbor address range with the prefix representation:
```shell
% gobgp policy neighbor add ns 10.0.0.0/24
``````
A NeighborSet is possible to have multiple address, if you want to remove the NeighborSet to specify only NeighborSet name.
```shell
% gobgp policy neighbor del ns1
```
If you want to remove one element(address) of NeighborSet, to specify a address in addition to the NeighborSet name.
```shell
% gobgp policy prefix del ns1 10.0.0.1
```

### 3.3. Operations for AsPathSet - add/del/show -
#### Syntax
```shell
# add AsPathSet
% gobgp policy as-path add <aspath set name> <as path>
# delete a specific AsPathSet
% gobgp policy as-path del <aspath set name>
# delete an as-path from a AsPathSet
% gobgp policy as-path del <aspath set name> <as path>
# show all AsPathSet information
% gobgp policy as-path
# show a specific AsPathSet information
% gobgp policy as-path <aspath set name>
```

#### Example
If you want to add the AsPathSet：
```shell
% gobgp policy as-path add ass1 ^65100
```

You can specify the position using regexp-like expression as follows:
- From: "^65100" means the route is passed from AS 65100 directly.
- Any: "65100" means the route comes through AS 65100.
- Origin: "65100$" means the route is originated by AS 65100.
- Only: "^65100$" means the route is originated by AS 65100 and comes from it directly.

Further you can specify the consecutive aspath and use regexp in each element as follows:
- ^65100_65001
- 65100_[0-9]+_.*$
- ^6[0-9]_5.*_65.?00$

An AsPathSet it is possible to have multiple as path, if you want to remove the AsPathSet to specify only AsPathSet name.
```shell
% gobgp policy as-path del ass1
```
If you want to remove one element(as path) of AsPathSet, to specify an as path in addition to the AsPathSet name.
```shell
% gobgp policy as-path del ass1 ^65100
```

### 3.4. Operations for CommunitySet - add/del/show -
#### Syntax
```shell
# add CommunitySet
% gobgp policy community add <community set name> <community>
# delete a specific CommunitySet
% gobgp policy community del <community set name>
# delete a community from a CommunitySet
% gobgp policy community del <community set name> <community>
# show all CommunitySet information
% gobgp policy community
# show a specific CommunitySet information
% gobgp policy community <community set name>
```

#### Example
If you want to add the CommunitySet：
```shell
% gobgp policy community add cs1 65100:10
```
   You can specify the position using regexp-like expression as follows:
   - 6[0-9]+:[0-9]+
   - ^[0-9]*:300$

A CommunitySet it is possible to have multiple community, if you want to remove the CommunitySet to specify only CommunitySet name.
```shell
% gobgp policy neighbor del cs1
```
If you want to remove one element(community) of CommunitySet, to specify a address in addition to the CommunitySet name.
```shell
% gobgp policy prefix del cs1 65100:10
```

### 3.5. Operations for ExtCommunitySet - add/del/show -
#### Syntax
```shell
# add ExtCommunitySet
% gobgp policy ext-community add <extended community set name> <extended community>
# delete a specific ExtCommunitySet
% gobgp policy ext-community del <extended community set name>
# delete a ext-community from a ExtCommunitySet
% gobgp policy ext-community del <extended community set name> <extended community>
# show all ExtCommunitySet information
% gobgp policy ext-community
# show a specific ExtCommunitySet information
% gobgp policy ext-community <extended community set name>
```

#### Example
If you want to add the ExtCommunitySet：
```shell
% gobgp policy ext-community add ecs1 RT:65100:10
```
Extended community set as \<SubType>:\<Global Admin>:\<LocalAdmin>.

If you read the [RFC4360](https://tools.ietf.org/html/rfc4360) and [RFC7153](https://tools.ietf.org/html/rfc7153), you can know more about Extended community.

You can specify the position using regexp-like expression as follows:
   - RT:[0-9]+:[0-9]+
   - SoO:10.0.10.10:[0-9]+

However, regular expressions for subtype can not be used, to use for the global admin and local admin.

A ExtCommunitySet it is possible to have multiple extended community, if you want to remove the ExtCommunitySet to specify only ExtCommunitySet name.
```shell
% gobgp policy neighbor del ecs1
```
If you want to remove one element(extended community) of ExtCommunitySet, to specify a address in addition to the ExtCommunitySet name.
```shell
% gobgp policy prefix del ecs1 RT:65100:10
```

### 3.6. Operations for LargeCommunitySet - add/del/show -
#### Syntax
```shell
# add LargeCommunitySet
% gobgp policy large-community add <set name> <large community>...
# delete a specific LargeCommunitySet
% gobgp policy large-community del <set name>
# delete a large-community from a LargeCommunitySet
% gobgp policy large-community del <set name> <large community>
# show all LargeCommunitySet information
% gobgp policy large-community
# show a specific LargeCommunitySet information
% gobgp policy large-community <set name>
```

#### Example
```shell
% gobgp policy large-community add l0 100:100:100
% gobgp policy large-community add l0 ^100:
% gobgp policy large-community add l0 :100$
% gobgp policy large-community del l0 100:100:100
% gobgp policy large-community add l0 200:100:100
% gobgp policy large-community
% gobgp policy large-community set l0 100:100:100 200:200:200 300:300:300
```

### 3.7 Statement Operation - add/del/show -
#### Syntax
```shell
# mod statement
% gobgp policy statement { add | del } <statement name>
# mod a condition to a statement
% gobgp policy statement <statement name> { add | del | set } condition { { prefix | neighbor | as-path | community | ext-community | large-community } <set name> [{ any | all | invert }] | as-path-length <len> { eq | ge | le } | rpki { valid | invalid | not-found } }
# mod an action to a statement
% gobgp policy statement <statement name> { add | del | set } action { reject | accept | { community | ext-community | large-community } { add | remove | replace } <value>... | med { add | sub | set } <value> | local-pref <value> | as-prepend { <asn> | last-as } <repeat-value> }
# show all statements
% gobgp policy statement
# show a specific statement
% gobgp policy statement <statement name>
```

### 3.8 Policy Operation - add/del/show -
#### Syntax
```shell
# mod policy
% gobgp policy { add | del | set } <policy name> [<statement name>...]
# show all policies
% gobgp policy
# show a specific policy
% gobgp policy <policy name>
```

## 4. <a name="vrf"> vrf subcommand
### 4.1 Add/Delete/Show VRF
#### Syntax
```shell
# add vrf
% gobgp vrf add <vrf name> rd <rd> rt {import|export|both} <rt>...
# del vrf
% gobgp vrf del <vrf name>
# show vrf
% gobgp vrf
```

#### Example
```shell
% gobgp vrf add vrf1 rd 10.100:100 rt both 10.100:100 import 10.100:101 export 10.100:102
% gobgp vrf
  Name                 RD                   Import RT                  Export RT
  vrf1                 10.100:100           10.100:100, 10.100:101     10.100:100, 10.100:102
% gobgp vrf del vrf1
% gobgp vrf
  Name                 RD                   Import RT            Export RT
```

### 4.2 Add/Delete/Show VRF routes
#### Syntax
```shell
# add routes to vrf
% gobgp vrf <vrf name> rib add <prefix> [-a <address family>]
# del routes from vrf
% gobgp vrf <vrf name> rib del <prefix> [-a <address family>]
# show routes in vrf
% gobgp vrf <vrf name> rib [-a <address family>]
```

#### Example
```shell
% gobgp vrf vrf1 rib add 10.0.0.0/24
% gobgp vrf vrf1 rib add 2001::/64 -a ipv6
% gobgp vrf vrf1 rib
  Network                Next Hop             AS_PATH              Age        Attrs
  10.100:100:10.0.0.0/24 0.0.0.0                                   00:00:40   [{Origin: i} {Extcomms: [10.100:100], [10.100:101]}]
% gobgp vrf vrf1 rib -a ipv6
  Network              Next Hop             AS_PATH              Age        Attrs
  10.100:100:2001::/64 ::                                        00:00:00   [{Origin: i} {Extcomms: [10.100:100], [10.100:101]}]
% gobgp vrf vrf1 rib del 10.0.0.0/24
% gobgp vrf vrf1 rib del 2001::/64
```

## 5. <a name="monitor"> monitor subcommand

### 5.1 monitor global rib

#### Syntax

```shell
# monitor global rib
% gobgp monitor global rib [-a <address family>] [--current]
```

#### Example

```shell
[TERM1]
% gobgp monitor global rib
[ROUTE] 10.0.0.0/24 via 0.0.0.0 aspath [] attrs [{Origin: i}]

[TERM2]
# monitor command blocks. add routes from another terminal
% gobgp global rib add 10.0.0.0/24
```

### 5.2 monitor neighbor status

#### Syntax

```shell
# monitor neighbor status
% gobgp monitor neighbor [--current]
# monitor specific neighbor status
% gobgp monitor neighbor <neighbor address> [--current]
```

#### Example

```shell
[TERM1]
% gobgp monitor neighbor
[NEIGH] 192.168.10.2 fsm: BGP_FSM_IDLE admin: down
[NEIGH] 192.168.10.2 fsm: BGP_FSM_ACTIVE admin: up
[NEIGH] 192.168.10.2 fsm: BGP_FSM_OPENSENT admin: up
[NEIGH] 192.168.10.2 fsm: BGP_FSM_OPENCONFIRM admin: up
[NEIGH] 192.168.10.2 fsm: BGP_FSM_ESTABLISHED admin: up

[TERM2]
% gobgp neighbor 192.168.10.2 disable
% gobgp neighbor 192.168.10.2 enable
```

### 5.3 monitor Adj-RIB-In

#### Syntax

```shell
# monitor Adj-RIB-In
% gobgp monitor adj-in [-a <address family>] [--current]
# monitor Adj-RIB-In for specific neighbor
% gobgp monitor adj-in <neighbor address> [-a <address family>] [--current]
```

#### Example

```shell
[GoBGP1]
% gobgp monitor adj-in
[ROUTE] 0:10.2.1.0/24 via 10.0.0.2 aspath [65002] attrs [{Origin: ?}]
[DELROUTE] 0:10.2.1.0/24 via <nil> aspath [] attrs []

[GoBGP2]
% gobgp global rib -a ipv4 add 10.2.1.0/24
% gobgp global rib -a ipv4 del 10.2.1.0/24
```

## 6. <a name="mrt"> mrt subcommand
### 6.1 dump mrt records
#### Syntax
```shell
% gobgp mrt dump rib global [<interval>]
% gobgp mrt dump rib neighbor <neighbor address> [<interval>]
```

#### Options

| short  |long    | description                    |
|--------|--------|--------------------------------|
| f      | format | filename format                |
| o      | outdir | output directory of dump files |

#### Example
see [MRT](https://github.com/osrg/gobgp/blob/master/docs/sources/mrt.md).

### 6.2 inject mrt records
#### Syntax
```shell
% gobgp mrt inject global <filename> [<count>]
```

#### Example
see [MRT](https://github.com/osrg/gobgp/blob/master/docs/sources/mrt.md).
