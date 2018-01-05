---
title: Concepts
weight: 1
---

MetalLB hooks into your Kubernetes cluster, and provides a network
load-balancer implementation. In short, it allows you to create
Kubernetes services of type "LoadBalancer" in clusters that don't run
on a cloud provider, and thus cannot simply hook into paid products to
provide load-balancers.

It has two features that work together to provide this service:
address allocation, and external announcement.

## Address allocation

In a cloud-enabled Kubernetes cluster, you request a load-balancer,
and your cloud platform assigns an IP address to you. In a bare metal
cluster, MetalLB is responsible for that allocation.

MetalLB cannot create IP addresses out of thin air, so you do have to
give it _pools_ of IP addresses that it can use. MetalLB will take
care of assigning and unassigning individual addresses as services
come and go, but it will only ever hand out IPs that are part of its
configured pools.

How you get IP address pools for MetalLB depends on your
environment. If you're running a bare metal cluster in a colocation
facility, your hosting provider probably offers IP addresses for
lease. In that case, you would lease, say, a /26 of IP space (64
addresses, and provide that range to MetalLB for cluster services.

Alternatively, your cluster might be purely private, providing
services to a nearby LAN but not exposed to the internet. In that
case, you could pick a range of IPs from one of the private address
spaces (so-called RFC1918 addresses), and assign those to
MetalLB. Such addresses are free, and work fine as long as you're only
providing cluster services to your LAN.

Or, you could do both! MetalLB lets you define as many address pools
as you want, and doesn't care what "kind" of addresses you give it.

## External announcement

Once MetalLB has assigned an external IP address to a service, it
needs to make the network beyond the cluster aware that the IP "lives"
in the cluster. MetalLB uses standard routing protocols to achieve
this, one of ARP or BGP.

### ARP

In ARP mode, one machine in the cluster takes ownership of the service
IPs, and uses
standard
[ARP](https://en.wikipedia.org/wiki/Address_Resolution_Protocol)
packets to make those IPs reachable. From the LAN's point of view, the
announcing machine simply has multiple IP addresses.

The [ARP mode]({{% relref "arp.md" %}}) sub-page has more details on
ARP mode's operation and limitations.

### BGP 

In BGP mode, all machines in the cluster
establish [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol)
peering sessions with nearby routers that you control, and tell those
routers how to forward traffic to the service IPs. Using BGP allows
for true load balancing across multiple nodes, and fine-grained
traffic control thanks to BGP's policy mechanisms.

The [BGP mode]({{% relref "bgp.md" %}}) sub-page has more details on
BGP mode's operation and limitations.
