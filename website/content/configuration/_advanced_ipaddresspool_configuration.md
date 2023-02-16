---
title: Advanced AddressPool configuration
weight: 1
---

### Controlling automatic address allocation

In some environments, you'll have some large address pools of "cheap"
private IPs (e.g. RFC1918), and some smaller pools of "expensive" IPs
(e.g. leased public IPv4 addresses).

By default, MetalLB will allocate IPs from any configured address pool
with free addresses. This might end up using "expensive" addresses for
services that don't require it.

To prevent this behaviour you can disable automatic allocation for a pool
by setting the `autoAssign` flag to `false`:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: cheap
  namespace: metallb-system
spec:
  addresses:
  - 192.168.10.0/24
```

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: expensive
  namespace: metallb-system
spec:
  addresses:
  - 42.176.25.64/30
  autoAssign: false
```

Addresses can still be specifically allocated from the "expensive"
pool with the methods described in
the [usage](/usage/#requesting-specific-ips) section.

{{% notice note %}}
To specify a single IP address in a pool, use `/32` in the CIDR notation
(e.g. `42.176.25.64/32`).
{{% /notice %}}

### Reduce scope of address allocation to specific Namespace and Service

This option can be used to reduce the scope of particular IPAddressPool
to set of namespaces and services, by adding an optional namespace
and/or service selectors.
This is useful for mutitenant context in which there is a need for pinning
IPAddressPool to specific namespace/service.

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: ippool-ns-service-alloc-sample
  namespace: metallb-system
spec:
  addresses:
    - 192.168.20.0/24
  avoidBuggyIPs: true
  serviceAllocation:
    priority: 50
    namespaces:
      - namespace-a
      - namespace-b
    namespaceSelectors:
      - matchLabels:
          foo: bar
    serviceSelectors:
      - matchExpressions:
          - {key: app, operator: In, values: [bar]}
```

The above IPAddressPool example is pinned to Service(s) which has a label
matching with expression `key: app, operator: In, values: [bar]` created
either in `namespace-a` or `namespace-b` or any namespace has a label
`foo:bar`.

Given a service, if multiple matching IPAddressPool are available MetalLB
will check for the availability of IPs sorting the matching IPAddressPool
by priority, starting from the highest to the lowest. A lower number for
priority field equals a higher priority. If multiple IPAddressPool have
the same priority, the choice will be random.
When not specifying a priority / setting priority 0 is considered as lowest
priority and will be used for assignment only if the pools with priority
can't be used.

{{% notice note %}}
When a service explicitly chooses an IPAddressPool via `metallb.universe.tf/address-pool`
annotation or an IP address via `spec.loadBalancerIP` or `metallb.universe.tf/loadBalancerIPs`
annotation which doesn't match the service will stay in pending.
{{% /notice %}}

### Handling buggy networks

Some old consumer network equipment mistakenly blocks IP addresses
ending in `.0` and `.255`, because of
misguided
[smurf protection](https://en.wikipedia.org/wiki/Smurf_attack).

If you encounter this issue with your users or networks, you can
set the `AvoidBuggyIPs` flag of the IPAddressPool CR.
By doing so, the `.0` and the `.255` addresses will be avoided.
