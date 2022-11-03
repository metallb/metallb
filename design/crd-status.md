# Exposing the status with CRDs

## Summary

The purpose of this enhancement is to expose useful information to perform troubleshooting
via CRDs.

## Motivation

Exposing part of the internal state was not possible until the introduction of CRD
based configuration, and despite part of the status is exposed via Prometheus
metrics, troubleshooting MetalLB often requires inspecting the logs of the various
controllers, and it's not always easy to understand why a service is not working.

For example, inspecting logs is required when:

* an invalid configuration was applied
* the session with a BGP peer is not established
* a service was not announced to a BGP peer

### Goals

* Expose useful information to ease the debug

A high level list of informations we want to retrieve is:

* configuration status (valid/invalid and reason)
* number of used / available IPs per IPAddressPool
* session state for both BGP and BFD, per node - peer
* service announcement status, per peer - node (BGP)
* service announcement node (L2)

### Non-Goals

* Providing BGP status for the legacy implementation
* Providing a Status subsection for each resource

## Proposal

### User Stories

#### Story 1

As a cluster administrator, I want to see from which nodes my service is
announced, and to which `BGPpeer`s.

#### Story 2

As a cluster administrator, I want to know if the BGP / BFD session with a given
peer is established or not, for each node.

#### Story 3

As a cluster administrator, I want to know if the configuration applied
is valid or not, and why it failed.

## Design Details

### Making the solution scalable

The biggest challenge is the fact that all the concepts related to MetalLB
are cluster scoped, but a lot of the information we care about is node scoped.

A clear example is the state of the BGP session established with a given
`BGPpeer`, where the `BGPpeer` is defined as a cluster concept, but sessions
are established from different nodes.

If we would add a `Status` field to the `BGPPeer` resource, it would add an
unwanted load to the APIServer on clusters with an high number of nodes,
especially when dealing with faulty networks where the connectivity is intermittent.

This consideration is driving the proposed design.

#### IPAddressPool Status

This should be easy enough, as it will require extending the current `IPAddressPool`
CRD with a `Status` section:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 192.168.10.0/24
  - 192.168.9.1-192.168.9.5
  - fc00:f853:0ccd:e799::/124
status:
  availableIPV4: 45
  availableIPV6: 145
  assignedIPV4: 5
  assignedIPV6: 52
```

```go
    type IPAddressPoolStatus struct {
        assignedIPV4  int
        assignedIPV6  int
        availableIPV4 int
        availableIPV6 int
}
```

#### Configuration Status

Given that the configuration is composed by multiple CRs, there are few cases
where a given configuration is invalid because of a single CR (i.e. invalid IP formatting).

The majority of the scenarios involve multiple CRs which are not compatible together.
For this reason, we think a global `ConfigurationStatus` indicator is better and
easier to understand, compared to a "per resource" status that tells if the resource
is valid or not.

```yaml
apiVersion: metallb.io/v1beta1
kind: ConfigurationStatus
metadata:
  name: config-status
  namespace: metallb-system
status:
    validConfig: false
    error: "peer 1.2.3.4 has myAsn different from 1.2.3.5, in FRR mode all myAsn must be equal"
```

```go
    type MetalLBConfigurationStatus struct {
        validConfig bool
        lastError   string
    }

    type ConfigurationStatus struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
        Status            MetalLBConfigurationStatus `json:"status,omitempty"`
    }
```

**Note:** given the fact that the configuration is parsed both by the speakers and
by the controller, we might want to expose the status of each component. However,
there is currently no logic in the configuration parsing that depends on the component.
For this reason, we can have an initial status that is produced only by the controller
(which runs in single instance) to validate the status.

As an alternative, we might consider having a state per component, named after the
single component that produces the status, but in general having a single place to
check seems more straightforward.

If we go with the _per component_ scenario, we might add a `loadedConfiguration`
field that exposes the latest loaded configuration by that component. This can't
be done if we let the controller to expose the single configuration because
what's loaded might depend on the order the CRs are received with.

#### BGP / BFD Session state

Because of the scalability concerns expressed above, the idea is to produce an instance
of the resource _per peer / node_, which exposes the state of the session between
the speaker running on a given node and a given peer.

The name of a given instance will be like `nodename-peer`, and each instance will
be labeled with the name of the node and the peer it refers to, to make it easier
to list the status of all the sessions related to a given `BGPPeer` and a given
node.

```go
    type MetalLBBGPStatus struct {
        bgpStatus string
        bfdStatus string
    }

    type BGPSessionStatus struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
        Status            MetalLBBGPStatus `json:"status,omitempty"`
    }
```

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPSessionStatus
metadata:
  name: worker0-peer1
  namespace: metallb-system
  labels:
    metallb.io/node: worker0
    metallb.io/peer: peer1
status:
    bgpStatus: Established
    bfdStatus: Up
```

The string exposed is taken directly from the output of FRR.

If BFD is not configured for a given `BGPPeer`, the exposed bfdStatus will be "N/A".

**A note about the implementation:** without entering to much into details,
we will need to implement some sort of polling of the FRR status. Given the
fact that this CR has no relation with the existing ones, a valid approach is
to follow what was done for the [metrics exporter](https://github.com/metallb/metallb/blob/main/frr-metrics/exporter.go)
and have a different component (or even the exporter itself) polling FRR and filling
the session status. The polling interval must be configurable and large enough to
avoid impacts both on FRR and on the API.
The speaker should continue not to have direct interactions with FRR.

#### Service Announcement - BGP

Given a service and a node, we want to expose the `BGPpeer`s the service is configured
to be advertised to.

```yaml
apiVersion: metallb.io/v1beta1
kind: ServiceBGPStatus
metadata:
  name: service1-worker0
  namespace: servicenamespace
  labels:
    metallb.io/node: worker0
    metallb.io/service: service1
status:
    bgpPeers:
        - peerA
        - peerB
```

```go
    type MetalLBServiceBGPStatus struct {
        Peers []string
    }

    type ServiceBGPStatus struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
        Status            MetalLBServiceBGPStatus `json:"status,omitempty"`
    }
```

The labels will allow to easily discover which services are advertised from
a given node, or all the peers a given service is advertised to.

**Note:** this status won't take into consideration the status of the
session with the given BGP peer, but only the will to advertise to that peer.
This, to overcome the considerations about scalability written in the preface.

#### Service Announcement - L2

The useful information related to L2 are related to the node that is exposing
the service, and via which interfaces.

```yaml
apiVersion: metallb.io/v1beta1
kind: ServiceL2Status
metadata:
  name: service1
  namespace: servicenamespace
  labels:
    metallb.io/node: worker0
    metallb.io/service: service1
status:
    node: worker0
    interfaces:
        - eth0
        - eth1
```

```go
    type MetalLBServiceL2Status struct {
        Node       string
        Interfaces []InterfaceInfo
    }

    type ServiceL2Status struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
        Status            MetalLBServiceL2Status `json:"status,omitempty"`
    }

    type InterfaceInfo struct {
        Name string `json:"name,omitempty"`
    }
```

The absence of interfaces means all interfaces are selected.

## Alternatives

We might consider exposing all the information (or part of it) as labels to be applied
to the services. However, the current proposal is easier to query and to navigate,
thanks to the various labels applied.

On top of that, there is some information (such as the state of the sessions) that
can't be added as service annotations.

## Development Phases

Each CRD can be developed and exposed regardless of the others. In order to consider
the status of a given CRD complete, the following items must be finished:

* implementation
* e2e testing
* documentation

After this proposal converges and is accepted, separate issues will be filed in order
to ease the development and allow the development to move in parallel.

### Test Plan

**e2e tests**: e2e tests will be expanded to validate that the exposed status is
consistent with the configuration. This includes (but not limited to) generating
the status change both from a MetalLB configuration change (i.e. adding a BGPPeer)
but also from external events (i.e. dropping a BGP session from outside).
We **must** ensure through tests that unnecessary updates are not generated if the
exposed status does not change.
**unit tests**: Unit tests will be added to any additional code in the MetalLB
repository.

### Open Points

This section contains the items which did not reach consensus during the discussion
on one hand, and can be added to the API in a second time on the other.
This will give the current version time to settle, and will allow us to ship a
version that we won't need to obsolete in the near future.

#### Exposing the allocated / non allocated ips of a given IPAddressPool

This will give visibility on which IPs are still available. On the other hand,
the status can grow considering the IPv6 allocations.
Knowing which IPs are available could somehow be useful, but this goes against
the philosophy of the IPAddressPool where all the IPs are supposed to be
interchangeable.

#### Exposing the number of services handled by this IPAddressPool

We are exposing the number of allocated / free IPs, but it might be interesting
to see how many services we are handling (which might not be a map of the ips,
considering dual stack services).

#### Adding the BGPAdvertisements that contributed to a ServiceBGPStatus

We are currently exporting the calculated state of a given service, which might
be troublesome to debug because the user might not know which BGPAdvertisements
are contributing to a given configuration.

#### Adding the nodes eligible for a given service in case of L2

We might add the nodes that are potential candidates for a given service.
