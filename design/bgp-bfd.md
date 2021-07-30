# BFD enablement in BGP

## Summary

The purpose of this enhancement is to use enable [BFD](https://en.wikipedia.org/wiki/Bidirectional_Forwarding_Detection) to be used in conjunction with BGP.

BFD will be available only in the upcoming [FRR based implementation](0001-frr.md).

## Motivation

Prior to adding BFD support, you can configure the `hold-time` parameter to speed up failure detection, but the minimum or fastest value is 3 seconds.

BFD provides a quicker path failure detection than BGP, and using them together will allow users to provide a better
service.

BFD for BGP is supported by FRR [out of the box](http://docs.frrouting.org/en/latest/bfd.html#bgp-bfd-configuration).

### Goals

* Provide a way to establish BFD on BGP session.
* Provide a way to configure the parameters of the BFD session associated with a BGP session.

### Non-Goals

* Implement BFD on the legacy BGP implementation.
* Configure BFD independent of a BGP session

## Proposal

### User Stories

#### Story 1

As a cluster administrator, I want to declare a BGP session to be backed up by BFD.

#### Story 2

As a cluster administrator, I want to be able to set the BFD parameters related to a BGP session.

## Design Details

### BFD Design proposal

The idea is to leverage FRR to enable BFD on a BGP session.

When declaring a BFD peer, all it takes to enable BFD is to add a `bfd` property on the neighbour:

```yaml
    neighbor <A.B.C.D|X:X::X:X|WORD> bfd profile BFDPROF
```

The proposal here is to define a bfd profile section in the config structure that looks like:

```yaml
    bfd-profiles:
    - name: bfdprofile1
      receive-interval: 150
      transmit-interval: 150
      detect-multiplier: 10
      echo-receive-interval: 20
      echo-transmit-interval: 20
      echo-mode: true
      passive-mode: true
      minimum-ttl: 5
```

When a property of the profile is not set, MetalLB will honor [FRR default values](https://docs.frrouting.org/en/latest/bfd.html#peer-profile-configuration).

When setting a BGP peer, an optional bfd-profile property will enable BFD:

```yaml
    peers:
    - peer-address: 10.0.0.1
      peer-asn: 64501
      my-asn: 64500
      bfd-profile: bfdprofile1
```

A configuration of BFD sessions while running in legacy mode will result in a rejection of the configuration
file with an error.

#### Impacts on the operator

Keeping the operator as the guinea pig for the CRD implementation, a new `BFDProfile` CRD will be introduced, with the form of:

```yaml
apiVersion: metallb.io/v1alpha1
kind: BFDProfile
metadata:
  name: profile
  namespace: metallb-system
spec:
  receive-interval: 150
  transmit-interval: 150
  detect-multiplier: 10
  echo-receive-interval: 20
  echo-transmit-interval: 20
  echo-mode: true
  passive-mode: true
  minimum-ttl: 5
```

Similarly, the BGPPeer CRD that is getting introduced will be configured with a new optional `bfdProfile` field.

Ideally, if/when the CRDs are moved back to MetalLB, it will be possible to enrich the BGPPeer CRD status with information containing the status of the BFD session.

#### Metrics

Metrics describing the status (and the health) of the bfd session between two peers will be produced.
FRR provides indication of the status of a given session in the form of

```bash
frr# show bfd peers
BFD Peers:
        peer 192.168.0.1
                ID: 1
                Remote ID: 1
                Status: up
                Uptime: 1 minute(s), 51 second(s)
                Diagnostics: ok
                Remote diagnostics: ok
                Peer Type: dynamic
                Local timers:
                        Detect-multiplier: 3
                        Receive interval: 300ms
                        Transmission interval: 300ms
                        Echo receive interval: 50ms
                        Echo transmission interval: disabled
                Remote timers:
                        Detect-multiplier: 3
                        Receive interval: 300ms
                        Transmission interval: 300ms
                        Echo receive interval: 50ms
```

but also provide indicators on the health of a given session:

```bash
frr# show bfd peer 192.168.0.1 counters
     peer 192.168.0.1
             Control packet input: 126 packets
             Control packet output: 247 packets
             Echo packet input: 2409 packets
             Echo packet output: 2410 packets
             Session up events: 1
             Session down events: 0
             Zebra notifications: 4
```

### Test Plan

**e2e tests**: E2E tests will be expanded to cover bfd, using external container(s)
running FRR. The test will need to cover the cases where a node is dropped, verifying
that the broken route is detected by BFD.
**unit tests**: Unit tests will be added to any additional code in the MetalLB
repository.

## Alternatives

The alternative is not to implement the feature and rely on separate instance of FRR in order
to cover BFD. However, the integration is straightforward and would be a nice addition on top of
BGP.

## Development Phases

The only constraint for this enhancement is the dependency on the FRR integration.

1 - FRR integration is complete, and available for use.
2 - The BFD feature is added together with e2e tests.
3 - The BFD feature is added to the Documentation and to the Operator as specific CRDs.
