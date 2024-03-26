---
title: Issues with K3s
weight: 20
---

## Conflicts with K3s' own LoadBalancer implementation

K3s come with its own service load balancer named Klipper. You need to disable it in order to run MetalLB.
To disable Klipper, run the server with the `--disable servicelb` option, as described in [K3s documentation](https://rancher.com/docs/k3s/latest/en/networking/).

As an alternative to disabling Klipper-LB you can configure MetalLB to only recognizes services that have been configured with a specific `loadBalancerClass` attribute,
as described by the [installation documentation](https://metallb.universe.tf/installation/#setting-the-loadbalancer-class).

For example, if you run the MetalLB `controller` and `speaker` pods with `--lb-class=metallb.universe.tf/metallb-class`, you can then directly reference this class in your services:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: traefik-metallb
  annotations:
    metallb.universe.tf/address-pool: ingress-ip-pool
spec:
  type: LoadBalancer
  loadBalancerClass: metallb.universe.tf/metallb-class
  ports:
  - port: 443
    targetPort: 8443
```

Klipper-LB will ignore this service due to its configured `loadBalancerClass`.

## Exposing K3s' bundled ingress controller

A common use-case for MetalLB is to expose the ingress controller of a cluster to the outside world.
K3s comes with [Traefik](https://traefik.io/) ingress controller, which is enabled by default.

The Traefik pods deployed by K3s tolerate the node-taint `CriticalAddonsOnly=true:NoExecute` by default, while the MetalLB `speaker` pods do not.
While this is not an issue in most cases, you may run into issues when configuring your services with `externalTrafficPolicy: Local` 
if your cluster's control-plane happens to be tainted with this specific taint.

To make this work, you need to add a toleration to your `speaker` pods. 
For example, when using Kustomize, you can add the toleration like this:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - github.com/metallb/metallb/config/native?ref=<METALLB_VERSION>

patches:
  - target:
      kind: DaemonSet
      name: speaker
    patch: |-
      - op: add
        path: /spec/template/spec/tolerations/-
        value:
          effect: NoExecute
          key: CriticalAddonsOnly
          operator: Exists
```

This will allow Kubernetes to schedule the `speaker` pods of MetalLB to all nodes that may also run K3s' own traefik pods, making `externalTrafficPolicy: Local` possible.
