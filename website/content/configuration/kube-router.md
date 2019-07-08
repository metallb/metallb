---
title: Issues with kube-router
weight: 20
---

MetalLB should work out of the box with kube-router, with one
exception: if you use kube-router's builtin external BGP peering mode,
you cannot use MetalLB's BGP mode as well.

There are no plans to address this limitation.
