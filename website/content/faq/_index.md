---
title: Frequently Asked Questions
weight: 6
---

## Can I have several address pools?

Yes, for example:
```
addresses:
- 192.168.12.0/24
- 192.168.144.0/20
```

You can even name them and then specify which pool to draw from.  See [usage]({{% relref "usage/_index.md" %}}) for using annotations to specify which IP pool and address as part of defining your LoadBalancer.

## In layer 2 mode, how to specify the host interface for an address pool?

There's no need: MetalLB automatically listens/advertises on all interfaces. That might sound like a problem, but because of the way ARP/NDP works, only clients on the right network will know to look for the service IP on the network.

*NOTE* Because of the way layer 2 mode functions, this works with tagged vlans as well.  Specify the network and the ip stack figures out the rest.

## Is MetalLB working on OpenStack?

Yes but by default, OpenStack has anti-spoofing protection enabled which prevents the VMs from using any IP that wasn't configured for them in the OpenStack control plane, such as LoadBalancer IPs from MetalLB. See [openstack port set --allowed-address](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/port.html).

## Can I update one pool at a time?

Not yet.  The whole configuration for MetalLB is stored in a ConfigMap called `config` in the metallb-system namespace. We anticipate converting to Custom Resources at some point as described in [Issue 196](https://github.com/metallb/metallb/issues/196).  Contributions Welcome! 