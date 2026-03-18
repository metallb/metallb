---
title: Issues with kube-router
weight: 20
---

From _v2.8.0_, kube-router requires `IPAddressPool`s be allow-listed in the kube-router daemon. To allow the IP address pools, pass the `--loadbalancer-ip-range` argument to the kube-router DaemonSet. An alternative is to disable external IP address validation by passing `--strict-external-ip-validation=false`. This is not recommended in clusters with untrusted tenants. See the [_v2.8.0_ release notes](https://github.com/cloudnativelabs/kube-router/releases/tag/v2.8.0) for more details.

If you use kube-router's builtin external BGP peering mode, you cannot use MetalLB's BGP mode as well.
There are no plans to address this limitation.
