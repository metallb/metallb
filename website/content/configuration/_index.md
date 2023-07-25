---
title: Configuration
weight: 4
---

MetalLB remains idle until configured. This is accomplished by
creating and deploying various resources into **the same namespace**
(metallb-system) MetalLB is deployed into.

There are various examples of the configuration CRs in
[`configsamples`](https://github.com/metallb/metallb/tree/main/configsamples).

Also, the API is [fully documented here](../apis/).

{{% notice note %}}
If you installed MetalLB with Helm, you will need to change the
namespace of the CRs to match the namespace in which MetalLB was
deployed.
{{% /notice %}}

## Defining the IPs to assign to the Load Balancer services

In order to assign an IP to the services, MetalLB must be instructed to do so via the
`IPAddressPool` CR.

All the IPs allocated via `IPAddressPool`s contribute to the pool of IPs that MetalLB
uses to assign IPs to services.

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.10.0/24
  - 192.168.9.1-192.168.9.5
  - fc00:f853:0ccd:e799::/124
```

Multiple instances of `IPAddressPool`s can co-exist and addresses can be defined by CIDR,
by range, and both IPV4 and IPV6 addresses can be assigned.

## Announce the service IPs

Once the IPs are assigned to a service, they must be announced.

The specific configuration depends on the protocol(s) you want to use
to announce service IPs. Jump to:

- [Layer 2 configuration](#layer-2-configuration)
- [BGP configuration](#bgp-configuration)
- [Advanced BGP configuration](./_advanced_bgp_configuration)
- [Advanced L2 configuration](./_advanced_l2_configuration)
- [Advanced IPAddressPool configuration](./_advanced_ipaddresspool_configuration)

Note: it is possible to announce the same service both via L2 and via BGP (see the relative
[FAQ](../faq/_index.md)).

## Layer 2 configuration

Layer 2 mode is the simplest to configure: in many cases, you don't
need any protocol-specific configuration, only IP addresses.

Layer 2 mode does not require the IPs to be bound to the network interfaces
of your worker nodes. It works by responding to ARP requests on your local
network directly, to give the machine's MAC address to clients.

In order to advertise the IP coming from an `IPAddressPool`, an `L2Advertisement`
instance must be associated to the `IPAddressPool`.

For example, the following configuration gives MetalLB control over
IPs from `192.168.1.240` to `192.168.1.250`, and configures Layer 2
mode:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.1.240-192.168.1.250
```

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
```

Setting no `IPAddressPool` selector in an `L2Advertisement` instance is interpreted
as that instance being associated to all the `IPAddressPool`s available.

So in case there are specialized `IPAddressPool`s, and only some of them must be
advertised via L2, the list of `IPAddressPool`s we want to advertise the IPs from
must be declared (alternative, a label selector can be used).

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
```

## BGP configuration

MetalLB needs to be instructed on how to establish a session with one
or more external BGP routers.

In order to do so, an instance of `BGPPeer` must be created for each
router we want metallb to connect to.

For a basic configuration featuring one BGP router and one IP address
range, you need 4 pieces of information:

- The router IP address that MetalLB should connect to,
- The router's AS number,
- The AS number MetalLB should use,
- An IP address range expressed as a CIDR prefix.

As an example if you want to give MetalLB AS number 64500, and connect
it to a router at 10.0.0.1 with AS number 64501, your configuration
will look like:

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: sample
  namespace: metallb-system
spec:
  myASN: 64500
  peerASN: 64501
  peerAddress: 10.0.0.1
```

Given an `IPAddressPool` like:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 192.168.1.240-192.168.1.250
```

MetalLB must be configured to advertise the IPs coming from it
via BGP.

This is done via the `BGPAdvertisement` CR.

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: example
  namespace: metallb-system
```

Setting no `IPAddressPool` selector in a `BGPAdvertisement` instance is interpreted
as that instance being associated to all the `IPAddressPool`s available.

So in case there are specialized `AddressPool`s, and only some of them must be
advertised via BGP, the list of `ipAddressPool`s we want to advertise the IPs from
must be declared (alternative, a label selector can be used).

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
```

### Enabling BFD support for BGP sessions

With the FRR mode, BGP sessions can be backed up by BFD sessions in order to provide a quicker path failure detection than BGP alone provides.

In order to enable BFD, a BFD profile must be added and referenced by a given peer:

```yaml
apiVersion: metallb.io/v1beta1
kind: BFDProfile
metadata:
  name: testbfdprofile
  namespace: metallb-system
spec:
  receiveInterval: 380
  transmitInterval: 270
```

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: peersample
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
  bfdProfile: testbfdprofile
```

## Configuration validation

MetalLB ships validation webhooks that check the validity of the CRs applied.

However, due to the fact that the global MetalLB configuration is composed by different pieces, not all of the
invalid configurations are blocked by those webhooks. Because of that, if a non valid MetalLB configuration
is applied, MetalLB discards it and keeps using the last valid configuration.

In future releases MetalLB will expose misconfigurations as part of Kubernetes resources,
but currently the only way to understand why the configuration was not loaded is by checking
the controller's logs.
