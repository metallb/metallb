---
title: MetalLB in layer 2 mode
weight: 1
---

In layer 2 mode, one node in your cluster assumes the responsibility
of advertising all service IPs to the local network. From the
network's perspective, it simply looks like that machine has multiple
IP addresses assigned to its network interface.

Under the hood, MetalLB responds to [ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol)
requests for IPv4 services, and [NDP](https://en.wikipedia.org/wiki/Neighbor_Discovery_Protocol) requests for IPv6.

The major advantage of the layer 2 mode is its universality: it will
work on any ethernet network, with no special hardware required, not
even fancy routers.

## Load-balancing behavior

In layer 2 mode, all traffic for all service IPs goes to one
node. From there, `kube-proxy` spreads the traffic to all the
service's pods.

In that sense, layer 2 does not implement a load-balancer. Rather, it
implements a failover mechanism so that a different node can take over
should the current leader node fail for some reason.

If the leader node fails for some reason, failover is automatic: the
old leader's lease times out after 10 seconds, at which point another
node becomes the leader and takes over ownership of all addresses.

## Limitations

Layer 2 mode has two main limitations you should be aware of:
single-node bottlenecking, and potentially slow failover.

As explained above, in layer2 mode a single leader-elected node
receives all traffic for all service IPs. This means that your cluster
ingress bandwidth is limited to the bandwidth of a single node. This
is a fundamental limitation of using ARP and NDP to steer traffic.

In the current implementation, failover between nodes depends on
cooperation from the clients. When a failover occurs, MetalLB sends a
number of gratuitous layer 2 packets (a bit of a misnomer - it should
really be called "unsolicited layer 2 packets") to notify clients that
the MAC address associated with the service IPs has changed.

Most operating systems handle "gratuitous" packets correctly, and
update their neighbor caches promptly. In that case, failover happens
within a few seconds. However, some systems either don't implement
gratuitous handling at all, or have buggy implementations that delay
the cache update.

All modern versions of major OSes (Windows, Mac, Linux) implement
layer 2 failover correctly, so the only situation where issues may
happen is with older or less common OSes.

To minimize the impact of planned failover on buggy clients, you
should keep the old leader node up for a couple of minutes after
flipping leadership, so that it can continue forwarding traffic for
old clients until their caches refresh.

During an unplanned failover, the service IPs will be unreachable
until the buggy clients refresh their cache entries.

If you encounter a situation where layer 2 mode failover is slow (more
than about 10s),
please [file a bug](https://github.com/google/metallb/issues/new)! We
can help you investigate and determine if the issue is with the
client, or a bug in MetalLB.
