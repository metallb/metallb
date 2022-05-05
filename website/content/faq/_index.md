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

## Does MetalLB work on OpenStack?

Yes but by default, OpenStack has anti-spoofing protection enabled which prevents the VMs from using any IP that wasn't configured for them in the OpenStack control plane, such as LoadBalancer IPs from MetalLB. See [openstack port set --allowed-address](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/port.html).
