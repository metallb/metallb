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


| Network addon | Compatible                                                                                                              |
|---------------|-------------------------------------------------------------------------------------------------------------------------|
| Antrea        | Yes (Tested on version [1.4 and 1.5](https://github.com/jayunit100/k8sprototypes/tree/master/kind/metallb-antrea))      |
| Calico        | Mostly (see [known issues]({{% relref "configuration/calico.md" %}}))                                                   |
| Canal         | Yes                                                                                                                     |
| Cilium        | Yes                                                                                                                     |
| Flannel       | Yes                                                                                                                     |
| Kube-ovn      | Yes                                                                                                                     |
| Kube-router   | Mostly (see [known issues]({{% relref "configuration/kube-router.md" %}}))                                              |

