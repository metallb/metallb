---
title: Release Notes
weight: 6
---


## Version 0.2.1

[Documentation for this release](https://metallb.universe.tf)

Notable fixes:

- MetalLB unable to start because Kubernetes cannot verify that
  "nobody" is a non-root
  user ([#85](https://github.com/google/metallb/issues/85))

## Version 0.2.0

[Documentation for this release](https://v0-2-0--metallb.netlify.com)

Major themes for this version are: improved BGP interoperability,
vastly increased test coverage, and improved documentation structure
and accessibility.

Notable features:
 
- This website! It replaces a loose set of markdown files, and
  hopefully makes MetalLB more accessible.
- The BGP speaker now speaks Multiprotocol BGP
  ([RFC 4760](https://tools.ietf.org/html/rfc4760)). While we still
  only support IPv4 service addresses, speaking Multiprotocol BGP is a
  requirement to successfully interoperate with several popular BGP
  stacks. In particular, this makes MetalLB compatible
  with [Quagga](http://www.nongnu.org/quagga/) and Ubiquiti's
  EdgeRouter and Unifi product lines.
- The development workflow with Minikube now works with Docker for
  Mac, allowing mac users to hack on MetalLB. See
  the [hacking documentation]({{% relref "hacking.md" %}}) for the
  required additional setup.

Notable fixes:

- Handle multiple BGP peers properly. Previously, bgp-speaker
  mistakenly made all its connections to the last defined peer,
  ignoring the others.
- Fix a startup race condition where MetalLB might never allocate an
  IP for some services.
- Test coverage is above 90% for almost all packages, up from ~0%
  previously.
- Fix yaml indentation in the MetalLB manifests.

## Version 0.1.0

[Documentation for this release](https://github.com/google/metallb/tree/v0.1)

This was the first tagged version of MetalLB. Its changelog is
effectively "MetalLB now exists, where previously it did not."

