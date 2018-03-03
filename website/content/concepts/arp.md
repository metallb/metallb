---
title: MetalLB in ARP mode
weight: 1
---

In ARP mode, one node in your cluster assumes the responsibility of
advertising all service IPs to the local network. From the network's
perspective, it simply looks like that machine has multiple IP
addresses assigned to its network interface.

ARP mode does this by listening
for [ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol)
requests, and responding to requests for the service IPs it knows
about.

ARP mode's major advantage is universality: it will work on any
ethernet network, with no special hardware required, not even fancy
routers.

## Load-balancing behavior

In ARP mode, all traffic for all service IPs goes to one node. From
there, `kube-proxy` spreads the traffic to all the service's pods.

In that sense, ARP mode does not implement a load-balancer. Rather, it
implements a failover mechanism so that a different node can take over
should the current leader node fail for some reason.

## Limitations

ARP mode has two main limitations you should be aware of: single-node
bottlenecking, and potentially slow failover.

As explained above, in ARP mode a single leader-elected node receives
all traffic for all service IPs. This means that your cluster ingress
bandwidth is limited to the bandwidth of a single node. This is a
fundamental limitation of using ARP to steer traffic.

In the current implementation of ARP mode, failover between nodes
depends on cooperation from the clients. When a failover occurs,
MetalLB sends a number of gratuitous ARP packets (a bit of a
misnomer - it should really be called "unsollicited ARP packets") to
notify clients that the MAC address associated with the service IPs
has changed.

Most operating systems handle "gratuitous ARP" correctly, and update
their ARP caches promptly. In that case, failover happens within a few
seconds. However, some systems either don't implement gratuitous ARP
handling at all, or have buggy implementations that delay the ARP
cache update.

All modern versions of major OSes (Windows, Mac, Linux) implement ARP
failover correctly, so the only situation where issues may happen is
with older or less common OSes.

To minimize the impact of planned failover on buggy clients, you
should keep the old leader node up for a couple of minutes after
flipping leadership, so that it can continue forwarding traffic for
old clients until their ARP caches refresh.

During an unplanned failover, the service IPs will be unreachable
until the buggy clients refresh their ARP cache entries.

If you encounter a situation where ARP failover is slow (more than
about 10s),
please [file a bug](https://github.com/google/metallb/issues/new)! We
can help you investigate and determine if the issue is with the
client, or a bug in MetalLB.
