# MetalLB Operator

## Summary

MetalLB currently includes [instructions and resources for deploying
MetalLB](https://metallb.universe.tf/installation/) using a few methods:
directly with manifests, customized manifests with kustomize, or via helm.
Another approach for managing the deployment of software on Kubernetes is via
an [operator](https://operatorhub.io/what-is-an-operator).

This document proposes creating an operator for MetalLB in a new git repository
under the metallb github organization and publishing it on
[OperatorHub.io](https://operatorhub.io/).

## Motivation

For any Kubernetes environment that primarily uses operators to manage add-ons
to the cluster, it would be beneficial to have the option of using an operator
to manage MetalLB, as well.

In [another proposal](https://github.com/metallb/metallb/pull/833), we have
discussed creating a CRD based configuration interface for MetalLB. It seems
that settling on the ideal data model may require some more time and
experimentation. We can make use of this separate operator repo as a place to
try out a CRD interface for MetalLB which writes out a ConfigMap and doesn't
have to disrupt the core MetalLB project.

### Goals

- Create an operator in a new git repository that makes it easy to deploy
  MetalLB on clusters where operators are the preferred method for managing
  cluster add-ons.
- Publish the operator on operatorhub.io to make it discoverable.
- Use the separate operator repository as a place to experiment with a CRD
  interface for configuring MetalLB.

### Non-Goals

- No changes are proposed for the core MetalLB project.
- This is not intended to replace the end goal of an eventual full CRD based
  interface in the core MetalLB project, but instead provide a place to
  iterate on an interface without any disruption to the project in the short
  term.

## Proposal

* Create github.com/metallb/metallb-operator.
* Grant russellb and any other current MetalLB maintainer commit rights to this
  repository. Further metallb-operator specific committers may be added in the
  future based on contributions to the operator and at the approval of the
  current MetalLB maintainers.
* Once the operator is functional, publish to operatorhub.io.
* Document the use of the operator on the MetalLB web site.
* Releases of metallb-operator would be independent from MetalLB.

## Alternatives

### Develop the Operator Elsewhere

If this proposal were to be rejected, the operator would live outside of the
metallb github organization. Otherwise, I expect most of the rest to remain
the same.
