---
title: MetalLB in layer 2 mode
weight: 1
---

In layer 2 mode, one node assumes the responsibility of advertising a service to
the local network. From the network's perspective, it simply looks like that
machine has multiple IP addresses assigned to its network interface.

Under the hood, MetalLB responds to
[ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol) requests for
IPv4 services, and
[NDP](https://en.wikipedia.org/wiki/Neighbor_Discovery_Protocol) requests for
IPv6.

The major advantage of the layer 2 mode is its universality: it will work on any
Ethernet network, with no special hardware required, not even fancy routers.

## Load-balancing behavior

In layer 2 mode, all traffic for a service IP goes to one node. From there,
`kube-proxy` spreads the traffic to all the service's pods.

In that sense, layer 2 does not implement a load balancer. Rather, it implements
a failover mechanism so that a different node can take over should the current
leader node fail for some reason.

If the leader node fails for some reason, failover is automatic: the failed
node is detected using [memberlist](https://github.com/hashicorp/memberlist),
at which point new nodes take over ownership of the IP addresses from the
failed node.

## Limitations

Layer 2 mode has two main limitations you should be aware of: single-node
bottlenecking, and potentially slow failover.

As explained above, in layer2 mode a single leader-elected node receives all
traffic for a service IP. This means that your service's ingress bandwidth is
limited to the bandwidth of a single node. This is a fundamental limitation of
using ARP and NDP to steer traffic.

In the current implementation, failover between nodes depends on cooperation
from the clients. When a failover occurs, MetalLB sends a number of gratuitous
layer 2 packets (a bit of a misnomer - it should really be called "unsolicited
layer 2 packets") to notify clients that the MAC address associated with the
service IP has changed.

Most operating systems handle "gratuitous" packets correctly, and update their
neighbor caches promptly. In that case, failover happens within a few
seconds. However, some systems either don't implement gratuitous handling at
all, or have buggy implementations that delay the cache update.

All modern versions of major OSes (Windows, Mac, Linux) implement layer 2
failover correctly, so the only situation where issues may happen is with older
or less common OSes.

To minimize the impact of planned failover on buggy clients, you should keep the
old leader node up for a couple of minutes after flipping leadership, so that it
can continue forwarding traffic for old clients until their caches refresh.

During an unplanned failover, the service IPs will be unreachable until the
buggy clients refresh their cache entries.

If you encounter a situation where layer 2 mode failover is slow (more than
about 10s), please [file a bug](https://github.com/metallb/metallb/issues/new)!
We can help you investigate and determine if the issue is with the client, or a
bug in MetalLB.

## How the L2 leader election works

The election of the "leader" (the node which is going to advertise the IP) of a
given loadbalancer IP is stateless and works in the following way:

- each speaker collects the list of the potential announcers of a given IP, taking
into account active speakers, external traffic policy, active endpoints, node selectors and other things.
- each speaker does the same computation: it gets a sorted list of a hash of "node+VIP" elements and
announces the service if it is the first item of the list.

This removes the need of having to keep memory of which speaker is in charge of
announcing a given IP.

### Adding or removing nodes

Given the leader election algoritm described above, removing a node does not change the
speaker announcing the VIP, while adding a node will change it only if it becomes the new
first element of the list.

### Brain split behaviour

Given the stateless nature of the mechanism, if a speaker mistakenly detects a set of nodes as
non active, it might calculate a different list, resulting in multiple (or no) speakers announcing the
same VIP.

## Comparison to Keepalived

MetalLB's layer2 mode has a lot of similarities to Keepalived, so if you're
familiar with Keepalived, this should all sound fairly familiar. However, there
are also a few differences worth mentioning. If you aren't familiar with
Keepalived, you can skip this section.

Keepalived uses the Virtual Router Redundancy Protocol (VRRP). Instances of
Keepalived continuously exchange VRRP messages with each other, both to select a
leader and to notice when that leader goes away.

MetalLB on the other hand relies on
[memberlist](https://github.com/hashicorp/memberlist) to know when a node in
the cluster is no longer reachable and the service IPs from that node should be
moved elsewhere.

Keepalived and MetalLB "look" the same from the client's perspective: the
service IP address seems to migrate from one machine to another when a failover
occurs, and the rest of the time it just looks like machines have more than one
IP address.

Because it doesn't use VRRP, MetalLB isn't subject to some of the limitations of
that protocol. For example, the VRRP limit of 255 load balancers per network
doesn't exist in MetalLB. You can have as many load-balanced IPs as you want, as
long as there are free IPs in your network. MetalLB also requires less
configuration than VRRP--for example, there are no Virtual Router IDs.

On the flip side, because MetalLB relies on
[memberlist](https://github.com/hashicorp/memberlist) for cluster membership
information, it cannot interoperate with third-party VRRP-aware routers and
infrastructure. This is working as intended: MetalLB is specifically designed
to provide load balancing and failover _within_ a Kubernetes cluster, and in
that scenario interoperability with third-party LB software is out of scope.
