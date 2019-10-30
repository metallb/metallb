---
title: Installation
weight: 3
---

Before starting with installation, make sure you meet all the
[requirements]({{% relref "/_index.md" %}}#requirements). In
particular, you should pay attention to [network addon
compatibility]({{% relref "installation/network-addons.md" %}}).

If you're trying to run MetalLB on a cloud platform, you should also
look at the [cloud compatibility]({{% relref "installation/clouds.md"
%}}) page and make sure your cloud platform can work with MetalLB
(most cannot).

There is two supported ways to install MetalLB: using plain Kubernetes
manifests, or using Kustomize.

## Installation by manifest

To install MetalLB, apply the manifest:

```shell
kubectl apply -f https://raw.githubusercontent.com/google/metallb/main/manifests/metallb.yaml
```

This will deploy MetalLB to your cluster, under the `metallb-system`
namespace. The components in the manifest are:

- The `metallb-system/controller` deployment. This is the cluster-wide
  controller that handles IP address assignments.
- The `metallb-system/speaker` daemonset. This is the component that
  speaks the protocol(s) of your choice to make the services
  reachable.
- Service accounts for the controller and speaker, along with the
  RBAC permissions that the components need to function.

The installation manifest does not include a configuration
file. MetalLB's components will still start, but will remain idle
until
you
[define and deploy a configmap]({{% relref "../configuration/_index.md" %}}).

## Installation with kustomize

You can install MetalLB with
[kustomize](https://github.com/kubernetes-sigs/kustomize) by pointing
on the remote kustomization fle :

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/danderson/metallb//manifests?ref=v0.8.2
  - configmap.yml 
```

If you want to use a
[configMapGenerator](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configGeneration.md)
for config file, you want to tell kustomize not to append a hash to
the configMap, as MetalLB is waiting for a configMap named `config`
(see
[https://github.com/kubernetes-sigs/kustomize/blob/master/examples/generatorOptions.md](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/generatorOptions.md)):

```
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/danderson/metallb//manifests?ref=v0.8.2

configMapGenerator:
- name: config
  files:
    - configs/config

generatorOptions:
 disableNameSuffixHash: true
```
