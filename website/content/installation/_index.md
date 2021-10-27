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

There are three supported ways to install MetalLB: using plain Kubernetes
manifests, using Kustomize, or using Helm.

## Preparation

If you're using kube-proxy in IPVS mode, since Kubernetes v1.14.2 you have to enable strict ARP mode.

*Note, you don't need this if you're using kube-router as service-proxy because it is enabling strict ARP by default.*

You can achieve this by editing kube-proxy config in current cluster:

```bash
kubectl edit configmap -n kube-system kube-proxy
```

and set:

```yaml
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: "ipvs"
ipvs:
  strictARP: true
```

You can also add this configuration snippet to your kubeadm-config, just append it with `---` after the main configuration.

If you are trying to automate this change, these shell snippets may help you:

```bash
# see what changes would be made, returns nonzero returncode if different
kubectl get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: false/strictARP: true/" | \
kubectl diff -f - -n kube-system

# actually apply the changes, returns nonzero returncode on errors only
kubectl get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: false/strictARP: true/" | \
kubectl apply -f - -n kube-system
```

## Installation by manifest

To install MetalLB, apply the manifest:

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/main/manifests/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/main/manifests/metallb.yaml
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
[Kustomize](https://github.com/kubernetes-sigs/kustomize) by pointing
at the remote kustomization file :

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/metallb/metallb//manifests?ref=v0.9.3
  - configmap.yml 
```

If you want to use a
[configMapGenerator](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configGeneration.md)
for config file, you want to tell Kustomize not to append a hash to
the config map, as MetalLB is waiting for a config map named `config`
(see
<https://github.com/kubernetes-sigs/kustomize/blob/master/examples/generatorOptions.md>):

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/metallb/metallb//manifests?ref=v0.9.3

configMapGenerator:
- name: config
  files:
    - configs/config

generatorOptions:
 disableNameSuffixHash: true
```

## Installation with Helm

You can install MetallLB with [Helm](https://helm.sh/)
by using the Helm chart repository: `https://metallb.github.io/metallb`

```bash
helm repo add metallb https://metallb.github.io/metallb
helm install metallb metallb/metallb
```

A values file may be specified on installation. This is recommended for providing configs in Helm values:

```bash
helm install metallb metallb/metallb -f values.yaml
```

MetalLB configs are set in `values.yaml` under `configInLine`:

```yaml
configInline:
  address-pools:
   - name: default
     protocol: layer2
     addresses:
     - 198.51.100.0/24
```

## Upgrade

When upgrading MetalLB, always check the [release notes](https://metallb.universe.tf/release-notes/)
to see the changes and required actions, if any. Pay special attention to the release notes when
upgrading to newer major/minor releases.

Unless specified otherwise in the release notes, upgrade MetalLB either using
[plain manifests](#installation-by-manifest) or using [Kustomize](#installation-with-kustomize) as
described above.

Please take the known limitations for [layer2](https://metallb.universe.tf/concepts/layer2/#limitations)
and [bgp](https://metallb.universe.tf/concepts/bgp/#limitations) into account when performing an
upgrade.
