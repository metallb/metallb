# Preferred Node Selection for L2 Mode

## Summary

A new `preferredNodeSelectors` field on `L2Advertisement` lets administrators express soft node
preferences for L2 leader election. The speakers try preferred nodes first when electing the
announcer for a LoadBalancer IP, falling back to any eligible node when preferred nodes are
unavailable. The API mirrors the Kubernetes `PreferredSchedulingTerm` pattern and builds on the existing
hash-based election algorithm.

Related: [#2797](https://github.com/metallb/metallb/issues/2797) (feature request for this design doc),
[PR #1804](https://github.com/metallb/metallb/pull/1804) (rejected predecessor, annotation-based, single node).

Depends on: [PR #3014](https://github.com/metallb/metallb/pull/3014), which refactors L2
election so `serviceSelectors` drives the candidate node set. The scoring step added here
slots into the post-#3014 `ShouldAnnounce` flow.

## Motivation

MetalLB L2 mode elects the announcing node for each LoadBalancer IP using
`sha256(nodeName + "#" + ipString)`. The hash gives deterministic, even distribution, but sometimes you need control over which node announces without losing failover.

### Scenario 1: Dedicated Load-Balancer Nodes

A cluster has three worker nodes and two "edge" nodes.
L2 traffic should land on the edge nodes when possible, but the workers are needed as failover targets when both edge nodes are down. Today, using `nodeSelectors` to
restrict to edge nodes means zero failover if both go down.

### Scenario 2: Zone-Aware Traffic Locality

A bare-metal cluster spans two server rooms (zone-a, zone-b). Clients connect through a switch in zone-a. The L2 announcer should be in zone-a to minimize cross-zone hops, but zone-b should take over during zone-a maintenance windows.

### Scenario 3: Stable Announcements During Rolling Upgrades

Cluster upgrades often take nodes out of rotation, for example by draining and rebooting them in a rolling fashion. With announcements spread across many nodes, IP assignments get shuffled each time an announcing node goes down, and a full upgrade cycle can shuffle them repeatedly. Pinning announcements to a small set of "anchor" nodes (upgraded first or last) keeps IPs stable for the majority of the upgrade window. The remaining nodes still serve as failover targets when the anchor nodes themselves are patched.

### Goals

- Allow administrators to express weighted soft preferences for which nodes announce L2 services.
- Preserve full failover: all eligible nodes remain candidates when preferred nodes are unavailable.
- Maintain backward compatibility: existing configurations without `preferredNodeSelectors` behave identically to today.
- Follow Kubernetes API patterns (`PreferredSchedulingTerm`).

### Non-Goals

- Influencing BGP mode node selection.
- Per-service preference overrides via annotations. This has previously been suggested, and the
  preference is to not have annotations used (see [PR #1804](https://github.com/metallb/metallb/pull/1804)).

## Proposed API

One new field on `L2AdvertisementSpec` and one new type.

### Struct delta on L2AdvertisementSpec

```go
type L2AdvertisementSpec struct {
	// ... existing fields unchanged ...

	// PreferredNodeSelectors allows specifying soft node preferences for L2
	// leader election. Nodes matching these selectors receive a higher score
	// and are preferred as the announcing node. If no preferred node is
	// available, any eligible node (per NodeSelectors) can announce.
	// Modeled after Kubernetes PreferredSchedulingTerm.
	// +optional
	PreferredNodeSelectors []PreferredNodeSelector `json:"preferredNodeSelectors,omitempty"`
}
```

### New type

```go
// PreferredNodeSelector expresses a weighted soft preference for nodes.
// This follows the Kubernetes PreferredSchedulingTerm pattern where Weight
// controls relative priority and Preference selects matching nodes.
type PreferredNodeSelector struct {
	// Weight associated with matching the corresponding preference,
	// in the range 1-100.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// A node selector that applies to this weight.
	Preference metav1.LabelSelector `json:"preference"`
}
```

The naming and structure follow Kubernetes'
[`PreferredSchedulingTerm`](https://pkg.go.dev/k8s.io/api/core/v1#PreferredSchedulingTerm):

- `preference` matches `PreferredSchedulingTerm.Preference` to signal this is a soft selector.
- `metav1.LabelSelector` is consistent with existing MetalLB selector fields
  (`NodeSelectors`, `IPAddressPoolSelectors`, etc.). We use this instead of `NodeSelectorTerm`, consistent with the rest of the MetalLB API.
- `int32` weight, range 1-100, required (no `omitempty`), matching
  `PreferredSchedulingTerm.Weight` type and validation.

### Versioning and Upgrade Path

The field is optional and added to `v1beta1`. New controllers reading old CRDs see nil, identical
to today's behavior.

Safe upgrade order:

1. Update CRDs (adds the new field to the schema).
2. Roll the controller and webhook to a version that understands and preserves
   `preferredNodeSelectors`. Without this, older components can drop the field on write.
3. Roll all speaker pods to the new version.
4. Only then create or update L2Advertisements with `preferredNodeSelectors`.

### Example CRs

Only nodes with `role: lb` are eligible. Among those, prefer edge nodes:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: prefer-edge
  namespace: metallb-system
spec:
  ipAddressPools:
    - production-pool
  nodeSelectors:
    - matchLabels:
        role: lb
  preferredNodeSelectors:
    - weight: 100
      preference:
        matchLabels:
          node-role: edge
```

All nodes are eligible. Prefer edge nodes. Any node can announce as fallback:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: prefer-edge-all-eligible
  namespace: metallb-system
spec:
  ipAddressPools:
    - production-pool
  preferredNodeSelectors:
    - weight: 100
      preference:
        matchLabels:
          node-role: edge
```

Minimal configuration, backward compatible (no preferences):

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
    - default-pool
```

## Semantics

### Relationship Between nodeSelectors and preferredNodeSelectors

`nodeSelectors` is the hard eligibility filter and determines which nodes can announce.
`preferredNodeSelectors` orders nodes within that eligible set. A node must pass
`nodeSelectors` (via some ad in the pool) before preference scores apply.

```
all cluster nodes
  --> per ad at config-parse time:
      selectedNodes(ad.nodeSelectors)                            = ad.Nodes (per ad)
    --> for each service, filter by serviceSelectors only:
        l2AdsForService(pool.L2Advertisements, svc)              = adsForService
      --> speakers from memberlist UsableSpeakers
        --> speakersForAds via adsMatchNodeL2(adsForService, s):
            node covered by at least one service-matching ad     = candidate nodes
          --> filtered by ETP:Local / healthy endpoints          = available nodes
            --> sum ad.PreferredNodes across adsForService       = scored available nodes
              --> sort: score DESC, sha256(node+"#"+ipString) ASC  = election order
                --> availableNodes[0] == myNode ? announce
```

The local participation guard is the same `adsForService`: if no ad in that set covers
`myNode` (`adsMatchNodeL2(adsForService, myNode)` is false), the speaker returns
`noMatchingAdvertisement` and sits out. Every speaker reaches this guard with the same
`adsForService` (no node argument on the filter), so all speakers that pass the guard see the
same candidate set and the same score map.

### Score Calculation

Scoring is ad-scoped. Within a single `L2Advertisement`, preferences apply only to nodes
matching that ad's own `nodeSelectors`. An ad with no `nodeSelectors` scores every node. An ad
never raises the score of a node outside its own eligible set.

Weights within one ad add up. A node matching both a weight-60 and a weight-50 selector in the
same ad scores 110 from that ad.

For a given service, the final score sums per-ad scores across every L2Advertisement that
targets the service's pool and matches the service via `serviceSelectors`. Ads that don't match
the service (by pool or by `serviceSelectors`) contribute nothing. Nodes matching no preference
in any applicable ad score 0.

### Sort Algorithm

The scoring step slots in between `availableNodes` and the existing `sort.Slice`
call in `ShouldAnnounce`, reusing the `adsForService` slice already computed
earlier in the same function:

```
adsForService = l2AdsForService(pool.L2Advertisements, svc)   // already computed in ShouldAnnounce
scores = map[nodeName]int64
for each ad in adsForService:
    for node, weight in ad.PreferredNodes:
        scores[node] += weight

sort availableNodes by:
    1. scores[node] descending                       (higher weight wins)
    2. sha256(node + "#" + ipString) ascending       (deterministic tie-break)
```

### Multiple L2Advertisements Per Pool

A pool can have multiple `L2Advertisement` objects, and MetalLB already aggregates fields
across them. `adsMatchNodeL2` ORs node eligibility across the service-matching ads at
election time, and `ipAdvertisementFor` unions advertised interfaces per service at
announcement time. `preferredNodeSelectors` aggregates per ad. Each ad scores only its own
eligible nodes, and a service's final score is the sum across the ads matching that service.

Example - preferences bound to the ad's own eligible set:

```
Ad1:  nodeSelectors: [{role: lb}]              → nodes A, B eligible for Ad1
Ad2:  nodeSelectors: [{role: edge}]            → nodes C, D eligible for Ad2
Ad3:  nodeSelectors: [{role: gpu}]             → node E eligible for Ad3
      preferredNodeSelectors: [{weight: 100, preference: {zone: primary}}]

Combined eligible set: A, B, C, D, E  (OR across all three ads)
If node C matches zone=primary → C still scores 0. Ad3's preference only applies to Ad3's
own eligible nodes (just E), so C is not lifted by a preference from an ad that does not
target it.
```

Example - two ads both targeting the pool with overlapping eligible sets, both contributing
preferences to the shared nodes:

```
Ad1:  nodeSelectors: [{role: lb}]
      preferredNodeSelectors: [{weight: 70, preference: {zone: primary}}]
Ad2:  nodeSelectors: [{role: lb}]
      preferredNodeSelectors: [{weight: 30, preference: {gpu: "true"}}]

Node X (role=lb, zone=primary, gpu=true)  → 70 (Ad1) + 30 (Ad2) = 100
Node Y (role=lb, zone=primary)            → 70 (Ad1) + 0         = 70
Node Z (role=lb)                          → 0       + 0          = 0
```

### Validation Rules

The L2Advertisement validating webhook passes all existing L2Advertisements and IPAddressPools
through `config.For()`, so it supports both single-object and cross-object checks.

New validation rules:

- Weight must be in the range 1-100. Enforced by kubebuilder markers on the CRD schema, no
  webhook code needed.
- Each `preference` label selector must be valid. Enforced by `metav1.LabelSelectorAsSelector()`
  during config parsing.

No cross-object validation is needed. You can combine `preferredNodeSelectors` with
`serviceSelectors` and with other L2Advertisements targeting the same pool. See
[Sort Algorithm](#sort-algorithm) and
[Multiple L2Advertisements Per Pool](#multiple-l2advertisements-per-pool).

### Config-Time Resolution

A new `PreferredNodes map[string]int64` field on the internal `L2Advertisement` config struct
holds per-ad scores. `l2AdvertisementFromCR` fills it alongside the existing `selectedNodes`
call.

```go
type L2Advertisement struct {
	Nodes          map[string]bool
	Interfaces     []string
	AllInterfaces  bool
	PreferredNodes map[string]int64  // new: node name -> aggregated weight, ad-scoped
}
```

`PreferredNodes` keys are a subset of `Nodes`. A missing key means the node scored 0 under
this ad, not that it is ineligible. `Nodes` remains the eligibility map.

### Backward Compatibility

When `preferredNodeSelectors` is nil or empty, `PreferredNodes` is nil, all scores are 0, and
the sort falls through to pure hash-based ordering. Behavior is identical to today's release.

## Details

### Scenario: Dedicated Edge Nodes with Failover

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: external-pool
  namespace: metallb-system
spec:
  addresses:
    - 192.168.10.0/24
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: prefer-edge
  namespace: metallb-system
spec:
  ipAddressPools:
    - external-pool
  preferredNodeSelectors:
    - weight: 100
      preference:
        matchLabels:
          node-role: edge
```

Nodes labeled `node-role: edge` score 100 and are tried first. All other nodes score 0 and
serve as failover. If both edge nodes go down, the hash picks the best worker.
When an edge node recovers, the speakers re-elect it on the next reconciliation cycle.

### Scenario: Multi-Tier Zone Preference

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: tiered-zones
  namespace: metallb-system
spec:
  ipAddressPools:
    - production-pool
  preferredNodeSelectors:
    - weight: 100
      preference:
        matchLabels:
          zone: primary
    - weight: 50
      preference:
        matchLabels:
          zone: secondary
```

A node in `zone: primary` scores 100. A node in `zone: secondary` scores 50. A node in neither
zone scores 0. Within each tier, the hash provides deterministic ordering.

### Scenario: Cumulative Weights

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: cumulative-example
  namespace: metallb-system
spec:
  ipAddressPools:
    - production-pool
  preferredNodeSelectors:
    - weight: 60
      preference:
        matchLabels:
          zone: primary
    - weight: 50
      preference:
        matchLabels:
          gpu: "true"
```

A node with both `zone: primary` and `gpu: "true"` scores 110. A node with only
`zone: primary` scores 60. A node with only `gpu: "true"` scores 50. The node scoring 110
is preferred over a node scoring 100 from a single high-weight selector.

## Additional Considerations

### Empty Preference Selector

A `PreferredNodeSelector` with an empty `preference` (no `matchLabels` or `matchExpressions`)
matches every node in the ad's own `ad.Nodes` set. If the ad has no `nodeSelectors`, every
cluster node gets the same weight bump and election order is unchanged relative to pure hash
order. If the ad has `nodeSelectors`, only nodes inside that eligible set get the bump, which
does lift them above nodes eligible only via other ads in the pool.

### Flapping Considerations

Volatile labels cause re-elections with `nodeSelectors`. `preferredNodeSelectors` increases the risk:

- A recovered node wins back ~1/N of IPs under hash-based election. A recovered preferred node reclaims all IPs it outscores others for, making unstable preferred nodes more disruptive.

Mitigations:

- Use stable labels (zone-level, role). Avoid labels toggled by automation.
- Avoid single-hostname preferences.

### Interaction with serviceSelectors

You can combine `preferredNodeSelectors` with `serviceSelectors`. Election runs per-service and
considers only ads that both target the service's pool and match the service via
`serviceSelectors` (see [#3014](https://github.com/metallb/metallb/pull/3014)). Scoring runs on
that filtered set, so a service-scoped ad's preferences only influence election for services it
matches.

Per-service preference maps to a single CR:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: zone-preference-for-frontend
  namespace: metallb-system
spec:
  ipAddressPools:
    - shared-pool
  serviceSelectors:
    - matchLabels:
        app: frontend
  preferredNodeSelectors:
    - weight: 100
      preference:
        matchLabels:
          zone: primary
```

Services not matching `app: frontend` ignore this advertisement.

Caveat: `ShouldAnnounce` runs per-service, so two services sharing an IP via
`allow-shared-ip` that match different preference-bearing ads compute different
score maps and can elect different nodes. The hash tie-break only fires on score
ties, so divergent scores bypass it. Shared-IP siblings should match the same
set of preference-bearing ads — the simplest way is to keep preferences on ads
without `serviceSelectors`.

## Drawbacks

Operators need to understand weighted scoring. Misconfigured preferences can cause unexpected
IP migration or reduce failover capacity.

## Alternatives Considered

### Option A: Flat PreferredNodeSelectors (No Weights)

```yaml
preferredNodeSelectors:
  - matchLabels:
      zone: primary
```

A simpler version where preferred nodes are just a label selector with no weight. Matching nodes
are preferred. Non-matching nodes are fallback.

Cannot express multi-tier prioritization (primary zone > secondary zone > everything else).

### Option B: Ordered NodeSelectorPriority

```yaml
nodeSelectorPriority:
  - priority: 10
    nodeSelector:
      matchLabels:
        zone: primary
  - priority: 20
    nodeSelector:
      matchLabels:
        zone: secondary
```

An explicit priority number (lower wins) similar to `IPAddressPool.serviceAllocation.priority`.

Consistent with pool priority, but invents new naming rather than following the Kubernetes `PreferredSchedulingTerm` pattern.

### Option C: Single "Prioritized" Selector Replacing nodeSelectors

```yaml
prioritizedNodeSelectors:
  - weight: 100
    preference:
      matchLabels:
        node-role: edge
  - weight: 0  # sentinel: eligible only as fallback
    preference:
      matchLabels:
        role: lb
```

A single field replaces `nodeSelectors` and folds eligibility and preference into one list.
Any positive weight marks a preferred node. Zero marks a fallback-only eligible node.
Operators reason about one field instead of two.

This shape diverges from the Kubernetes `required` / `preferred` scheduling split that MetalLB
already follows via `nodeSelectors`, overloads one field with two distinct semantics, and
forces an API break or a compatibility shim on `nodeSelectors`. A sibling
`preferredNodeSelectors` field keeps the current shape and avoids a migration.

## Test Plan

### Unit Tests

Scenarios to cover:

- Preference scoring: single selector, multiple selectors within one ad, cumulative weights,
  equal-weight tie-breaking via hash
- Ad-scoped scoring: an ad's preferences only apply to nodes matching its own `nodeSelectors`.
  A preference-only ad (no `nodeSelectors`) scores every node
- Multi-ad aggregation: per-service scores sum across all ads matching the service
- `serviceSelectors` + `preferredNodeSelectors`: preference applies only to services matching
  the ad
- Failover: preferred node removed, preferred node returns, all preferred nodes unavailable
- Interaction with existing features: `nodeSelectors` filtering, `externalTrafficPolicy: Local`
- Backward compatibility: nil/empty `preferredNodeSelectors` produces identical behavior to
  today
- Config parsing: correct `PreferredNodes` map population, invalid label selectors

### E2E Tests

Scenarios to cover:

- Preferred node announces the service
- Preferred node failover and reclaim
- Runtime label and preference configuration changes
- Combined with `nodeSelectors` (required + preferred)
- Multiple L2Advertisements targeting the same pool, each contributing preferences
- `serviceSelectors` + `preferredNodeSelectors` scoped to a subset of services
- No nodes match preference (fallback to hash)
- No preferred selectors configured (regression)

## Development Phases

- Add the new type and field to the CRD, run code generation
- Implement preference resolution in config parsing
- Modify the election sort in the speaker
- Add unit and e2e tests
- Document the feature
