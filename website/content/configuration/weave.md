---
title: Issues with Weave
weight: 20
---

Weave Net doesn't support `externalTrafficPolicy: Local` in its
default configuration. If you switch a service to use the local
traffic policy, Weave will blackhole the traffic.

If you want to use the local traffic policy, you need to use Weave
version 2.4.0 or later, and enable the `NO_MASQ_LOCAL` flag, as
described in [Weave's
documentation](https://www.weave.works/docs/net/latest/kubernetes/kube-addon/#configuration-options).

In version [2.7.0](https://github.com/weaveworks/weave/releases/tag/v2.7.0) and later, the `NO_MASQ_LOCAL` flag is enabled by default.
