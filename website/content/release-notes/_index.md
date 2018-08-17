---
title: Release Notes
weight: 7
---

## Version 0.7.3

[Documentation for this release](https://metallb.universe.tf)

Bugfixes:

- Fix BGP announcement refcounting when using shared
  IPs. ([#295](https://github.com/google/metallb/issues/295))

## Version 0.7.2

[Documentation for this release](https://v0-7-2--metallb.netlify.com)

Bugfixes:

- Fix gratuitous ARP and NDP announcements on IP
  failover. ([#291](https://github.com/google/metallb/issues/291))
- Fix BGP dialing on Arm64, by using `x/sys/unix` instead of the
  `syscall` package. ([#289](https://github.com/google/metallb/issues/289))

## Version 0.7.1

[Documentation for this release](https://v0-7-1--metallb.netlify.com)

Bugfixes:

- Actually allow layer2 mode to use the Local traffic
  policy. Oops. ([#279](https://github.com/google/metallb/issues/279))

## Version 0.7.0

[Documentation for this release](https://v0-7-0--metallb.netlify.com)

Action required if updating from 0.6.x:

- MetalLB no longer does leader election. After upgrading to 0.7, you
  can delete a number of k8s resources associated with that. This is
  just a cleanup, nothing bad happens if you leave the resources
  orphaned in your cluster. Depending on your installation method,
  some of these may have already been cleaned up for you.
  - `kubectl delete -nmetallb-system endpoints metallb-speaker`
  - `kubectl delete -nmetallb-system rolebinding leader-election`
  - `kubectl delete -nmetallb-system role leader-election`

New features:

- Layer2 mode now supports `externalTrafficPolicy=Local`, meaning layer2
  services can see the true client source
  IP. ([#257](https://github.com/google/metallb/issues/257))
- Layer2 mode now selects leader nodes on a per-service level, instead of using
  a single leader node for all services in the cluster. If you have many
  services, this change spreads the load of handling incoming traffic across
  more than one machine. ([#195](https://github.com/google/metallb/issues/195))
- MetalLB's maturity has upgraded from _alpha_ to _beta_! Mostly this
  just reflects the increased confidence in the code from the larger
  userbase, and adds some guarantees around graceful upgrades from one
  version to the next.

Bugfixes:

- Speaker no longer sends localpref over eBGP sessions
  ([#266](https://github.com/google/metallb/issues/266))

This release includes contributions from Baul, David Anderson, Ryan
Roemmich, Sanjeev Rampal, and Steve Sloka. Thanks to all of them for
making MetalLB better!

## Version 0.6.2

[Documentation for this release](https://v0-6-2--metallb.netlify.com)

Bugfixes:

- Fix nil pointer deref crash on BGP peers that reject MetalLB's OPEN message too promptly ([#250](https://github.com/google/metallb/issues/250))

## Version 0.6.1

[Documentation for this release](https://v0-6-1--metallb.netlify.com)

Bugfixes:

- Speaker no longer goes into a tight CPU-burning loop when pods are
  deleted on the
  node. ([#246](https://github.com/google/metallb/issues/246))

## Version 0.6.0

[Documentation for this release](https://v0-6-0--metallb.netlify.com)

Action required if upgrading from 0.5.x:

- As documented in the 0.5.0 release notes, several deprecated fields
  have been removed from the configuration. If you didn't update your
  configurations for 0.5, you may need to make the following changes:
  - Rename the `cidr` field of address pools to `addresses`
  - Rename `protocol: arp` and `protocol: ndp` to `protocol: layer2`
  - Replace `arp-network` statements with a range-based IP allocation

New features:

- You can now colocate multiple services on a single IP address, using
  annotations on the Service objects. See
  the
  [IP sharing documentation]({{% relref "usage/_index.md" %}}#ip-address-sharing) for
  instructions and caveats. ([#121](https://github.com/google/metallb/issues/121))
- Layer 2 mode now listens on all interfaces for ARP and NDP requests,
  not just the interface used for communication by Kubernetes
  components. ([#165](https://github.com/google/metallb/issues/165))
- MetalLB now uses structured logging instead of Google's glog
  package. Logging events are written to standard output as a series
  of JSON objects suitable for collection by centralized logging
  systems. ([#189](https://github.com/google/metallb/issues/189))
- BGP connections can now specify a password for TCP MD5 secured BGP
  sessions. ([#215](https://github.com/google/metallb/issues/215))
- MetalLB is now available as a Helm package in the "stable" Helm
  repository. Note that, due to code review delay, it may take several
  days after a release before the Helm package is
  updated. ([#177](https://github.com/google/metallb/issues/177))

Bugfixes:

- Correctly use AS_SEQUENCE in eBGP session messages, rather than
  AS_SET ([#225](https://github.com/google/metallb/issues/225))

This release includes contributions from David Anderson, ghorofamike,
Serguei Bezverkhi, and Zsombor Welker. Thanks to all of them for making
MetalLB better!

## Version 0.5.0

[Documentation for this release](https://v0-5-0--metallb.netlify.com)

Action required if upgrading from 0.4.x:

- The `cidr` field of address pools in the configuration file has been
  renamed to `addresses`. MetalLB 0.5 understands both `cidr` and
  `addresses`, but in 0.6 it will only understand `addresses`, so
  please update now.
- The `arp` and `ndp` protocols have been replaced by a unified
  `layer2` protocol. MetalLB 0.5 understands both the old and new
  names, but 0.6 will only understand `layer2`, so please update now.
- Remove any `arp-network` entries from your configuration. If your
  address pool overlaps with the ethernet network or broadcast
  addresses for your LAN, use IP range notation (see new features) to
  exclude them from your address pool.
- The router IDs used on BGP sessions may change in this version, in
  clusters where nodes have multiple IP addresses. If your BGP
  infrastructure monitors or enforces specific router IDs for peers,
  you may need to update those systems to match new router IDs.
- The Prometheus metrics for ARP and NDP traffic have been
  merged. Instead of `arp_*` and `ndp_*` metrics, there is now single
  set of `layer2_*` metrics, in which the `ip` label can be IPv4 or
  IPv6.

New features:

- ARP and NDP modes have been replaced by a single "layer 2" mode,
  indicated by `protocol: layer2` in the configuration file. Layer 2
  mode uses ARP and NDP under the hood, but having a single protocol
  name makes it easier to build protocol-agnostic configuration
  templates.
- You can give addresses to MetalLB using a simple IP range notation,
  in addition to CIDR prefixes. For example,
  `192.168.0.0-192.168.0.255` is equivalent to `192.168.0.0/24`. This
  makes it much easier to allocate IP ranges that don't fall cleanly
  on CIDR prefix boundaries.
- BGP mode supports nodes with multiple interfaces and IP addresses
  ([#182](https://github.com/google/metallb/issues/182)). Previously,
  MetalLB could only establish working BGP sessions on the node's
  "primary" interface, i.e. the one that owned the IP that Kubernetes
  uses to identify the node. Now, peerings may be established via any
  interface on the nodes, and traffic will flow in the expected
  manner.

Bugfixes:

- The NDP
  handler
  [refcounts its sollicited multicast group memberships](https://github.com/google/metallb/issues/184),
  to avoid extremely rare cases where it might stop responding for a
  service IP.

## Version 0.4.6

[Documentation for this release](https://v0-4-6--metallb.netlify.com)

Bugfixes:

- [Remove the --config-ns flag](https://github.com/google/metallb/issues/193)

## Version 0.4.5

[Documentation for this release](https://v0-4-5--metallb.netlify.com)

Bugfixes:

- [Controller doesn't clean up balancers that change their type away from LoadBalancer](https://github.com/google/metallb/issues/190)

## Version 0.4.4

[Documentation for this release](https://v0-4-4--metallb.netlify.tf)

This was a broken attempt to fix the same bugs as 0.4.5. You should
not use this version.

## Version 0.4.3

[Documentation for this release](https://v0-4-3--metallb.netlify.com)

Changes:

- Make the configmap's namespace and name configurable via flags, for
  Helm upstreaming.

## Version 0.4.2

[Documentation for this release](https://v0-4-2--metallb.netlify.com)

Bugfixes:

- [Speaker doesn't readvertise existing services on sessions added by node label changes](https://github.com/google/metallb/issues/181).

## Version 0.4.1

[Documentation for this release](https://v0-4-1--metallb.netlify.com)

Bugfixes:

- [Make speaker not crash on machines with IPv6 disabled](https://github.com/google/metallb/issues/180).

## Version 0.4.0

[Documentation for this release](https://v0-4-0--metallb.netlify.com)

Action required if upgrading from 0.3.x:

- MetalLB's use of Kubernetes labels has changed slightly to conform
  to Kubernetes best practices. If you were using a label match on
  `app: controller` or `app: speaker` Kubernetes labels to find
  MetalLB objects, you should now match on a combination of `app:
  metallb`, `component: controller` or `component: speaker`, depending
  on what objects you want to select.
- RBAC rules have changed, and now allow the MetalLB speaker to list
  and watch Node objects. If you are not installing MetalLB via the
  provided manifest, you will need to make this change by hand.
- If you want to switch to using Helm to manage your MetalLB
  installation, you must first uninstall the manifest-based version,
  with `kubectl delete -f metallb.yaml`.

New features:

- Initial IPv6 support! The `ndp` protocol allows v6 Kubernetes
  clusters to advertise their services using
  the
  [Neighbor Discovery Protocol](https://en.wikipedia.org/wiki/Neighbor_Discovery_Protocol),
  IPv6's analog to ARP. If you have an IPv6 Kubernetes cluster, please
  try it out
  and [file bugs](https://github.com/google/metallb/issues/new)!
- BGP peers now have
  a
  [node selector]({{% relref "configuration/_index.md" %}}#limiting-peers-to-certain-nodes). You
  can use this to integrate MetalLB into more complex cluster network
  topologies.
- MetalLB now has
  a
  [Helm chart](https://github.com/google/metallb/tree/master/helm/metallb). If
  you use [Helm](https://helm.sh) on your cluster, this should make it
  easier to track and manage your MetalLB installation. The chart will
  be submitted for inclusion in the main Helm stable repository
  shortly after the release is finalized. Use of Helm is optional,
  installing the manifest directly is still fully supported.

Other improvements:

- MetalLB
  now
  [backs off on failing BGP connections](https://github.com/google/metallb/issues/84),
  to avoid flooding logs with failures
- ARP mode should be a little
  more
  [interoperable with clients](https://github.com/google/metallb/issues/172),
  and failover should be a little faster, thanks to tweaks to its
  advertisement logic.
- ARP and NDP modes export [Prometheus](https://prometheus.io) metrics
  for requests received, responses sent, and failover-related
  transmissions. This brings them up to "monitoring parity" with BGP
  mode.
- Binary internals were refactored to share more common code. This
  should reduce the amount of visual noise in the logs.

This release includes contributions from Oga Ajima, David Anderson,
Matt Layher, John Marcou, Paweł Prażak, and Hugo Slabbert. Thanks to
all of them for making MetalLB better!

## Version 0.3.1

[Documentation for this release](https://v0-3-1--metallb.netlify.com)

Fixes a couple
of [embarrassing bugs](https://github.com/google/metallb/issues/142)
that sneaked into 0.3.

Bugfixes:

- Revert to using `apps/v1beta2` instead of `apps/v1` for MetalLB's
  Deployment and Daemonset, to remain compatible with Kubernetes 1.8.
- Create the `metallb-system` namespace when installing
  `test-bgp-router`.
- Disable BIRD in `test-bgp-router`. Bird got updated to 2.0, and the
  integration with `test-bgp-router` needs some reworking.

## Version 0.3.0

[Documentation for this release](https://v0-3-0--metallb.netlify.com)

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
  the [ARP mode tutorial]({{% relref "tutorial/layer2.md" %}}) to get
  started. There is also a page about ARP
  mode's [behavior and tradeoffs]({{% relref "concepts/layer2.md" %}}),
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

