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
  ([RFC 6793](https://tools.ietf.org/html/rfc6793))
- Some IPv4 addresses for MetalLB to hand out

## Compatibility with Kubernetes deployments

MetalLB is a good fit for bare metal (aka "on premises") clusters:
- [Kubeadm](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/)-managed clusters
- [Tectonic](https://coreos.com/tectonic/) on bare metal
- [Minikube](https://github.com/kubernetes/minikube) sandboxes

Kubernetes's current design does not permit several load-balancing
implementations to coexist. This means that MetalLB will _not_ work with
fully hosted cloud Kubernetes solutions like:
- [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/)
- [Azure Container Service](https://azure.microsoft.com/en-us/services/container-service/)

If you're using a cloud provider as a pure VM host, MetalLB can be
made to work, with unusual custom configuration. The following
deployment types are _not recommended_ for first-time users:
- [Google Compute Engine](https://kubernetes.io/docs/getting-started-guides/gce/)
- [Azure ACS-Engine](https://github.com/Azure/acs-engine/blob/master/docs/kubernetes.md)
- [Amazon Elastic Compute Cloud](https://kubernetes.io/docs/getting-started-guides/aws/)


# Usage

Install MetalLB on your cluster by applying the MetalLB manifest.

(TODO: write more of this)

# Disclaimer

This is not an official Google project, it is just code that happens
to be owned by Google.
