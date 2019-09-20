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

There are two supported ways to install MetalLB: using Kubernetes
manifests, or using the [Helm](https://helm.sh) package manager.

## Installation with Kubernetes manifests

To install MetalLB, simply apply the manifest:

```shell
kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml
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

You can install MetalLB with [kustomize](https://github.com/kubernetes-sigs/kustomize) by pointing on the remote kustomization file :

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/danderson/metallb//manifests?ref=v0.8.2
  - configmap.yml 
```
If you want to use a [configMapGenerator](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configGeneration.md) for config file, you want to tell kustomize not to append a hash to the configMap, as MetalLB is waiting for a configMap named `config` (see [https://github.com/kubernetes-sigs/kustomize/blob/master/examples/generatorOptions.md](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/generatorOptions.md)):

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

## Installation with Helm

{{% notice note %}} Due to code review turnaround time, it usually
takes a few days after each MetalLB release before the Helm chart is
updated in the stable repository.

If you're coming here shortly after a new release, you may end up
installing an older version of MetalLB if you use Helm. This mismatch
usually gets fixed within 2-3 days.
{{% /notice %}}

MetalLB maintains a Helm package in the `stable` package
repository. If you use the Helm package manager in your cluster, you
can install MetalLB that way.

```
helm install --name metallb stable/metallb
```

{{% notice warning %}}
Although Helm allows you to easily deploy multiple releases at the
same time, you should _not_ do this with MetalLB! Multiple copies of
MetalLB will conflict with each other and lead to cluster instability.
{{% /notice %}}

By default, the helm chart looks for MetalLB configuration in the
`metallb-config` ConfigMap, in the namespace you deployed to. It's up
to you
to [define and deploy]({{% relref "../configuration/_index.md" %}})
that configuration.

Alternatively, you can manage the configuration with Helm itself, by
putting the configuration under the `configInline` key in your
`values.yaml`.
