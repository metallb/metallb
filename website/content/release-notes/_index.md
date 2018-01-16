---
title: Release Notes
weight: 7
---

## Version 0.3.0

[Documentation for this release](https://metallb.universe.tf)

Action required if upgrading from 0.2.x:

- The `bgp-speaker` DaemonSet has been renamed to just
  `speaker`. Before applying the manifest for 0.3.0, delete the old
  daemonset with `kubectl delete -n metallb-system
  ds/bgp-speaker`. This will take down your load-balancers until you
  deploy the new DaemonSet.
- The
  [configuration file format](https://raw.githubusercontent.com/google/metallb/master/manifests/example-config.yaml) has
  changed in a few backwards-incompatible ways. You need to update
  your ConfigMap by hand:
  - Each `address-pool` must now have a `protocol` field, to select
    between ARP and BGP mode. For your existing configurations, add
    `protocol: bgp` to each address pool definition.
  - The `advertisements` field of `address-pool` has been renamed to
    `bgp-advertisements`, and is now optional. If you don't need any
    special advertisement settings, you can remove the section
    entirely, and MetalLB will use a reasonable default.
  - The `communities` section has been renamed to `bgp-communities`.

New features:

- MetalLB now supports ARP advertisement, enabled by setting
  `protocol: arp` on an address pool. ARP mode does not require any
  special network equipment, and minimal configuration. You can follow
  the [ARP mode tutorial]({{% relref "tutorial/arp.md" %}}) to get
  started. There is also a page about ARP
  mode's [behavior and tradeoffs]({{% relref "concepts/arp.md" %}}),
  and documentation
  on [configuring ARP mode]({{% relref "configuration/_index.md" %}}).
- The container images are
  now
  [multi-architecture images](https://blog.docker.com/2017/11/multi-arch-all-the-things/). MetalLB
  now supports running on all supported Kubernetes architectures:
  amd64, arm, arm64, ppc64le, and s390x.
- You can
  now
  [disable automatic address allocation]({{% relref "configuration/_index.md" %}}#controlling-automatic-address-allocation) on
  address pools, if you want to have manual control over the use of
  some addresses.
- MetalLB pods now come
  with
  [Prometheus scrape annotations](https://github.com/prometheus/prometheus/blob/master/documentation/examples/prometheus-kubernetes.yml). If
  you've configured your Prometheus-on-Kubernetes to automatically
  discover monitorable pods, MetalLB will be discovered and scraped
  automatically. For more advanced monitoring needs,
  the
  [Prometheus Operator](https://coreos.com/operators/prometheus/docs/latest/user-guides/getting-started.html) supports
  more flexible monitoring configurations in a Kubernetes-native way.
- We've documented how
  to
  [Integrate with the Romana networking system]({{% relref "configuration/romana.md" %}}),
  so that you can use MetalLB alongside Romana's BGP route publishing.
- The website got a makeover, to accommodate the growing amount of
  documentation in a discoverable way.

This release includes contributions from David Anderson, Charles
Eckman, Miek Gieben, Matt Layher, Xavier Naveira, Marcus Söderberg,
Kouhei Ueno. Thanks to all of them for making MetalLB better!

## Version 0.2.1

[Documentation for this release](https://v0-2-1--metallb.netlify.com)

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
  the [hacking documentation]({{% relref "community/_index.md" %}})
  for the required additional setup.

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

