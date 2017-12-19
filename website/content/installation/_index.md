---
title: Installation
weight: 3
---

Installing MetalLB is very simple: just apply the manifest!

```shell
kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml
```

This will deploy MetalLB to your cluster, under the `metallb-system`
namespace. The components in the manifest are:

- The `metallb-system/controller` deployment. This is the cluster-wide
  controller that handles IP address assignments.
- The `metallb-system/speaker` daemonset. This is the component
  that peers with your BGP router(s) or sends out ARP requests
  and announces assigned service IPs to the world.
- Service accounts for the controller and speaker, along with the
  RBAC permissions that the components need to function.

The installation manifest does not include a configuration
file. MetalLB's components will still start, but will remain idle
until you define and deploy a configmap.
