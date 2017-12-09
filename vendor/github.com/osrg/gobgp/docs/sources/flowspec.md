# Flow Specification

GoBGP supports [RFC5575](https://tools.ietf.org/html/rfc5575),
[RFC7674](https://tools.ietf.org/html/rfc7674),
[draft-ietf-idr-flow-spec-v6](https://tools.ietf.org/html/draft-ietf-idr-flow-spec-v6)
and [draft-ietf-idr-flowspec-l2vpn](https://tools.ietf.org/html/draft-ietf-idr-flowspec-l2vpn).

## Prerequisites

Assume you finished [Getting Started](getting-started.md).

## Contents

- [Configuration](#configuration)
- [CLI Syntax](#cli-syntax)

## Configuration

To enable FlowSpec family, please enumerate the corresponding "afi-safi-name" in
"neighbors.afi-safis" section like the below.

```toml
[[neighbors]]
  # ...(snip)...
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv4-flowspec"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "ipv6-flowspec"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l3vpn-ipv4-flowspec"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l3vpn-ipv6-flowspec"
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
      afi-safi-name = "l2vpn-flowspec"
  # ...(snip)...
```

## CLI Syntax

### IPv4/IPv6 FlowSpec

```bash
# Add a route
$ gobgp global rib -a {ipv4-flowspec|ipv6-flowspec} add match <MATCH> then <THEN>
    <MATCH> : { destination <PREFIX> [<OFFSET>] |
                source <PREFIX> [<OFFSET>] |
                protocol <PROTOCOLS>... |
                fragment <FRAGMENTS>... |
                tcp-flags <TCP_FLAGS>... |
                port <ITEM>... |
                destination-port <ITEM>... |
                source-port <ITEM>... |
                icmp-type <ITEM>... |
                icmp-code <ITEM>... |
                packet-length <ITEM>... |
                dscp <ITEM>... |
                label <ITEM>... }...
    <PROTOCOLS> : [&] [<|<=|>|>=|==|!=] <PROTOCOL>
    <PROTOCOL> : egp, gre, icmp, igmp, igp, ipip, ospf, pim, rsvp, sctp, tcp, udp, unknown, <DEC_NUM>
    <FRAGMENTS> : [&] [=|!|!=] <FRAGMENT>
    <FRAGMENT> : dont-fragment, is-fragment, first-fragment, last-fragment, not-a-fragment
    <TCP_FLAGS> : [&] [=|!|!=] <TCP_FLAG>
    <TCP_FLAG> : F, S, R, P, A, U, E, C
    <ITEM> : [&] [<|<=|>|>=|==|!=] <DEC_NUM>
    <THEN> : { accept |
               discard |
               rate-limit <RATE> [as <AS>] |
               redirect <RT> |
               mark <DEC_NUM> |
               action { sample | terminal | sample-terminal } }...
    <RT> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxxx::xxxx:yyy, xxx.xxx:yyy

# Show routes
$ gobgp global rib -a {ipv4-flowspec|ipv6-flowspec}

# Delete route
$ gobgp global rib -a {ipv4-flowspec|ipv6-flowspec} del match <MATCH_EXPR>
```

### VPNv4/VPNv6 FlowSpec

```bash
# Add a route
$ gobgp global rib -a {ipv4-l3vpn-flowspec|ipv6-l3vpn-flowspec} add rd <RD> match <MATCH> then <THEN> [rt <RT>]
    <RD> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxx.xxx:yyy
    <MATCH> : { destination <PREFIX> [<OFFSET>] |
                source <PREFIX> [<OFFSET>] |
                protocol <PROTOCOLS>... |
                fragment <FRAGMENTS>... |
                tcp-flags <TCP_FLAGS>... |
                port <ITEM>... |
                destination-port <ITEM>... |
                source-port <ITEM>... |
                icmp-type <ITEM>... |
                icmp-code <ITEM>... |
                packet-length <ITEM>... |
                dscp <ITEM>... |
                label <ITEM>...}...
    <PROTOCOLS> : [&] [<|<=|>|>=|==|!=] <PROTOCOL>
    <PROTOCOL> : egp, gre, icmp, igmp, igp, ipip, ospf, pim, rsvp, sctp, tcp, udp, unknown, <DEC_NUM>
    <FRAGMENTS> : [&] [=|!|!=] <FRAGMENT>
    <FRAGMENT> : dont-fragment, is-fragment, first-fragment, last-fragment, not-a-fragment
    <TCP_FLAGS> : [&] [=|!|!=] <TCP_FLAG>
    <TCP_FLAG> : F, S, R, P, A, U, E, C
    <ITEM> : [&] [<|<=|>|>=|==|!=] <DEC_NUM>
    <THEN> : { accept |
               discard |
               rate-limit <RATE> [as <AS>] |
               redirect <RT> |
               mark <DEC_NUM> |
               action { sample | terminal | sample-terminal } }...
    <RT> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxxx::xxxx:yyy, xxx.xxx:yyy

# Show routes
$ gobgp global rib -a {ipv4-l3vpn-flowspec|ipv6-l3vpn-flowspec}

# Delete route
$ gobgp global rib -a {ipv4-l3vpn-flowspec|ipv6-l3vpn-flowspec} del rd <RD> match <MATCH_EXPR>
```

### L2VPN FlowSpec

```bash
# Add a route
$ gobgp global rib -a l2vpn-flowspec add rd <RD> match <MATCH> then <THEN> [rt <RT>]
    <RD> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxx.xxx:yyy
    <MATCH> : { destination <PREFIX> [<OFFSET>] |
                source <PREFIX> [<OFFSET>] |
                protocol <PROTOCOLS>... |
                fragment <FRAGMENTS>... |
                tcp-flags <TCP_FLAGS>... |
                port <ITEM>... |
                destination-port <ITEM>... |
                source-port <ITEM>... |
                icmp-type <ITEM>... |
                icmp-code <ITEM>... |
                packet-length <ITEM>... |
                dscp <ITEM>... |
                label <ITEM>... |
                destination-mac <MAC_ADDRESS> |
                source-mac <MAC_ADDRESS> |
                ether-type <ETHER_TYPES>... |
                llc-dsap <ITEM>... |
                llc-ssap <ITEM>... |
                llc-control <ITEM>... |
                snap <ITEM>... |
                vid <ITEM>... |
                cos <ITEM>... |
                inner-vid <ITEM>... |
                inner-cos <ITEM>... }...
    <PROTOCOLS> : [&] [<|<=|>|>=|==|!=] <PROTOCOL>
    <PROTOCOL> : egp, gre, icmp, igmp, igp, ipip, ospf, pim, rsvp, sctp, tcp, udp, unknown, <DEC_NUM>
    <FRAGMENTS> : [&] [=|!|!=] <FRAGMENT>
    <FRAGMENT> : dont-fragment, is-fragment, first-fragment, last-fragment, not-a-fragment
    <TCP_FLAGS> : [&] [=|!|!=] <TCP_FLAG>
    <TCP_FLAG> : F, S, R, P, A, U, E, C
    <ETHER_TYPES> : [&] [<|<=|>|>=|==|!=] <ETHER_TYPE>
    <ETHER_TYPE> : aarp, apple-talk, arp, ipv4, ipv6, ipx, loopback, net-bios, pppoe-discovery, pppoe-session, rarp, snmp, vmtp, xtp, <DEC_NUM>
    <ITEM> : [&] [<|<=|>|>=|==|!=] <DEC_NUM>
    <THEN> : { accept |
               discard |
               rate-limit <RATE> [as <AS>] |
               redirect <RT> |
               mark <DEC_NUM> |
               action { sample | terminal | sample-terminal } }...
    <RT> : xxx:yyy, xxx.xxx.xxx.xxx:yyy, xxxx::xxxx:yyy, xxx.xxx:yyy

# Show routes
$ gobgp global rib -a l2vpn-flowspec

# Delete route
$ gobgp global rib -a l2vpn-flowspec del rd <RD> match <MATCH_EXPR>
```

### Match (Traffic Filtering Rules)

| Type | Key              | Operator/Operand Type | Value                                                  |
| ---- | ---------------- | --------------------- | ------------------------------------------------------ |
| 1    | destination      | -                     | IP Prefix (or IP Address).                             |
| 2    | source           | -                     | IP Prefix (or IP Address).                             |
| 3    | protocol         | Numeric               | Protocol name, decimal number, `true` or `false`.      |
| 4    | port             | Numeric               | Decimal number, `true` or `false`.                     |
| 5    | destination-port | Numeric               | Decimal number, `true` or `false`.                     |
| 6    | source-port      | Numeric               | Decimal number, `true` or `false`.                     |
| 7    | icmp-type        | Numeric               | Decimal number, `true` or `false`.                     |
| 8    | icmp-code        | Numeric               | Decimal number, `true` or `false`.                     |
| 9    | tcp-flags        | Bitmask               | TCP flag or its combination.                           |
| 10   | packet-length    | Numeric               | Decimal number, `true` or `false`.                     |
| 11   | dscp             | Numeric               | Decimal number, `true` or `false`.                     |
| 12   | fragment         | Bitmask               | Fragment type or its combination joined with `+`.      |
| 13   | label            | Numeric               | Decimal number, `true` or `false`.                     |
| 14   | ether-type       | Numeric               | Ethernet type name, decimal number, `true` or `false`. |
| 15   | source-mac       | -                     | MAC address.                                           |
| 16   | destination-mac  | -                     | MAC address.                                           |
| 17   | llc-dsap         | Numeric               | Decimal number, `true` or `false`.                     |
| 18   | llc-ssap         | Numeric               | Decimal number, `true` or `false`.                     |
| 19   | llc-control      | Numeric               | Decimal number, `true` or `false`.                     |
| 20   | snap             | Numeric               | Decimal number, `true` or `false`.                     |
| 21   | vid              | Numeric               | Decimal number, `true` or `false`.                     |
| 22   | cos              | Numeric               | Decimal number, `true` or `false`.                     |
| 23   | inner-vid        | Numeric               | Decimal number, `true` or `false`.                     |
| 24   | inner-cos        | Numeric               | Decimal number, `true` or `false`.                     |

**Note:** IPv4/VPNv4 FlowSpec families support types 1-12, IPv6/VPNv6 FlowSpec
families support types 1-13 and L2VPN FlowSpec family supports types 1-24.

#### Operator/Operand Types

| Type    | Value                                                      |
| ------- | ---------------------------------------------------------- |
| Numeric | \[&] \[== &#124; > &#124; >= &#124; < &#124; <= &#124; !=] |
| Bitmask | \[&] \[= &#124; ! &#124; !=]                               |

**Note:** For the decimal type values (e.g., `port`), you can combine the
following operators and the reserved values. The following complies with
[draft-ietf-idr-rfc5575](https://tools.ietf.org/html/draft-ietf-idr-rfc5575bis-06#section-4.2.3).

| lt   | gt   | eq   | Operator/Value                                     |
| ---- | ---- | ---- | -------------------------------------------------- |
| 0    | 0    | 0    | `true` (no operator and independent of the value)  |
| 0    | 0    | 1    | ==                                                 |
| 0    | 1    | 0    | \>                                                 |
| 0    | 1    | 1    | \>=                                                |
| 1    | 0    | 0    | \<                                                 |
| 1    | 0    | 1    | \<=                                                |
| 1    | 1    | 0    | !=                                                 |
| 1    | 1    | 1    | `false` (no operator and independent of the value) |

**Note:** For the bitmask operand, RFC5575 says "=value" and "value" is the
different in the bitwise match operation. With "=value", it is evaluated as
"(data & value) == value"; with "value" (without "="), "data & value" evaluates
to TRUE if any of the bits in the value mask are set in the data.

#### Example - Destination Prefix

| Key         | Value                     |
| ----------- | ------------------------- |
| destination | IP Prefix (or IP Address) |

```bash
# gobgp global rib -a ipv4-flowspec add match destination <IPv4 Prefix> then <THEN>
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then accept
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?}]

# If IPv4 address is specified, it will be treated as /32 prefix
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.1 then accept
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.1/32] fictitious                                00:00:00   [{Origin: ?}]

# gobgp global rib -a ipv6-flowspec add match destination <IPv6 Prefix> [OFFSET] then <THEN>
$ gobgp global rib -a ipv6-flowspec add match destination 2001:db8:1::/64 then accept
$ gobgp global rib -a ipv6-flowspec
   Network                          Next Hop             AS_PATH              Age        Attrs
*> [destination: 2001:db8:1::/64/0] fictitious                                00:00:00   [{Origin: ?}]

# With prefix offset
$ gobgp global rib -a ipv6-flowspec add match destination 2001:db8:1::/64 32 then accept
$ gobgp global rib -a ipv6-flowspec
   Network                           Next Hop             AS_PATH              Age        Attrs
*> [destination: 2001:db8:1::/64/32] fictitious                                00:00:00   [{Origin: ?}]

# As with IPv4 address, if IPv6 address is specified, it will be treated as /128 prefix
$ gobgp global rib -a ipv6-flowspec add match destination 2001:db8:1::1 then accept
$ gobgp global rib -a ipv6-flowspec
   Network                            Next Hop             AS_PATH              Age        Attrs
*> [destination: 2001:db8:1::1/128/0] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - IP Protocol/Next Header

| Key      | Operator                                                   | Value                                             |
| -------- | ---------------------------------------------------------- | ------------------------------------------------- |
| protocol | \[&] \[== &#124; > &#124; >= &#124; < &#124; <= &#124; !=] | Protocol name, decimal number, `true` or `false`. |

Supported Protocol Names: `icmp`, `igmp`, `tcp`, `egp`, `igp`, `udp`, `rsvp`,
`gre`, `ospf`, `ipip`, `pim`, `sctp`.

```bash
# gobgp global rib -a ipv4-flowspec add match protocol <Protocol> then <THEN>
$ gobgp global rib -a ipv4-flowspec add match protocol tcp then accept
$ gobgp global rib -a ipv4-flowspec
   Network              Next Hop             AS_PATH              Age        Attrs
*> [protocol: ==tcp]    fictitious                                00:00:00   [{Origin: ?}]

# Combination of rules
# Note: "true" or "false" should be the last of rule without operator
$ gobgp global rib -a ipv4-flowspec add match protocol '==tcp &=udp icmp >igmp >=egp <igp <=rsvp !=gre &!ospf true' then accept
$ gobgp global rib -a ipv4-flowspec
   Network                                                                  Next Hop             AS_PATH              Age        Attrs
*> [protocol: ==tcp&==udp ==icmp >igmp >=egp <igp <=rsvp !=gre&!=ospf true] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - Port

| Key  | Operator                                                   | Value                             |
| ---- | ---------------------------------------------------------- | --------------------------------- |
| port | \[&] \[== &#124; > &#124; >= &#124; < &#124; <= &#124; !=] | Decimal number, `true` or `false` |

```bash
# gobgp global rib -a ipv4-flowspec add match port <Port> then <THEN>
$ gobgp global rib -a ipv4-flowspec add match port 80 then accept
$ gobgp global rib -a ipv4-flowspec
   Network              Next Hop             AS_PATH              Age        Attrs
*> [port: ==80]         fictitious                                00:00:00   [{Origin: ?}]

# Combination of rules
# Note: "true" or "false" should be the last of rule without operator
$ gobgp global rib -a ipv4-flowspec add match port '==80 &=90 8080 >9090 >=10080 <10090 <=18080 !=19090 &!443 true' then accept
$ gobgp global rib -a ipv4-flowspec
   Network                                                                  Next Hop             AS_PATH              Age        Attrs
*> [port: ==80&==90 ==8080 >9090 >=10080 <10090 <=18080 !=19090&!=443 true] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - TCP flags

| Key       | Operand                      | Value                        |
| --------- | ---------------------------- | ---------------------------- |
| tcp-flags | \[&] \[= &#124; ! &#124; !=] | TCP flag or its combination. |

Supported TCP Flags: `F (=FIN)`, `S (=SYN)`, `R (=RST)`, `P (=PUSH)`,
`A (=ACK)`, `U (=URGENT)`, `C (=CWR)`, `E (=ECE)`.

```bash
# gobgp global rib -a ipv4-flowspec add match tcp-flags <TCP Flags> then <THEN>
$ gobgp global rib -a ipv4-flowspec add match tcp-flags SA then accept
$ gobgp global rib -a ipv4-flowspec
   Network              Next Hop             AS_PATH              Age        Attrs
*> [tcp-flags: SA]      fictitious                                00:00:00   [{Origin: ?}]

# Combination of rules
# Note: '=!C' will be converted to '!=C' for the backward compatibility
$ gobgp global rib -a ipv4-flowspec add match tcp-flags '==S &=SA A !F !=U =!C' then accept
$ gobgp global rib -a ipv4-flowspec
   Network                          Next Hop             AS_PATH              Age        Attrs
*> [tcp-flags: =S&=SA A !F !=U !=C] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - Fragment

| Key      | Operand                      | Value                                             |
| -------- | ---------------------------- | ------------------------------------------------- |
| fragment | \[&] \[= &#124; ! &#124; !=] | Fragment type or its combination joined with `+`. |

Supported Fragment Types: `not-a-fragment`, `dont-fragment`, `is-fragment`,
`first-fragment`, `last-fragment`.

```bash
# gobgp global rib -a ipv4-flowspec add match fragment <Fragment> then <THEN>
$ gobgp global rib -a ipv4-flowspec add match fragment dont-fragment then accept
$ gobgp global rib -a ipv4-flowspec
   Network                   Next Hop             AS_PATH              Age        Attrs
*> [fragment: dont-fragment] fictitious                                00:00:00   [{Origin: ?}]

# Combination of rules
$ gobgp global rib -a ipv4-flowspec add match fragment dont-fragment is-fragment+first-fragment then accept
$ gobgp global rib -a ipv4-flowspec
   Network                                              Next Hop             AS_PATH              Age        Attrs
*> [fragment: dont-fragment is-fragment+first-fragment] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - Ethernet Type

| Key      | Operand                                                    | Value                                                  |
| -------- | ---------------------------------------------------------- | ------------------------------------------------------ |
| fragment | \[&] \[== &#124; > &#124; >= &#124; < &#124; <= &#124; !=] | Ethernet type name, decimal number, `true` or `false`. |

Supported Ethernet Type Names: `ipv4`, `arp`, `rarp`, `vmtp`, `apple-talk`,
`aarp`, `ipx`, `snmp`, `net-bios`, `xtp`, `ipv6`, `pppoe-discovery`,
`pppoe-session`, `loopback`.

```bash
# gobgp global rib -a l2vpn-flowspec add rd <RD> match ether-type <Ethernet Type> then <THEN>
$ gobgp global rib -a l2vpn-flowspec add rd 65000:100 match ether-type arp then accept
$ gobgp global rib -a l2vpn-flowspec
   Network                            Next Hop             AS_PATH              Age        Attrs
*> [rd: 65000:100][ether-type: ==arp] fictitious                                00:00:00   [{Origin: ?}]
```

#### Example - Source MAC

| Key        | Value        |
| ---------- | ------------ |
| source-mac | MAC Address. |

```bash
# gobgp global rib -a l2vpn-flowspec add rd <RD> match source-mac <MAC Address> then <THEN>
$ gobgp global rib -a l2vpn-flowspec add rd 65000:100 match source-mac aa:bb:cc:dd:ee:ff then accept
$ gobgp global rib -a l2vpn-flowspec
   Network                                        Next Hop             AS_PATH              Age        Attrs
*> [rd: 65000:100][source-mac: aa:bb:cc:dd:ee:ff] fictitious                                00:00:00   [{Origin: ?}]
```

### Then (Traffic Filtering Actions)

| Type   | Action                         | Description                                                              |
| ------ | ------------------------------ | ------------------------------------------------------------------------ |
| -      | accept                         | Accept the traffic.                                                      |
| 0x8006 | discard                        | Discard the traffic using traffic-rate of 0.                             |
| 0x8006 | rate-limit \<RATE> \[as \<AS>] | Specify the rate of traffic in float value.                              |
| 0x8007 | action sample                  | Enables the traffic sampling and logging.                                |
| 0x8007 | action terminal                | Specify the termination of the traffic filter.                           |
| 0x8007 | action sample-terminal         | Specify both of sample and terminal.                                     |
| 0x8008 | redirect \<RT>                 | Redirect to VRF which has the given RT in its import policy.             |
| 0x8009 | mark \<VALUE>                  | Modifies the DSCP in IPv4 or Traffic Class in IPv6 with the given value. |

#### Example - accept/discard

```bash
# accept action
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then accept
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?}]


# discard action
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then discard
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [discard]}]
```

#### Example - rate-limit

```bash
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then rate-limit 100.0
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [rate: 100.000000]}]

# With the informational AS number
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then rate-limit 100.0 as 65000
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [rate: 100.000000(as: 65000)]}]
```

#### Example - action

```bash
# sample action
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then action sample
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [action: sample]}]

# terminal action
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then action terminal
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [action: terminal]}]

# sample-terminal action
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then action sample-terminal
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [action: terminal-sample]}]
```

#### Example - redirect

```bash
# with Two Octet AS specific RT
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then redirect 65000:100
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [redirect: 65000:100]}]

# with IPv4 address specific RT
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then redirect 1.1.1.1:100
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [redirect: 1.1.1.1:100]}]

# with IPv6 address specific RT
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then redirect 2001:db8::1:100
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [redirect: 2001:db8::1:100]}]

# with Four Octet AS specific RT
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then redirect 200.200:100
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [redirect: 200.200:100]}]
```

#### Example - mark

```bash
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 then mark 10
$ gobgp global rib -a ipv4-flowspec
   Network                    Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [remark: 10]}]
```

### Example of Combinations of Rules and Actions

```bash
# add a flowspec rule which redirect flows whose dst 10.0.0.0/24 and src 20.0.0.0/24 to VRF with RT 10:10
$ gobgp global rib -a ipv4-flowspec add match destination 10.0.0.0/24 source 20.0.0.0/24 then redirect 10:10
$ gobgp global rib -a ipv4-flowspec
   Network                                         Next Hop             AS_PATH              Age        Attrs
*> [destination: 10.0.0.0/24][source: 20.0.0.0/24] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [redirect: 10:10]}]

# add a flowspec rule which discard flows whose dst 2001::2/128 and port equals 80 and with TCP flags not match SA (SYN/ACK) and not match U (URG)
$ gobgp global rib -a ipv6-flowspec add match destination 2001::2/128 port '==80' tcp-flags '!=SA&!=U' then discard
$ gobgp global rib -a ipv6-flowspec
   Network                                                       Next Hop             AS_PATH              Age        Attrs
*> [destination: 2001::2/128/0][port: ==80][tcp-flags: !=SA&!=U] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [discard]}]

# add another flowspec rule which discard flows whose
# - ip protocol is tcp
# - destination port is 80 or greater than or equal to 8080 and lesser than or equal to 8888
# - packet is a first fragment or a last fragment
$ gobgp global rib -a ipv4-flowspec add match protocol tcp destination-port '==80' '>=8080&<=8888' fragment '=first-fragment =last-fragment' then discard
$ gobgp global rib -a ipv4-flowspec
   Network                                                                                           Next Hop             AS_PATH              Age        Attrs
*> [protocol: ==tcp][destination-port: ==80 >=8080&<=8888][fragment: =first-fragment =last-fragment] fictitious                                00:00:00   [{Origin: ?} {Extcomms: [discard]}]
```
