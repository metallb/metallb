---
title: MetalLB
---

# MetalLB

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using standard
routing protocols.

{{% notice note %}}
MetalLB is a young project. You should treat it as a **beta** system.
The [project maturity]({{% relref "concepts/maturity.md" %}}) page
explains what that implies.
{{% /notice %}}

## Why?

Kubernetes does not offer an implementation of network load balancers
([Services of type LoadBalancer](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/))
for bare-metal clusters. The implementations of network load balancers that
Kubernetes does ship with are all glue code that calls out to various
IaaS platforms (GCP, AWS, Azure...). If you're not running on a
supported IaaS platform (GCP, AWS, Azure...), LoadBalancers will
remain in the "pending" state indefinitely when created.

Bare-metal cluster operators are left with two lesser tools to bring
user traffic into their clusters, "NodePort" and "externalIPs"
services. Both of these options have significant downsides for
production use, which makes bare-metal clusters second-class citizens
in the Kubernetes ecosystem.

MetalLB aims to redress this imbalance by offering a network load balancer
implementation that integrates with standard network equipment, so
that external services on bare-metal clusters also "just work" as much
as possible.

## Requirements

MetalLB requires the following to function:

- A [Kubernetes](https://kubernetes.io) cluster, running Kubernetes
  1.13.0 or later, that does not already have network load-balancing
  functionality.
- A
  [cluster network configuration]({{% relref "installation/network-addons.md" %}}) that
  can coexist with MetalLB.
- Some IPv4 addresses for MetalLB to hand out.
- When using the BGP operating mode, you will need one or more routers
  capable of speaking [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol).
- When using the L2 operating mode, traffic on port 7946 (TCP & UDP, other port can be configured) must be allowed between nodes, as required by
  [memberlist](https://github.com/hashicorp/memberlist).

## Usage

The [concepts]({{% relref "concepts/_index.md" %}}) section will give
you a primer on what MetalLB does in your cluster. When you're ready
to deploy to a Kubernetes cluster, head to the
[installation](/installation/) and [usage]({{% relref
"usage/_index.md" %}}) guides.

## FRR Mode

MetalLB implements an experimental FRR Mode that uses an [FRR](https://frrouting.org/) container as the backend for handling BGP sessions. It provides features that are not available with the native BGP implementation, such as pairing BGP sessions with BFD sessions, and advertising IPV6 addresses.

The FRR mode is considered to be experimental, please see the [installation](https://metallb.universe.tf/installation/) section for instructions on how to enable it.

## Contributing

We welcome contributions in all forms. Please check out
the [contributing guide]({{% relref "community/_index.md" %}}) for more
information.

One lightweight way you can contribute is
to
[tell us that you're using MetalLB](https://github.com/metallb/metallb/issues/5),
which will give us warm fuzzy feelings :).

We are a [Cloud Native Computing Foundation](https://cncf.io/) sandbox project.
![cncf](/images/cncf-color.png)
