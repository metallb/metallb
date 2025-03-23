---
title: Frequently Asked Questions
weight: 6
---

## Can I have several address pools?

Yes, a given `IPAddressPool` can allocate multiple IP ranges, and you can have multiple instances, for example:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.10.0/24
```

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: second-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.9.1-192.168.9.5
  - fc00:f853:0ccd:e799::/124
```

You can even specify which pool to draw from using their name. See [usage]({{% relref "usage/_index.md" %}}) for using annotations to specify which IP pool and address as part of defining your LoadBalancer.

## In layer 2 mode, how to specify the host interface for an address pool?

There's no need: MetalLB automatically listens/advertises on all interfaces. That might sound like a problem, but because of the way ARP/NDP works, only clients on the right network will know to look for the service IP on the network.

*NOTE* Because of the way layer 2 mode functions, this works with tagged vlans as well.  Specify the network and the ip stack figures out the rest.

## Can I have the same service advertised via L2 and via BGP?

Yes. This is achieved by simply having an `L2Advertisement` and a `BGPAdvertisement` referencing the same `IPAddressPool`.

In the most simple form:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.10.0/24
```

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: bgp
  namespace: metallb-system
```

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: l2
  namespace: metallb-system
```

## How can I understand which node advertises a given Service?
In addition to logs, Kubernetes Events and Prometheus metrics, it is possible to understand the advertisement status via the ServiceL2Status and ServiceBGPStatus resources that the speakers manage, for example:
```
$ kubectl get servicel2statuses -n metallb-system
NAME       ALLOCATED NODE       SERVICE NAME   SERVICE NAMESPACE
l2-r8jwb   kind-worker2         service1       ns1
l2-svkqj   kind-worker          service2       ns2

$ kubectl get servicebgpstatuses -n metallb-system
NAME        NODE                 SERVICE NAME   SERVICE NAMESPACE
bgp-82jzt   kind-worker2         service4       ns4
bgp-b8fmt   kind-worker2         service3       ns3
bgp-bt56x   kind-worker          service4       ns4
bgp-c64s2   kind-worker          service3       ns3
```
Each resource is labeled with "metallb.io/node", "metallb.io/service-name", "metallb.io/service-namespace", with the matching node / service name / service namespace respectively. This makes it easier to list the statuses related to a given combination, for example:
```
$ kubectl get servicebgpstatuses -n metallb-system -l metallb.io/service-name="service3",metallb.io/service-namespace="ns3",metallb.io/node=kind-worker
NAME        NODE          SERVICE NAME   SERVICE NAMESPACE
bgp-c64s2   kind-worker   service3       ns3
```
As the API docs mention, the ServiceBGPStatus resource only represents the intention of the node to advertise the Service to a set of peers, where the actual advertisements depend on the status of the corresponding BGP sessions. The status of a BGP session can be understood via metrics / logs, or in the case of [the FRR-k8s mode](https://metallb.io/concepts/bgp/index.html#frr-k8s-mode) via its BGPSessionState resource.

## Does MetalLB work on OpenStack?

Yes but by default, OpenStack has anti-spoofing protection enabled which prevents the VMs from using any IP that wasn't configured for them in the OpenStack control plane, such as LoadBalancer IPs from MetalLB. See [openstack port set --allowed-address](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/port.html).
