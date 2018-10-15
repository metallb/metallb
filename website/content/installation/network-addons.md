---
title: Network Addon Compatibility
---

Generally speaking, MetalLB doesn't care which network addon you
choose to run in your cluster, as long as it provides the standard
behaviors that Kubernetes expects from network addons.

The following is a list of network addons that have been tested with
MetalLB, for your reference. The list is presented in alphabetical
order, we express no preference for one addon over another.

Addons that are not on this list probably work, we just haven't tested
them. Please
[send us a patch]({{% relref "community/_index.md" %}}#contributing) if you
have information on network addons that aren't listed!

Network addon | Compatible
--------------|---------------
Calico        | Partial (see [known issues]({{% relref "configuration/calico.md" %}}))
Cilium        | Yes
Flannel       | Yes
Kube-router   | No ([work in progress](https://github.com/google/metallb/issues/160))
Romana        | Yes (see [guide]({{% relref "configuration/romana.md" %}}) for advanced integration)
Weave Net     | Yes

### IPVS mode in kube-proxy

Starting in Kubernetes 1.9, `kube-proxy` has beta support for a more
efficient "IPVS mode", in addition to the default "iptables mode."
MetalLB is currently **not compatible** with IPVS mode in kube-proxy,
due to several outstanding bugs with IPVS mode's handling of
LoadBalancer services. See
our [tracking bug](https://github.com/google/metallb/issues/153) for
details.
