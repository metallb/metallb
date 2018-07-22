# GoBGP systemd Integration for CentOS

The following document describes how to manage `gobgp` with `systemd`.

Download `gobgp` binaries, unpack them, and put them `/usr/bin/`:

```bash
mkdir -p /tmp/gobgp
cd /tmp/gobgp && curl -s -L -O https://github.com/osrg/gobgp/releases/download/v1.31/gobgp_1.31_linux_amd64.tar.gz
tar xvzf gobgp_1.31_linux_amd64.tar.gz
mv gobgp /usr/bin/
mv gobgpd /usr/bin/
```

Grant the capability to bind to system or well-known ports, i.e. ports with
numbers `0–1023`, to `gobgpd` binary:

```bash
/sbin/setcap cap_net_bind_service=+ep /usr/bin/gobgpd
/sbin/getcap /usr/bin/gobgpd
```

First, create a system account for `gobgp` service:

```bash
groupadd --system gobgpd
useradd --system -d /var/lib/gobgpd -s /bin/bash -g gobgpd gobgpd
mkdir -p /var/{lib,run,log}/gobgpd
chown -R gobgpd:gobgpd /var/{lib,run,log}/gobgpd
mkdir -p /etc/gobgpd
chown -R gobgpd:gobgpd /etc/gobgpd
```

Paste the below to create `gobgpd` configuration file. The `router-id` in this
example is the IP address of the interface the default route of the host is
pointing to.

```bash
DEFAULT_ROUTE_INTERFACE=$(cat /proc/net/route | cut -f1,2 | grep 00000000 | cut -f1)
DEFAULT_ROUTE_INTERFACE_IPV4=$(ip addr show dev $DEFAULT_ROUTE_INTERFACE | grep "inet " | sed "s/.*inet //" | cut -d"/" -f1)
BGP_AS=65001
BGP_PEER=10.0.255.1
cat << EOF > /etc/gobgpd/gobgpd.conf
[global.config]
  as = $BGP_AS
  router-id = "$DEFAULT_ROUTE_INTERFACE_IPV4"

[[neighbors]]
  [neighbors.config]
    neighbor-address = "$BGP_PEER"
    peer-as = $BGP_AS
EOF
chown -R gobgpd:gobgpd /etc/gobgpd/gobgpd.conf
```

Next, copy the `systemd` unit file, i.e. `gobgpd.service`, in this directory
to `/usr/lib/systemd/system/`:

```bash
cp gobgpd.service /usr/lib/systemd/system/
```

Next, enable and start the `gobgpd` services:

```bash
systemctl enable gobgpd
systemctl start gobgpd
```

If necessary, create an `iptables` rule to allow traffic to `gobgpd` service:

```bash
iptables -I INPUT 4 -p tcp -m state --state NEW --dport 179 -j ACCEPT
```

Also, add the following rule into `INPUT` chain in `/etc/sysconfig/iptables`:

```plaintext
# BGP
-A INPUT -p tcp -m state --state NEW -m tcp --dport 179 -j ACCEPT
```

Check the status of the services:

```bash
systemctl status gobgpd
```

The logs are available via `journald`:

```bash
journalctl -u gobgpd.service --since today
journalctl -u gobgpd.service -r
```

A user may interract with GoBGP daemon via `gobgp` tool:

```bash
# gobgp global
AS:        65001
Router-ID: 10.0.255.1
Listening Port: 179, Addresses: 0.0.0.0, ::

# gobgp global rib summary
Table ipv4-unicast
Destination: 0, Path: 0

# gobgp neighbor
Peer            AS Up/Down State       |#Received  Accepted
10.0.255.1   65001   never Active      |        0
```
