# RPKI

This page explains how to use a Resource Public Key Infrastructure
(RPKI) server to do Origin AS Validation.

## Prerequisites

Assume you finished [Getting Started](https://github.com/osrg/gobgp/blob/master/docs/sources/getting-started.md).

## Contents

- [Configuration](#section0)
- [Validation](#section1)
- [Policy with validation results](#section2)
- [Force Re-validation](#section3)
- [Monitoring validation](#section4)

## <a name="section0"> Configuration

You need to add **[RpkiServers]** section to your configuration
file. We use the following file. Note that this is for route server
setup but RPKI can be used with non route server setup.

```toml
[global.config]
as = 64512
router-id = "10.0.255.254"

[[neighbors]]
  [neighbors.config]
    peer-as = 65001
    neighbor-address = "10.0.255.1"
  [neighbors.route-server.config]
    route-server-client = true

[[neighbors]]
  [neighbors.config]
    peer-as = 65002
    neighbor-address = "10.0.255.2"
  [neighbors.route-server.config]
    route-server-client = true

[[rpki-servers]]
  [rpki-servers.config]
    address = "210.173.170.254"
    port = 323
```

## <a name="section1"> Validation

You can verify whether gobgpd successfully connects to the RPKI server
and get the ROA (Route Origin Authorization) information in the
following way:

```bash
$ gobgp rpki server
Session                State  Uptime     #IPv4/IPv6 records
210.173.170.254:323    Up     00:03:06   14823/2168
```

```bash
$ gobgp rpki table 210.173.170.254|head -n4
Network            Maxlen AS
2.0.0.0/12         16     3215
2.0.0.0/16         16     3215
2.1.0.0/16         16     3215
```

By default, IPv4's ROA information is shown. You can see IPv6's like:

```bash
$ gobgp rpki -a ipv6 table 210.173.170.254|head -n4
fujita@ubuntu:~$ gobgp rpki -a ipv6|head -n3
Network                                    Maxlen AS
2001:608::/32                              32     5539
2001:610::/32                              48     1103
2001:610:240::/42                          42     3333
```

We configure the peer 10.0.255.1 to send three routes:

1. 2.0.0.0/12 (Origin AS: 3215)
2. 2.1.0.0/16 (Origin AS: 65001)
3. 192.186.1.0/24 (Origin AS: 65001)

From the above ROA information, the first is valid. the second is
invalid (the origin should be 3215 too). the third is a private IPv4
address so it should not be in the ROA.

Let's check out the adjacent rib-in of the peer:

```bash
$ gobgp neighbor 10.0.255.1 adj-in
    Network              Next Hop             AS_PATH              Age        Attrs
    V   2.0.0.0/12       10.0.255.1           3215                 00:08:39   [{Origin: i}]
    I   2.1.0.0/16       10.0.255.1           65001                00:08:39   [{Origin: i}]
    N   192.168.1.0/24   10.0.255.1           65001                00:08:39   [{Origin: i}]
```

As you can see, the first is marked as "V" (Valid), the second as "I"
(Invalid), and the third as "N" (Not Found).


## <a name="section2"> Policy with validation results

The validation result can be used as [Policy's
condition](https://github.com/osrg/gobgp/blob/master/docs/sources/policy.md). You
can do any actions (e.g., drop the route, adding some extended
community attribute, etc) according to the validation result. As an
example, this section shows how to drop an invalid route.

Currently, all the routes from peer 10.0.255.1 are included in peer 10.0.255.2's local RIB.

```bash
$ gobgp neighbor 10.0.255.2 local
    Network              Next Hop             AS_PATH              Age        Attrs
    V*> 2.0.0.0/12       10.0.255.1           3215                 00:23:47   [{Origin: i}]
    I*> 2.1.0.0/16       10.0.255.1           65001                00:23:47   [{Origin: i}]
    N*> 192.168.1.0/24   10.0.255.1           65001                00:23:47   [{Origin: i}]
```

We add a policy to the above configuration.

```toml
[global.config]
as = 64512
router-id = "10.0.255.254"

[[neighbors]]
  [neighbors.config]
    peer-as = 65001
    neighbor-address = "10.0.255.1"
  [neighbors.route-server.config]
    route-server-client = true

[[neighbors]]
  [neighbors.config]
    peer-as = 65002
    neighbor-address = "10.0.255.2"
  [neighbors.route-server.config]
    route-server-client = true
  [neighbors.apply-policy-config]
    import-policy-list = ["AS65002-IMPORT-RPKI"]


[[rpki-servers]]
  [rpki-servers.config]
    address = "210.173.170.254"
    port = 323

[[policy-definitions]]
  name = "AS65002-IMPORT-RPKI"
  [[policy-definitions.statements]]
    name = "statement1"
    [policy-definitions.statements.conditions.bgp-conditions]
      rpki-validation-result = "invalid"
    [policy-definitions.statements.conditions.actions.route-disposition]
      reject-route = true
```

The value for **RpkiValidationResult** are defined as below.

| Validation Result | Value           |
|-------------------|-----------------|
| Not Found         |   "not-found"   |
| Valid             |   "valid"       |
| Invalid           |   "invalid"     |

With the new configuration, the IMPORT policy rejects the invalid 2.1.0.0/16.

```bash
$ gobgp neighbor 10.0.255.2 local
    Network              Next Hop             AS_PATH              Age        Attrs
    V*> 2.0.0.0/12       10.0.255.1           3215                 00:00:21   [{Origin: i}]
    N*> 192.168.1.0/24   10.0.255.1           65001                00:00:21   [{Origin: i}]
```


### Detailed Information about validation
You can get the detailed information about announced routes.
```bash
$ gobgp neighbor 10.0.255.1 adj-in 2.1.0.0/16 validation
Target Prefix: 2.1.0.0/16, AS: 65001
  This route is invalid  reason: as
  No VRP ASN matches the route origin ASN.

  Matched VRPs:
    No Entry
  Unmatched AS VRPs:
    Network            AS    MaxLen
    2.0.0.0/12         3215  16
    2.1.0.0/16         3215  16
  Unmatched Length VRPs:
    No Entry
```
From this, we can notice that 2.1.0.0/16 (Origin AS: 65001) is invalid due to its origin AS,
the origin AS should be 3215.

## <a name="section3"> Force Re-validation

Validation is executed every time bgp update messages arrive. The
changes of ROAs doesn't trigger off validation. The following command
enables you to validate all the routes.

```bash
$ gobgp rpki validate
```
