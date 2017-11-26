# MetalLB

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using BGP.

[![license](https://img.shields.io/github/license/google/metallb.svg?maxAge=2592000)](https://github.com/google/netboot/blob/master/LICENSE) [![Travis](https://img.shields.io/travis/google/metallb.svg?maxAge=2592000)](https://travis-ci.org/google/netboot) [![Quay.io](https://img.shields.io/badge/containers-ready-green.svg)](https://quay.io/metallb) [![Go report card](https://goreportcard.com/badge/github.com/google/metallb)](https://goreportcard.com/report/github.com/google/metallb)

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

# Usage

Install MetalLB on your cluster by applying the MetalLB manifest.

(TODO: write more of this)

# Disclaimer

This is not an official Google project, it is just code that happens
to be owned by Google.
