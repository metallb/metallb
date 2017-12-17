---
title: MetalLB
type: index
weight: 2
---

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using BGP or ARP.

## Why?

Kubernetes does not offer an implementation of network load-balancers (Service
objects with `spec.type=LoadBalancer`) for bare metal clusters. The
implementations of Network LB that Kubernetes does ship with are all glue code
that calls out to various IaaS platforms (GCP, AWS, Azure...). If you're not
running on one of those platforms, `LoadBalancer`s will remain in the "pending"
state indefinitely when created.

Bare metal cluster operators are left with two lesser tools to bring user
traffic into their clusters, `NodePort` and `externalIPs` services. Both of
these options have significant downsides for production use, which makes bare
metal clusters second class citizens in the Kubernetes ecosystem.

MetalLB aims to redress this imbalance by offering a Network LB implementation
that integrates with standard network equipment, so that external services on
bare metal clusters also "just work" as much as possible.

## Requirements

MetalLB requires the following to function:

- A [Kubernetes](https://kubernetes.io) cluster, running Kubernetes
  1.8.0 or later, that does not already have network load-balancing
  functionality.
- One or
  more [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol)
  capable routers that support 4-byte AS numbers
  ([RFC 6793](https://tools.ietf.org/html/rfc6793)) or, in the case of ARP, no new equipment is
  needed at all.
- Some IPv4 addresses for MetalLB to hand out.

The [requirements]({{% relref "installation.md#requirements" %}}) page goes into more detail.

You should also note that MetalLB is currently a young project, so you
should treat it as an "alpha"
product. The [project maturity]({{% relref "maturity.md" %}}) page explains what
that implies.

## Usage

Want to test-drive MetalLB? Follow the [tutorial]({{% relref "tutorial.md" %}}) to
set up a self-contained MetalLB in minikube.

Deploying to a real cluster? Head to
the [installation]({{% relref "installation.md" %}})
and [usage]({{% relref "usage.md" %}}) guides.

You might also find the [design document]({{% relref "design.md" %}})
useful to better understand how MetalLB operates.

## Contributing

We welcome contributions in all forms. Please check out
the [contributing guide]({{% relref "hacking.md" %}}) for more
information.

One lightweight way you can contribute is
to
[tell us that you're using MetalLB](https://github.com/google/metallb/issues/5),
which will give us warm fuzzy feelings :).
