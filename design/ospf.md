# MetalLB support for OSPF

## Summary

Add OSPF (OSPFv2, and later OSPFv3) as an announcement protocol for
LoadBalancer IPs, next to BGP and L2. The speaker's FRR instance (via the
frr-k8s backend) forms OSPF adjacencies on selected node interfaces and
originates the assigned service IPs as AS-external routes. MetalLB acts as a
pure ASBR: it never becomes transit for other traffic and never installs
learned routes on the nodes.

This was requested in [#256](https://github.com/metallb/metallb/issues/256),
which was closed with an invitation to submit a design proposal. This document
is that proposal, backed by a working prototype built on the frr-k8s raw
configuration escape hatch (see [Prototype validation](#prototype-validation)).

## Motivation

* **ECMP.** Per RFC 2328, OSPF installs all equal-cost routes, so an upstream
  router load-balances traffic for a LoadBalancer IP across every announcing
  node. Plain BGP selects a single best path unless the router supports (and
  is licensed/configured for) BGP multipath, so today all traffic for a
  service commonly funnels through one node.
* **Hardware support.** Many devices support OSPF out of the box while gating
  BGP behind additional licenses (several Cisco and Dell models were cited in
  the issue). Consumer/prosumer gateways such as UniFi expose OSPF as a
  first-class UI feature, making MetalLB-over-OSPF the natural fit for
  homelab and edge clusters on that gear.
* **No protocol implementation needed.** With FRR as the established backend,
  supporting OSPF is a configuration-rendering problem, not a routing-stack
  problem. The concern that led to this being parked in 2018 (implementing
  OSPF from scratch) no longer applies.

## Limitations/Requirements

* OSPF support targets the **frr-k8s backend only**. The native Go BGP
  implementation is out of scope by definition, and the legacy in-cluster FRR
  mode is deprecated.
* The router and the nodes must share an L2 segment (broadcast network type)
  or a point-to-point link on the interfaces where adjacency is formed.
* `ospfd` (and `ospf6d` for IPv6) must be enabled in the FRR daemons file
  shipped by frr-k8s. `staticd`, used for route origination, is already
  started automatically by modern FRR.

## Goals

* Announce LoadBalancer IPs as OSPF AS-external (type-5) LSAs from the set of
  nodes that should attract traffic, with configurable metric and metric type.
* Form adjacencies on operator-selected interfaces/areas, with configurable
  timers, authentication, and optional BFD (reusing the existing `BFDProfile`
  CRD).
* Keep the nodes out of the routing topology: no transit, no installation of
  learned OSPF routes into node kernels.
* Dual-stack parity via OSPFv3, possibly as a follow-up phase.

## Non Goals

* Acting as a general-purpose OSPF router: no virtual links, no area border
  router (ABR) behavior, no route leaking between areas, no summarization
  beyond what the advertisement granularity provides.
* Installing routes learned from OSPF on the nodes (no equivalent of BGP's
  receive side).
* Supporting OSPF with the native or legacy FRR backends.
* RIP (raised in the issue as an alternative; it does not provide ECMP, which
  is a primary motivation).

## User Stories

### Story 1

As a cluster administrator whose ToR switches only support OSPF (or require
an extra license for BGP), I want MetalLB to announce service IPs over OSPF
so I can use LoadBalancer services without new hardware or licenses.

### Story 2

As a cluster administrator, I want traffic for a LoadBalancer IP spread
across all announcing nodes via ECMP, instead of following a single BGP best
path.

### Story 3

As a homelab operator running a UniFi gateway, I want to enable OSPF in the
UniFi UI and have MetalLB services reachable and load-balanced, without
maintaining custom FRR configuration on the gateway.

## Proposal

### How announcement works

OSPF has no equivalent of bgpd's `network` statement for originating an
arbitrary prefix; ospfd advertises what it redistributes. For every IP that
must be announced from a node, the rendered FRR configuration:

1. installs a blackhole static route for the IP (`ip route <ip>/32 Null0`,
   handled by staticd; the blackhole is invisible to service traffic because
   kube-proxy's DNAT happens before the routing decision),
2. redistributes static routes into ospfd behind a route-map matching a
   prefix-list containing exactly the prefixes MetalLB manages, producing a
   type-5 external LSA per IP.

Every announcing node originates the same LSA with the same metric; upstream
routers then perform standard OSPF ECMP across the nodes.

To satisfy the "never on the path" constraint, the rendered configuration
always includes a deny-all install filter (`ip protocol ospf route-map ...`),
so LSAs learned from the network are used for protocol operation only and are
never installed in the node kernel routing table.

Withdrawal is the removal of the static route (and prefix-list entry), which
flushes the LSA. Node failure is detected by the router via the dead interval,
or sub-second with BFD.

## The API Changes

Two new namespaced CRDs in `metallb.io/v1beta1`, mirroring the shape of the
BGP pair (`BGPPeer`/`BGPAdvertisement`), plus a new typed section in the
frr-k8s API.

### MetalLB: OSPFInterface

OSPF is interface/area-based rather than peer-based, so the counterpart of
`BGPPeer` describes where adjacencies are formed:

```yaml
apiVersion: metallb.io/v1beta1
kind: OSPFInterface
metadata:
  name: uplink
  namespace: metallb-system
spec:
  interfaces:
    - eth0
  area: 0.0.0.0
  helloInterval: 2s      # optional, FRR defaults apply
  deadInterval: 8s       # optional
  networkType: broadcast # optional: broadcast | point-to-point
  authentication:        # optional
    keyID: 1
    passwordSecret:      # message-digest (MD5) / RFC 5709 HMAC-SHA
      name: ospf-auth
      namespace: metallb-system
  bfdProfile: fast       # optional, reuses BFDProfile
  nodeSelectors: []      # optional, defaults to all nodes
```

### MetalLB: OSPFAdvertisement

The counterpart of `BGPAdvertisement`, selecting pools and setting external
route attributes:

```yaml
apiVersion: metallb.io/v1beta1
kind: OSPFAdvertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
    - production
  metric: 20             # optional
  metricType: E2         # optional: E1 | E2 (default E2)
  nodeSelectors: []      # optional
```

Pool selection semantics (`ipAddressPools`, `ipAddressPoolSelectors`,
`nodeSelectors`) are identical to `BGPAdvertisement`. As with BGP and L2, a
pool can be advertised by multiple protocols simultaneously.

### FRR-K8S

`FRRConfigurationSpec` gains an `ospf` section next to `bgp`. Sketch:

```yaml
apiVersion: frrk8s.metallb.io/v1beta1
kind: FRRConfiguration
metadata:
  name: worker-0
  namespace: metallb-system
spec:
  ospf:
    routers:
      - routerID: 172.18.0.2   # optional
        interfaces:
          - name: eth0
            area: 0.0.0.0
            helloInterval: 2s
            deadInterval: 8s
            bfdProfile: fast
        prefixes:              # originated as external LSAs
          - prefix: 192.168.10.1/32
            metric: 20
            metricType: E2
```

frr-k8s renders this into the static routes, prefix-list, route-maps,
`redistribute static`, interface stanzas and the mandatory no-install filter
described above, and enables `ospfd`/`ospf6d` in its daemons file. Merging
multiple `FRRConfiguration` resources follows the existing frr-k8s rules;
interface/area parameters must be identical across merged configurations
(conflict → configuration rejected), while originated prefixes are unioned.

### Rejected alternatives

#### Implementing OSPF natively in Go

Rejected for the same reasons the native BGP path is frozen: a full link-state
implementation is a large, security-sensitive surface, and FRR is already the
strategic backend. This was the blocker when #256 was originally triaged.

#### Originating routes via a dummy interface and `redistribute connected`

Assigning LB IPs to a node interface and redistributing connected routes works
but requires managing kernel interfaces/addresses from the speaker, conflicts
with L2 mode's address handling, and leaks the IPs into other subsystems that
enumerate local addresses. Blackhole statics are contained entirely inside FRR
and were validated in the prototype.

#### Exposing OSPF only through frr-k8s `spec.raw`

The raw escape hatch is explicitly unsupported and unvalidated; users would
hand-maintain FRR syntax, and MetalLB could not reconcile advertisements with
service state (every service IP change would require editing the raw blob).
Raw remains useful for experimentation only.

## Design Details

### Speaker changes

The speaker already dispatches announcements through a
`map[config.Proto]Protocol`. The work is:

* a new `config.OSPF` protocol, produced by the config conversion layer from
  the new CRDs, with validation (webhook + reconcile-time) gated on the
  frr-k8s backend, mirroring how native-mode limitations are enforced today;
* an `ospfController` implementing the speaker `Protocol` interface. Node
  selection (which nodes announce a given service, honoring
  `externalTrafficPolicy`, node selectors and service health) reuses the
  logic of the BGP controller;
* an `internal/ospf/frrk8s` renderer that folds the OSPF state into the same
  per-node `FRRConfiguration` object the BGP path already writes, so a node
  running both protocols produces a single merged resource.

### Rendered FRR configuration

The per-node configuration validated by the prototype:

```
ip prefix-list METALLB-OSPF seq 5 permit 192.168.10.1/32
!
route-map OSPF-REDIST permit 10
 match ip address prefix-list METALLB-OSPF
!
route-map OSPF-NO-INSTALL deny 10
!
ip protocol ospf route-map OSPF-NO-INSTALL
!
ip route 192.168.10.1/32 Null0
!
interface eth0
 ip ospf area 0.0.0.0
 ip ospf hello-interval 2
 ip ospf dead-interval 8
!
router ospf
 redistribute static route-map OSPF-REDIST
```

### OSPFv3 / dual stack

OSPFv3 is a separate daemon (`ospf6d`) with distinct configuration syntax and
an incompatible authentication model (RFC 7166 / IPsec instead of MD5). The
API above is address-family neutral (interfaces + areas + prefixes), so v3 is
an additive rendering path. Proposal: ship OSPFv2 first, follow with OSPFv3
in a second phase before declaring the feature GA, keeping dual-stack parity
a goal rather than a v1 requirement. Notably, UniFi exposes only OSPFv2.

### BFD

ospfd supports `ip ospf bfd`. The existing `BFDProfile` CRD is reused as-is;
without BFD, failover is bounded by the dead interval (verified at exactly
the dead interval in the prototype).

### Graceful shutdown

On speaker drain/shutdown, ospfd can advertise `max-metric router-lsa` to
divert traffic before withdrawal, giving hitless rollouts. This should be
wired into the frr-k8s reload path (open point).

### Metrics and status

* Adjacency state per interface/neighbor and count of originated prefixes,
  exposed by the frr-k8s metrics sidecar alongside the BGP metrics.
* A `ServiceOSPFStatus` resource analogous to `ServiceBGPStatus` can follow
  the pattern from the CRD status design; proposed as a follow-up, not part
  of the initial implementation.

### Prototype validation

A Phase-0 prototype ran the full mechanism with zero code changes, using
frr-k8s `spec.raw` on a kind dev-env cluster (3 nodes) with an external FRR
10.5.3 container acting as the upstream router:

* all three nodes reached Full adjacency with the router;
* the LB IP appeared on the router as a type-5 external route with three
  equal-cost nexthops, installed in the router kernel as a 3-way ECMP route;
* traffic to the service IP from beyond the router was served by multiple
  backends;
* the no-install filter kept node kernel tables free of OSPF routes;
* killing a node's FRR pod removed its nexthop after the dead interval (8s
  with 2s/8s timers) with traffic continuing on the remaining paths, and the
  nexthop returned automatically on recovery;
* `staticd` is auto-started by the FRR image, so origination via blackhole
  statics requires no daemon changes beyond enabling `ospfd`.

## Test Plan

* Unit tests for config conversion/validation and for the FRR configuration
  rendering (frr-k8s side), following the golden-file pattern used by the
  BGP renderers.
* E2E: a new `ospftests` suite reusing the external-container topology of the
  BGP suite, with the container running ospfd (the prototype setup is the
  template). Scenarios:
  * adjacency establishment on the selected interface/area, with and without
    authentication and BFD;
  * external LSA origination/withdrawal following service lifecycle and pool
    membership;
  * ECMP: the router holds one nexthop per eligible node; `nodeSelectors`
    and `externalTrafficPolicy: Local` shrink the set accordingly;
  * failover on node/pod loss within dead-interval (and BFD) bounds;
  * no OSPF routes installed on nodes;
  * coexistence: the same pool advertised via BGP and OSPF from the same
    nodes.
* Manual validation against a UniFi gateway with OSPF enabled in the UI,
  documented as a website how-to.

## Development Phases

1. This design document, iterated with maintainers.
2. frr-k8s: typed OSPF API, rendering, daemons enablement, metrics, e2e.
3. MetalLB: CRDs, conversion, validation, webhooks, generated manifests/helm.
4. MetalLB speaker: `ospfController` + frr-k8s renderer, unit tests.
5. E2E suite, dev-env support (`--protocol ospf`), website docs including the
   UniFi guide.
6. Follow-ups: OSPFv3, `ServiceOSPFStatus`, graceful shutdown via max-metric.

## Open Points

* Naming: `OSPFInterface` vs `OSPFArea` as the adjacency-configuration CRD.
* Authentication scope for v1: plain MD5 (`message-digest`) only, or also
  RFC 5709 HMAC-SHA variants.
* Whether area types beyond a regular area (stub/NSSA membership) are needed
  at all, given the ASBR-only role (type-5 LSAs cannot be flooded into stub
  areas; NSSA would require type-7 support).
* VRF support: the BGP path supports VRFs; the OSPF API sketch reserves a
  per-router structure to allow it, but v1 targets the default VRF.
* Exact merging/conflict rules for OSPF sections across multiple
  `FRRConfiguration` resources in frr-k8s.
* Whether enabling `ospfd`/`ospf6d` unconditionally in the frr-k8s daemons
  file is acceptable (memory footprint per pod) or should be driven by the
  presence of OSPF configuration.
