---
title: Project Maturity
weight: 4
---

MetalLB is currently in *alpha*.

## Adoption

MetalLB is being used in several production and non-production
clusters, by several people and companies. So far, it appears to be
very stable in those deployments, but in the grand scheme of things,
MetalLB is still not hugely "battle tested."

If you use it today, you'll be an early adopter and may encounter more
issues than usual. Please file bugs! We want to hear from you and
address pain points.

## Test coverage

The codebase has reasonable test coverage, but lacks comprehensive
end-to-end tests. Empirical testing, combined with the unit tests we
have, show that the common codepaths work properly, but edge cases may
have bugs.

Increasing test coverage is an active area of work.

## Configuration format

The configuration format was grown organically during the prototyping
phase, and may change in backwards-incompatible ways as the project is
refactored to support additional routing protocols and features.

Release notes during alpha will highlight any required changes to
configuration when upgrading between versions. Stabilization of the
configuration format is the main blocker for graduating MetalLB to
"beta" status.

## Documentation

Documentation exists, but hasn't been battle-tested by many readers
unfamiliar with the project. As such, it may be incomplete or
confusing in parts.

If you find shortcomings in the documentation, please file bugs! Your
perspective is very valuable, and we want to hear about what works and
what doesn't.

## Developers

While MetalLB's copyright is owned by Google, this is _not_ an
official Google project. The project have any other form of corporate
sponsorship, other than GCP credits generously provided by Google to
run test infrastructure.

The majority of code changes, as well as the overall direction of the
project, is a personal endeavor of [one person](https://www.dave.tf),
working on MetalLB in their spare time.

This means that, currently, support and new feature development is
mostly at the mercy of one person's availability and resources. You
should set your expectations appropriately.

If you would like to help improve this
balance,
[contributions]({{% relref "community/_index.md" %}}#contributing) are
very welcome! In addition to code contributions, donation of resources
(hardware, cloud environments...) are also very welcome: the more
different conditions we can test MetalLB in, the fewer bugs and
regressions will be introduced!
