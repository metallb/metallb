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

## In layer 2 mode, how to specify the host interface for an address pool?

There's no need: MetalLB automatically listens/advertises on all interfaces. That might sound like a problem, but because of the way ARP/NDP works, only clients on the right network will know to look for the service IP on the network.

## Is MetalLB working on OpenStack?

Yes but by default, OpenStack has anti-spoofing protection enabled which prevents the VMs from using any IP that wasn't configured for them in the OpenStack control plane, such as LoadBalancer IPs from MetalLB. See [openstack port set --allowed-address](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/port.html).
