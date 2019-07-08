---
title: Project Maturity
weight: 4
---

MetalLB is currently in *beta*.

## Adoption

MetalLB is being used in several production and non-production clusters, by
several people and companies. Based on the infrequency of bug reports, MetalLB
appears to be robust in those deployments.

If you use it today, you are still an early adopter, compared to larger projects
in the Kubernetes ecosystem. As such, you may encounter more issues than
usual. Please file bugs! We want to hear from you and address pain points.

## Test coverage

The codebase has reasonable test coverage, but lacks comprehensive
end-to-end tests. Empirical testing, combined with the unit tests we
have, show that the common codepaths work properly, but edge cases may
have bugs.

Increasing test coverage is an active area of work.

## Configuration format

The configuration format may change in backwards-incompatible ways as the
project is refactored to support additional routing protocols and features.

Backward-incompatible changes to configuration will be rolled out in a "make
before break" fashion: first there will be a release that understands the new
_and_ the old configuration format, so that you can upgrade your configuration
separately from the code. The next release after that removes support for the
old configuration format.

Release notes during beta will highlight any required changes to configuration
when upgrading between versions, and give advance warning of removals planned
for the following version.

## Documentation

Documentation exists, but hasn't been battle-tested by many readers
unfamiliar with the project. As such, it may be incomplete or
confusing in parts.

If you find shortcomings in the documentation, please file bugs! Your
perspective is very valuable, and we want to hear about what works and
what doesn't.

## Developers

MetalLB's copyright was owned by Google, until March 2019. However, it
was never an official Google project. The project doesn't have any
form of corporate sponsorship.

The majority of code changes, as well as the overall direction of the project,
is a personal endeavor of [one person](https://www.dave.tf), working on MetalLB
in their spare time as motivation allows.

This means that, currently, support and new feature development is mostly at the
mercy of one person's availability and resources. You should set your
expectations appropriately.

If you would like to help improve this balance, [contributions]({{% relref
"community/_index.md" %}}#contributing) are very welcome! In addition to code
contributions, donation of resources (hardware, cloud environments...) are also
very welcome: the more different conditions we can test MetalLB in, the fewer
bugs and regressions will be introduced!
