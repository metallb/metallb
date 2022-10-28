# Announce LoadBalancer services from VRFs

## Summary

The purpose of this enhancement is allow MetalLB to peer with external BGP peers
using interfaces belonging to [Linux VRFs](https://www.kernel.org/doc/html/latest/networking/vrf.html),
and to announce traffic from within VRFs.

The VRF support will be available only via the [FRR based implementation](0001-frr.md).

## Motivation

Having interfaces wrapped in VRFs allow to set a different default gateway than
the regular one. This helps to avoid asymmetrical routing in scenarios where the
nodes have multiple interfaces, providing that either the CNI is able to receive
the traffic inside the VRF or the host networking is set up in a way that the VRF
is able to communicate with the default one for the traffic related to the
service's IP.

This is the default behaviour for a client coming from a router connected via an
interface different from the default one.

```none
             ▲
             │
             │
        ┌────┴────┐
        │ eth0    │
        │         │
┌───────┴─────────┴───────────┐
│                             │               ┌──────────────┐
│                             │               │              │
│                             ├────────┐      │              │
│                             │        │      │              │
│                             │ eth1   │◄─────┤     DCGW     │◄───────  Client
│                             │        │      │              │
│                             ├────────┘      │              │
│                             │               │              │
│                      Node   │               └──────────────┘
│                             │
└─────────────────────────────┘
```

And this is how it would look like with VRF:

```none



               ┌─────────┐
               │ eth0    │
               │         │
       ┌───────┴─────────┴───────────┐
       │                             │               ┌──────────────┐
       │                      ┌──────┤               │              │
       │                      │      ├────────┐      │              │
       │                      │      │        │      │              │
       │                      │ VRF  │ eth1   │◄─────┤     DCGW     │◄───────  Client
       │                      │      │        │      │              │
       │                  ────┼────► ├────────┘      │              │
       │                      └──────┤               │              │
       │                      Node   │               └──────────────┘
       │                             │
       └─────────────────────────────┘

```

Other advantages of having VRFs on the node are:

* being able to have more complex routing tables (i.e. multiple DCGWs for HA)
without impacting the node networking
* ease the troubleshooting of the traffic directed to the service
* making it possible to have multiple clients (coming from different interfaces)
sharing the same IP

VRF for BGP is supported by FRR [out of the box](https://docs.frrouting.org/en/latest/bgp.html#clicmd-router-bgp-ASN-vrf-VRFNAME).

### Goals

* Establish a BGP session with a BGP peer using an interface with VRF as master
* Announce a LoadBalancer service to a BGP peer peered via an interface belonging
to a VRF
* Eventually, provide a guideline and an example on how to setup the VRF and the
host routing, knowing that the routing logic might be CNI dependant

### Non-Goals

* Creating the VRF
* Having MetalLB configure the host networking routing to drive the traffic
towards / from the VRF
* Having MetalLB to check the existence of the VRF on the host. What MetalLB will
require to do is to expose the information that the BGP session can't be established,
both via the current Prometheus metrics and via CRDs, in case we are going to
extend the CRDs to report the state of the session.

## Proposal

### User Stories

#### Story 1

As a cluster administrator, I want to declare a BGP session specifying the host
VRF to establish the session from.

#### Story 2

As a cluster administrator, I want to have the LoadBalancer IP announced to BGP
peers connected from within a VRF

## Design Details

### VRF Design proposal

FRR is able to run a bgpd process inside a VRF, so what needs to be done is to
enable the specific configuration.
From the [FRR docs](https://docs.frrouting.org/en/latest/bgp.html#clicmd-router-bgp-ASN-vrf-VRFNAME):

```none
router bgp ASN vrf VRFNAME

    VRFNAME is matched against VRFs configured in the kernel. When vrf VRFNAME is not specified, the BGP protocol process belongs to the default VRF.
```

The proposal here is to define a new `vrf` field in the BGPPeer CR. By setting it,
the speaker will instruct FRR to create a new router section
related to the router and will configure the neighbour inside of it.

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: peer
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 10.2.2.254
  vrf: red
```

Any announcement meant to be performed towards the vrf-ed BGPPeer will be added
in the related router's section.

```none
router bgp 64512 vrf red
  no bgp ebgp-requires-policy
  no bgp network import-check
  no bgp default ipv4-unicast

  bgp router-id 10.1.1.255

  neighbor 10.2.2.254 remote-as 64512
  neighbor 10.2.2.254 ebgp-multihop
  neighbor 10.2.2.254 port 179
  neighbor 10.2.2.254 timers 1 1
  neighbor 10.2.2.254 password password
  neighbor 10.2.2.254 update-source 10.1.1.254

  address-family ipv4 unicast
    neighbor 10.2.2.254 activate
    neighbor 10.2.2.254 route-map 10.2.2.254-red-in in
    network 172.16.1.10/24
    neighbor 10.2.2.254 route-map 10.2.2.254-red-out out
  exit-address-family
```

**Note**: We must be careful in handling the FRR configuration rendering,
because all the route maps are handled in terms of the peer ip. With
VRFs, we can have different BGP peers with the same IP.

**Note 2**: Having multiple BGP peers reached via different VRFs will allow
them to share the same IP. For this reason, we will have to change the
validating webhook logic in order to allow it.

### How to configure host networking

The right combination of host routing through ip rules and iptables rules is
strongly dependant on the CNI. Because of this reason, we don't think it is safe
to have a one size-fit all solution. What we will provide with this change is
a sample script (and possibly a controller) that works for kindnet (in order
to enable CI) and calico, with the idea of extending / validating it with the
other major CNIs.

### Test Plan

**e2e tests**: the dev-env will be extended with an option to have additional
interfaces inside a VRF, and to configure the host network in such a way that
the service is accessible from inside a VRF.

**unit tests**: Unit tests will be added (mostly to the frr package) to ensure
we are producing valid configurations.

## Alternatives

The alternative is to embed all the logic inside MetalLB, but this means that
we might end up with per-cni logic and feels a bit less modular.

## Development Phases

There are strictly no phases. We want to:

- validate that FRR is able to implement what we think we require
- extend dev-env to allow traffic directed to a LB to flow from / to a VRF
- implement the feature
- have running CI that validates it
- document the feature and how to setup host networking
