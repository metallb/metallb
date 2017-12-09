# Graceful Restart

This page explains how to configure [Graceful Restart](https://tools.ietf.org/html/rfc4724),
[Graceful Restart Notification Support](https://tools.ietf.org/html/draft-ietf-idr-bgp-gr-notification-07) and
[Long Lived Graceful Restart](https://tools.ietf.org/html/draft-uttaro-idr-bgp-persistence-02).
Graceful Restart has two sides. One is restarting speaker which does restart,
the other is receiving speaker (helper speaker) which helps a restarting speaker
to do graceful restart. GoBGP supports both roles.

## Contents

- [Helper speaker](#helper)
- [Restarting speaker](#restarting)
- [Graceful Restart Notification Support](#notification)
- [Long Lived Graceful Restart](#long-lived)

## <a name="helper"> Helper speaker

Below is the configuration to enable helper speaker behavior.

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
```

Check graceful restart capability is negotiated.

```shell
$ gobgp n 10.0.255.1
BGP neighbor is 10.0.255.1, remote AS 65001
  BGP version 4, remote router ID 192.168.0.2
  BGP state = BGP_FSM_ESTABLISHED, up for 00:00:36
  BGP OutQ = 0, Flops = 0
  Hold time is 0, keepalive interval is 30 seconds
  Configured hold time is 90, keepalive interval is 30 seconds
  Neighbor capabilities:
    BGP_CAP_MULTIPROTOCOL:
        RF_IPv4_UC:     advertised and received
    BGP_CAP_ROUTE_REFRESH:      advertised and received
    BGP_CAP_GRACEFUL_RESTART:   advertised and received
        Remote: restart time 90 sec
            RF_IPv4_UC
    BGP_CAP_FOUR_OCTET_AS_NUMBER:       advertised and received
  Message statistics:
                         Sent       Rcvd
    Opens:                  1          1
    Notifications:          0          0
    Updates:                2          1
    Keepalives:             2          2
    Route Refresh:          0          0
    Discarded:              0          0
    Total:                  5          4
  Route statistics:
    Advertised:             1
    Received:               0
    Accepted:               0
```

## <a name="restarting"> Restarting speaker

To support restarting speaker behavior, try the configuration below.

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
    restart-time = 120
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
    afi-safi-name = "ipv4-unicast"
    [neighbors.afi-safis.mp-graceful-restart.config]
        enabled = true
```

By this configuration, if graceful restart capability is negotiated with the peer,
the peer starts graceful restart helper procedure, when `gobgpd` dies involuntarily or
`SIGINT`, `SIGKILL` signal is sent to `gobgpd`.
Note when `SIGTERM` signal is sent to `gobgpd`, graceful restart negotiated peers
don't start graceful restart helper procedure, since `gobgpd` sends notification
messages to these peers before it die.

When you restart `gobgpd`, add `-r` option to let peers know `gobgpd` is
recovered from graceful restart.

```shell
$ gobgpd -f gobgpd.conf -r
```

Let's see how capability negotiation changes.

```shell
$ gobgp n 10.0.255.1
BGP neighbor is 10.0.255.1, remote AS 65001
  BGP version 4, remote router ID 192.168.0.2
  BGP state = BGP_FSM_ESTABLISHED, up for 00:00:03
  BGP OutQ = 0, Flops = 0
  Hold time is 0, keepalive interval is 30 seconds
  Configured hold time is 90, keepalive interval is 30 seconds
  Neighbor capabilities:
    BGP_CAP_MULTIPROTOCOL:
        RF_IPv4_UC:     advertised and received
    BGP_CAP_ROUTE_REFRESH:      advertised and received
    BGP_CAP_GRACEFUL_RESTART:   advertised and received
        Local: restart time 90 sec, restart flag set
            RF_IPv4_UC, forward flag set
        Remote: restart time 90 sec
            RF_IPv4_UC
    BGP_CAP_FOUR_OCTET_AS_NUMBER:       advertised and received
  Message statistics:
                         Sent       Rcvd
    Opens:                  1          1
    Notifications:          0          0
    Updates:                2          1
    Keepalives:             1          1
    Route Refresh:          0          0
    Discarded:              0          0
    Total:                  4          3
  Route statistics:
    Advertised:             1
    Received:               0
    Accepted:               0
```

You can see `restart flag` and `forward flag` is set.

Without `-r` option, the peers which are under helper procedure will
immediately withdraw all routes which were advertised from `gobgpd`.

Also, when `gobgpd` doesn't recovered within `restart-time`, the peers will
withdraw all routes.
Default value of `restart-time` is equal to `hold-time`.

## <a name="notification"> Graceful Restart Notification Support

[RFC4724](https://tools.ietf.org/html/rfc4724) specifies gracful restart procedures are triggered only when
the BGP session between graceful restart capable peers turns down without
a notification message for backward compatibility.
[Graceful Restart Notification Support](https://tools.ietf.org/html/draft-ietf-idr-bgp-gr-notification-07)
expands this to trigger graceful restart procedures also with a notification message.
To turn on this feature, add `notification-enabled = true` to configuration like below.

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
    notification-enabled = true
```

## <a name="long-lived"> Long Lived Graceful Restart

### Long Lived Graceful Restart Helper Speaker Configuration

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
    long-lived-enabled = true
```

### Long Lived Graceful Restart Restarting Speaker Configuration

Unlike normal graceful restart, long-lived graceful restart supports
restart-time as per address family.

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
    long-lived-enabled = true
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
    afi-safi-name = "ipv4-unicast"
    [neighbors.afi-safis.long-lived-graceful-restart.config]
        enabled = true
        restart-time = 100000
```

### Conbination with normal Graceful Restart

You can also use long lived graceful restart with normal graceful restart.

```toml
[global.config]
  as = 64512
  router-id = "192.168.255.1"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "10.0.255.1"
    peer-as = 65001
  [neighbors.graceful-restart.config]
    enabled = true
    long-lived-enabled = true
    restart-time = 120
  [[neighbors.afi-safis]]
    [neighbors.afi-safis.config]
    afi-safi-name = "ipv4-unicast"
    [neighbors.afi-safis.mp-graceful-restart.config]
        enabled = true
    [neighbors.afi-safis.long-lived-graceful-restart.config]
        enabled = true
        restart-time = 100000
```
