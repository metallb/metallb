# MetalLB

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using BGP.

[![Project maturity: alpha](https://img.shields.io/badge/maturity-alpha-yellow.svg)](docs/maturity.md) [![license](https://img.shields.io/github/license/google/metallb.svg?maxAge=2592000)](https://github.com/google/netboot/blob/master/LICENSE) [![Travis](https://img.shields.io/travis/google/metallb.svg?maxAge=2592000)](https://travis-ci.org/google/netboot) [![Quay.io](https://img.shields.io/badge/containers-ready-green.svg)](https://quay.io/metallb) [![Go report card](https://goreportcard.com/badge/github.com/google/metallb)](https://goreportcard.com/report/github.com/google/metallb)

# Why?

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

# Requirements

MetalLB requires the following to function:

- A [Kubernetes](https://kubernetes.io) cluster, running Kubernetes
  1.8.0 or later, that does not already have network load-balancing
  functionality.
- One or
  more [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol)
  capable routers that support 4-byte AS numbers
  ([RFC 6793](https://tools.ietf.org/html/rfc6793)).
- Some IPv4 addresses for MetalLB to hand out.

The [requirements](docs/requirements.md) page goes into more detail.

You should also note that MetalLB is currently a young project, so you
should treat it as an "alpha"
product. The [project maturity](docs/maturity.md) page explains what
that implies.

# Usage

Want to test-drive MetalLB? Follow the [tutorial](docs/tutorial.md) to
set up a self-contained MetalLB in minikube.

Deploying to a real cluster? Familiarize yourself with
the [concepts and limitations](docs/concepts-limitations.md), then
head to
the [installation, configuration and usage](docs/installation.md)
guide.

# Contributing

We welcome contributions in all forms. Please check out
the [contributing guide](CONTRIBUTING.md) for more information, and
the [hacking guide](docs/hacking.md) for some technical pointers.

One lightweight way you can contribute is
to
[tell us that you're using MetalLB](https://github.com/google/metallb/issues/5),
which will give us warm fuzzy feelings :).

# Disclaimer

This is not an official Google project, it is just code that happens
to be owned by Google.
