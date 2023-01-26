# Configuring Preferred Node To Be Layer2 Leader

## Summary

The purpose of the ehancement is to support configuring preferred node to be the leader for L2 mode.

With this enhancement, the leader election will be artificially controllable.

## Motivation

MetalLB’s layer2 mode is simliar to `Keepalived`(https://metallb.universe.tf/concepts/layer2/#comparison-to-keepalived) which using `VRRP`.

This enhancement implement functionality similar to [Priority and Preemption](https://www.cisco.com/assets/sol/sb/Switches_Emulators_v2_3_5_xx/help/350_550/index.html#page/tesla_350_550_olh/ts_vrrp_18_09.html) configuration of `VRRP`.

### Goals

- In L2 mode, being able to specify a node as the preferred leader for a service.
- When a node is configured as the preferred leader, it replaces the current leader.

### Non-Goals

- Change the current leader mechanism election using memberlist.
- Change the data models of Metallb's CRDs.

## Proposal

### User Stories

#### Story 1

When using Metallb's Layer2 mode, user wants to specify one node as the leader for a service and let all traffic for this service's IP goes to this sepcified node.

#### Story 2

When using Metallb's Layer2 mode, the current leader node requires maintenance, user want to manually switchover to a sepcified node.

## Design Details

### Preferred Node Design proposal

The proposal here is to:

1. Add preferred node configuration for a service. In order not to change exsiting data model of the CRDs, 
   the configuration can be placed in an annotation of the service. This can also make the configuration 
   service-oriented and more flexible.

2. Enhance Metallb's layer2 leader election process, choose the preferred node as leader.

#### Preferred Node configuration

If user prefer one node to serve the Load Balancer IP, add the `metallb.universe.tf/preferredNode` annotation to service,
with the name of the node as the annotation value. For example:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/preferredNode: NodeA
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

#### Leader election modification

When doing layer2 annoncement, if the service being annonced is configured with preferred node
and the node is available, just make the preferred one as the leader.

#### Note: The preferred node handling should be put before the default electing hashing algorithm. 

This is to implement the `Preemption`(service change will trigger leader re-election, so the preferred one will be choosen immediately) 

### Test Plan

#### Unit tests

The additional code must be covered by unit tests.

#### E2e tests

This is a new feature, the coverage of the e2e tests must be extended

##### Test strategy

In order to ensure this feature is working, we must verify that:
- Create a service with preferred node annotation
- Add preferred node to a existing service's annotation 
- Change preferred node in the service's annotation
- Cover both node available and unavailable cases

## Alternatives

- Support priority configuration for multiple nodes rather than single node.
- Put the configuration in L2Advertisement CRD rather than a service's annotation.

## Development Phases

There are strictly no phases.
