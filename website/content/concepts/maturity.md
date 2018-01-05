---
title: Project Maturity
weight: 4
---

MetalLB is currently in *alpha*.

## Adoption

MetalLB is being used in at least 1 non-critical production cluster,
and is stable there. However, it has not been "battle tested" by many
people yet.

If you use it today, you'll be an early adopter and may encounter more
issues than usual. Please file bugs! We want to hear from you and
address pain points.

## Test coverage

The codebase has some test coverage, but it it's still sorely lacking
in important places. Empirical testing shows that the common codepaths
work properly, but edge cases may have bugs.

Increasing test coverage is an active area of work.

## Configuration format

The configuration format was grown organically during the prototyping
phase, and may change in backwards-incompatible ways as the project is
refactored to support routing protocols other than BGP.

Release notes during alpha will highlight any required changes to
configuration when upgrading between versions.

## Documentation

Documentation exists, but hasn't been battle-tested by many readers
unfamiliar with the project. As such, it may be incomplete or
confusing in parts.

If you find shortcomings in the documentation, please file bugs! Your
perspective is very valuable, and we want to hear about what works and
what doesn't.
