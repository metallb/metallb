# Preferred Node Selection for L2 Mode

## Summary

A new `preferredNodeSelectors` field on `L2Advertisement` lets administrators express soft node
preferences for L2 leader election. The speakers try preferred nodes first when electing the
announcer for a LoadBalancer IP, falling back to any eligible node when preferred nodes are
unavailable. The API mirrors the Kubernetes `PreferredSchedulingTerm` pattern and builds on the existing
hash-based election algorithm.

Related: [#2797](https://github.com/metallb/metallb/issues/2797) (feature request for this design doc),
[PR #1804](https://github.com/metallb/metallb/pull/1804) (rejected predecessor, annotation-based, single node).

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
2. Roll all speaker pods to the new version.
3. Only then create or update L2Advertisements with `preferredNodeSelectors`.

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

All nodes are eligible. Prefer edge nodes; any node can announce as fallback:

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
`nodeSelectors` before preference scores apply.

```
all cluster nodes
  --> intersected with active speakers             = reachable nodes
    --> filtered by nodeSelectors (hard)            = eligible nodes
      --> filtered by ETP:Local / healthy endpoints = available nodes
      --> scored by preferredNodeSelectors (soft)     = ordered available nodes
        --> tie-broken by sha256 hash                  = final election order
```

### Score Calculation

Weights are additive. A node matching multiple `PreferredNodeSelector` entries accumulates
their weights. For example, if a node matches both a weight-60 and a weight-50 selector, its
total score is 110.

Nodes that match no preference selector receive a score of 0.

### Sort Algorithm

```
ad = the single preference-bearing L2Advertisement for this pool (if any)
scores = map[nodeName]int32
if ad != nil:
    for each preferredNodeSelector in ad:
        for each eligible node matching preferredNodeSelector.Preference:
            scores[node] += preferredNodeSelector.Weight

sort availableNodes by:
    1. scores[node] descending       (higher weight wins)
    2. sha256(node + "#" + ip) ascending  (deterministic tie-break)
```

Within the same score tier, the hash breaks ties, so all speakers pick the same winner.

### Multi-L2Advertisement Restriction

A pool can have multiple `L2Advertisement` objects. MetalLB already combines fields across
advertisements: node eligibility is OR'd (`poolMatchesNodeL2`), interfaces are UNION'd
(`ipAdvertisementFor`).

Preference scores are not aggregated across advertisements. At most one `L2Advertisement`
with non-empty `preferredNodeSelectors` may target a given pool. This is enforced by the
validating webhook, described in [Validation Rules](#validation-rules).

This keeps all preference math in a single policy object per pool. If weights were summed across multiple L2Advertisements, a new overlapping
advertisement could silently change the effective priority of nodes established by another
team's policy, and the resulting totals would not be visible in any single object.

This restriction only applies to `preferredNodeSelectors`. Multiple L2Advertisements can
still target the same pool to combine `nodeSelectors` or `interfaces` as they do today.
Note that preferences apply to the combined eligible set from all
advertisements:

```
Ad1:  nodeSelectors: [{role: lb}]              → nodes A, B eligible
Ad2:  nodeSelectors: [{role: edge}]            → nodes C, D eligible
Ad3:  nodeSelectors: [{role: gpu}]
      preferredNodeSelectors: [{weight: 100, preference: {zone: primary}}]
                                               → node E eligible

Combined eligible set: A, B, C, D, E  (OR across all three ads)
If node C matches zone=primary → C gets score 100 (even though Ad1/Ad2 made C eligible, not Ad3)
```

Note: if Ad3 omitted `nodeSelectors` entirely, it would make all cluster nodes eligible
(not just A-D), since an L2Advertisement with no `nodeSelectors` matches every node.
This would silently expand the eligible set beyond what Ad1 and Ad2 intended.
Administrators adding a preference-only L2Advertisement should be aware of this and scope
its `nodeSelectors` to match the intended eligible set.

See also
[Interaction with serviceSelectors](#interaction-with-serviceselectors) for additional
considerations on why cross-advertisement preference aggregation was avoided.

### Validation Rules

The L2Advertisement validating webhook passes all existing L2Advertisements and IPAddressPools
through `config.For()`, so it supports both single-object and cross-object checks.

New validation rules:

- Weight must be in the range 1-100. Enforced by kubebuilder markers on the CRD schema, no
  webhook code needed.
- Each `preference` label selector must be valid. Enforced by `metav1.LabelSelectorAsSelector()`
  during config parsing.
- An L2Advertisement cannot set both `serviceSelectors` and `preferredNodeSelectors`. Rejected
  during config parsing.
- At most one L2Advertisement with non-empty `preferredNodeSelectors` may target a given pool.
  Rejected during config parsing via cross-object comparison of all L2Advertisements and their
  target pools.

### Config-Time Resolution

The speaker resolves preference scores at config-parse time. `nodeSelectors` are already
resolved this way via `selectedNodes()` to `Nodes map[string]bool`. A new
`PreferredNodes map[string]int32` field on the internal `L2Advertisement` config struct stores
the resolved scores.

```go
type L2Advertisement struct {
	Nodes          map[string]bool
	Interfaces     []string
	AllInterfaces  bool
	PreferredNodes map[string]int32  // new: node name -> aggregated weight
}
```

When a node's labels change, the speaker re-parses the config (existing behavior) and
recomputes `PreferredNodes`.

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
matches all nodes, adding the same weight to each. All scores increase equally, so the election order does not change.

### Flapping Considerations

Volatile labels cause re-elections with `nodeSelectors`. `preferredNodeSelectors` increases the risk:

- A recovered node wins back ~1/N of IPs under hash-based election. A recovered preferred node reclaims all IPs it outscores others for, making unstable preferred nodes more disruptive.

Mitigations:

- Use stable labels (zone-level, role). Avoid labels toggled by automation.
- Avoid single-hostname preferences.

### Interaction with serviceSelectors

The validating webhook rejects an L2Advertisement that sets both `serviceSelectors` and
`preferredNodeSelectors`.

All speakers must agree on the winner for a given IP, and election is computed pool-wide
(`poolMatchesNodeL2` evaluates all advertisements targeting the pool). A service-scoped
advertisement carrying preference scores would influence election for every service in the
pool, not just the ones matching the selector.

For per-service preference, isolate the service onto its own `IPAddressPool` using
`serviceAllocation.serviceSelectors`, then attach a dedicated `L2Advertisement` with
`preferredNodeSelectors` to that pool.

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
are preferred; non-matching nodes are fallback.

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

## Test Plan

### Unit Tests

Scenarios to cover:

- Preference scoring: single selector, multiple selectors, cumulative weights, equal-weight
  tie-breaking via hash
- Failover: preferred node removed, preferred node returns, all preferred nodes unavailable
- Interaction with existing features: `nodeSelectors` filtering, `externalTrafficPolicy: Local`
- Backward compatibility: nil/empty `preferredNodeSelectors` produces identical behavior to today
- Config parsing: correct `PreferredNodes` map population, invalid label selectors
- Validation: `serviceSelectors` + `preferredNodeSelectors` rejected, two preference-bearing
  ads targeting the same pool rejected

### E2E Tests

Scenarios to cover:

- Preferred node announces the service
- Preferred node failover and reclaim
- Runtime label and preference configuration changes
- Combined with `nodeSelectors`
- No nodes match preference (fallback to hash)
- No preferred selectors configured (regression)

## Development Phases

- Add the new type and field to the CRD, run code generation
- Implement preference resolution in config parsing
- Modify the election sort in the speaker
- Add unit and e2e tests
- Document the feature
