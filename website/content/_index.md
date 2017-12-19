---
title: MetalLB
---

# MetalLB

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using standard
routing protocols.

{{% notice note %}}
MetalLB is a young project, so you should treat it as an **alpha**
system. The [project maturity]({{% relref "maturity.md" %}}) page
explains what that implies.
{{% /notice %}}

## Why?

Kubernetes does not offer an implementation of network load-balancers
([Services of type LoadBalancer](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/))
for bare metal clusters. The implementations of Network LB that
Kubernetes does ship with are all glue code that calls out to various
IaaS platforms (GCP, AWS, Azure...). If you're not running on a
supported IaaS platform (GCP, AWS, Azure...), LoadBalancers will
remain in the "pending" state indefinitely when created.

Bare metal cluster operators are left with two lesser tools to bring
user traffic into their clusters, "NodePort" and "externalIPs"
services. Both of these options have significant downsides for
production use, which makes bare metal clusters second class citizens
in the Kubernetes ecosystem.

MetalLB aims to redress this imbalance by offering a Network LB
implementation that integrates with standard network equipment, so
that external services on bare metal clusters also "just work" as much
as possible.

## Requirements

MetalLB requires the following to function:

- A [Kubernetes](https://kubernetes.io) cluster, running Kubernetes
  1.8.0 or later, that does not already have network load-balancing
  functionality.
- Some IPv4 addresses for MetalLB to hand out.
- Depending on the operating mode, you may need one or more routers
  capable of
  speaking
  [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol)
  or
  [RIP](https://en.wikipedia.org/wiki/Routing_Information_Protocol).

## Usage

Want to test-drive MetalLB? Follow
the [tutorial]({{% relref "tutorial.md" %}}) to set up a
self-contained MetalLB in minikube.

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
