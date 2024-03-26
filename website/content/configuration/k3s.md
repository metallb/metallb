---
title: Issues with K3s
weight: 20
---

## Conflicts with K3s' own LoadBalancer implementation

K3s comes with its own service load balancer named Klipper. You need to disable it or use [loadBalancerClass](https://metallb.universe.tf/installation/#setting-the-loadbalancer-class) in order to run MetalLB properly.

To disable Klipper, run the server with the `--disable servicelb` option, as described in [K3s documentation](https://rancher.com/docs/k3s/latest/en/networking/).

## Exposing K3s' bundled ingress controller

A common use-case for MetalLB is to expose the ingress controller of a cluster to the outside world.
K3s comes with [Traefik](https://traefik.io/) ingress controller, which is enabled by default.

The Traefik pods deployed by K3s tolerate the node-taint `CriticalAddonsOnly=true:NoExecute` by default, while the MetalLB `speaker` pods do not.
While this is not an issue in most cases, you may run into issues when configuring your services with `externalTrafficPolicy: Local` 
if your cluster's control-plane happens to be tainted with this specific taint.

An option to make this work is adding the toleration for the taint `CriticalAddonsOnly=true:NoExecute` to your `speaker` pods. 
This will allow Kubernetes to schedule the `speaker` pods of MetalLB to all nodes that may also run K3s' own traefik pods, making `externalTrafficPolicy: Local` possible.
