---
title: Project Maturity
weight: 4
---

MetalLB is currently in *beta*.

## Adoption

MetalLB is being used in several production and non-production clusters, by
several people and companies. It is bundled together with the major
Kubernetes distributions for on premise deployments.
Based on the infrequency of bug reports, MetalLB appears to be robust in those deployments.

In any case, no one is perfect and issues might happen.
Please file bugs! We want to hear from you and address pain points.

## Test coverage

The codebase has reasonable test coverage, and a good amount of
end-to-end tests which covers against most part of regressions.
Despite that, edge cases may have bugs, so if you find unexpected
behaviours, please consider [filing an issue](https://github.com/metallb/metallb/issues).

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

## Kubernetes compatibility

The MetalLB maintainers aim to keep compatibility with the supported versions
of Kubernetes. If a kubernetes version is not supported anymore, the compatibility is
provided on best effort basis.

## Documentation

Documentation exists, but hasn't been battle-tested by many readers
unfamiliar with the project. As such, it may be incomplete or
confusing in parts.

If you find shortcomings in the documentation, please file bugs! Your
perspective is very valuable, and we want to hear about what works and
what doesn't.

## Developers

The MetalLB copyright was owned by Google, until March 2019. However, it
was never an official Google project. The project doesn't have any
form of corporate sponsorship.

MetalLB was created by [one person](https://www.dave.tf), working on MetalLB in
their spare time as motivation allows.  The original author has now been
empowering a [team of maintainers]({{% relref "community/_index.md"
%}}#contributing) to assist with moving the project forward.

Most support and new feature development is at the mercy of the availability of
this small group, so you should set your expectations accordingly.

If you would like to help improve this balance, [contributions]({{% relref
"community/_index.md" %}}#contributing) are very welcome! In addition to code
contributions, donation of resources (hardware, cloud environments...) are also
very welcome: the more different conditions we can test MetalLB in, the fewer
bugs and regressions will be introduced!
