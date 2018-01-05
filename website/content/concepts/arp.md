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
bottlenecking, and slow failover.

As explained above, in ARP mode a single leader-elected node receives
all traffic for all service IPs. This means that your cluster ingress
bandwidth is limited to the bandwidth of a single node. This is a
fundamental limitation of using ARP to steer traffic.

In the current implementation of ARP mode, failover between nodes is
quite slow. ARP mode does not use virtual MAC addresses to facilitate
failovers, so if the cluster leader switches from node A to node B,
traffic from existing clients will continue to flow to node A until
the client's ARP caches expire. On most systems, the ARP cache expires
every 1-2 minutes.

This means that during a planned failover, you should keep the old
leader node up for a couple of minutes after flipping leadership, so
that it can continue forwarding traffic for old clients until their
ARP caches refresh.

During an unplanned failover, the service IPs will be unreachable
until the clients refresh their ARP cache entries.

This slow failover is a limitation of the current implementation of
ARP mode. Future improvements will eliminate the failover delay.
