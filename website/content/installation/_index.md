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
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/main/config/manifests/metallb-native.yaml
```

{{% notice note %}}
If you want to deploy MetalLB using the [FRR mode](https://metallb.universe.tf/configuration/#enabling-bfd-support-for-bgp-sessions), apply the manifests:

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/main/config/manifests/metallb-frr.yaml
```

Please do note that these manifests deploy MetalLB from the main development branch. We highly encourage cloud operators to deploy a stable released version of MetalLB on production environments!

{{% /notice %}}

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
until you [start deploying resources]({{% relref "../configuration/_index.md" %}}).

There are also two all-in-one manifests to allow the integration with prometheus. They assume that the
prometheus operator is deployed in the `monitoring` namespace using the `prometheus-k8s`
service account. It is suggested to use either the charts or kustomize if they
need to be changed.

{{% notice note %}}

You may notice the "prometheus" variants of the manifests (for example `https://raw.githubusercontent.com/metallb/metallb/main/config/manifests/metallb-native-prometheus.yaml`).
Those manifests rely on a very specific way of deploying Prometheus via the [kube prometheus](https://github.com/prometheus-operator/kube-prometheus) repository, and
are mainly used by our CI, but they might not be compatible to your Prometheus deployment.

{{% /notice %}}

## Installation with kustomize

You can install MetalLB with
[Kustomize](https://github.com/kubernetes-sigs/kustomize) by pointing
at the remote kustomization file.

In the following example, we are deploying MetalLB with the native bgp implementation :

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/metallb/metallb/config/native?ref=main
```

In order to deploy the [FRR mode](https://metallb.universe.tf/configuration/#enabling-bfd-support-for-bgp-sessions):

```yaml
# kustomization.yml
namespace: metallb-system

resources:
  - github.com/metallb/metallb/config/frr?ref=main
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

{{% notice note %}}
The speaker pod requires elevated permission in order to perform its network functionalities.

If you are using MetalLB with a kubernetes version that enforces [pod security admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) (which is beta in k8s
1.23), the namespace MetalLB is deployed to must be labelled with:

```yaml
  labels:
    pod-security.kubernetes.io/enforce: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn: privileged
```

{{% /notice %}}

{{% notice note %}}
If you want to deploy MetalLB using the [FRR mode](https://metallb.universe.tf/configuration/#enabling-bfd-support-for-bgp-sessions), the following value must be set:

```yaml
speaker:
  frr:
    enabled: true
```

{{% /notice %}}

## Using the MetalLB Operator

The MetalLB Operator is available on OperatorHub at [operatorhub.io/operator/metallb-operator](https://operatorhub.io/operator/metallb-operator). It eases the deployment and life-cycle of MetalLB in a cluster and allows configuring MetalLB via CRDs.

{{% notice note %}}
If you want to deploy MetalLB using the [FRR mode](https://metallb.universe.tf/configuration/#enabling-bfd-support-for-bgp-sessions), you must edit the ClusterServiceVersion resource
named `metallb-operator`:

```bash
kubectl edit csv metallb-operator
```

and change the `BGP_TYPE` environment variable of the `manager` container to `frr`:

```yaml
- name: METALLB_BGP_TYPE
  value: frr
```

## FRR daemons logging level

The FRR daemons logging level are configured using the speaker `--log-level` argument following the below mapping:

Speaker log level | FRR log level
------------------|--------------
all, debug        | debugging
info              | informational
warn              | warnings
error             | error
none              | emergencies

To override this behavior, you can set the `FRR_LOGGING_LEVEL` speaker's environment to any [FRR supported value](https://docs.frrouting.org/en/latest/basic.html#clicmd-log-stdout-LEVEL).

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

## Setting the LoadBalancer Class

MetalLB supports [LoadBalancerClass](https://kubernetes.io/docs/concepts/services-networking/service/#load-balancer-class),
which allows multiple load balancer implementations to co-exist. In order to set the loadbalancer class MetalLB should be listening
for, the `--lb-class=<CLASS_NAME>` parameter must be provided to both the speaker and the controller.

The helm charts support it via the `loadBalancerClass` parameter.
