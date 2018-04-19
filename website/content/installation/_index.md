---
title: Installation
weight: 3
---

Before starting with installation, make sure you meet all
the [requirements]({{% relref "/_index.md" %}}#requirements). In
particular, you should pay attention
to
[network addon compatibility]({{% relref "installation/network-addons.md" %}}).

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

## Installation with Helm

{{% notice note %}}
Due to code review turnaround time, it usually takes a few days after
each MetalLB release before the Helm chart is updated in the stable
repository.

Currently, the Helm chart is **not** up to date with the latest
release of MetalLB. If you need to use the latest release, please use
an alternate installation method.
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
putting the configuration under the `config.inline` key in your
`values.yaml`.
