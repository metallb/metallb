---
title: Advanced L2 configuration
weight: 1
---

### Limiting the set of nodes where the service can be announced from

In L2 mode, only one node is elected to announce the IP from.

Normally, all the nodes where a `Speaker` is running are eligible for any given IP.

There can be scenarios where only a subset of the nodes are exposed to a given network, so
it can be useful to limit only those nodes as potential entry points for the service IP.

This is achieved by using the node selector in the `L2Advertisement` CR.

{{<mermaid align="center">}}
graph TD
    metallBA-->|announces|subnetA(Subnet A)
    metallBB-->|announces|subnetA(Subnet A)
    metallBC-->|announces|subnetB(Subnet B)

    subgraph NodeA
        metallBA("MetalLB<br>Speaker")
    end
    subgraph NodeB
        metallBB("MetalLB<br>Speaker")
    end
    subgraph NodeC
        metallBC("MetalLB<br>Speaker")
    end

{{< /mermaid >}}

In this example, NodeA and NodeB are exposed to the subnet A, whereas node C is exposed to subnet B.

In order to limit the set of nodes for a given advertisement, the node selector must be set:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: NodeA
  - matchLabels:
      kubernetes.io/hostname: NodeB
```

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - second-pool
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: NodeC
```

In this way, all the IPs coming from `first-pool` will be reachable only via `NodeA`
and `NodeB`, and only one of those node will be chosen to expose the IP.

On the other hand, IPs coming from `second-pool` will be exposed always via `NodeC`.

### Specify network interfaces that LB IP can be announced from

In L2 mode, by default a metallb speaker announces the LoadBalancer IP from all the network interfaces of a node. We can use `interfaces` in `L2Advertisement` to select a subset of them.

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - third-pool
  interfaces:
  - eth0
  - eth1
```

This `L2Advertisement` will make MetalLB announce the Services associated to IPs from `third-pool` only from the interfaces `eth0` and `eth1` of all nodes.

The `interfaces` selector can also be used together with `nodeSelectors`. In this example, the IPs from `fourth-pool` will be announced only from `eth3` of `NodeA`:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - fourth-pool
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: NodeA
  interfaces:
  - eth3
```

{{% notice note %}}
The IP from a given `IPAddressPool` is advertised using the union of all the `L2Advertisements` referencing it.
This means that the interfaces used to advertise an IP will be the union of the interfaces selected by all the L2Advertisements.

For example:

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example-advertisement9
  namespace: metallb-system
spec:
  interfaces:
  - eno1
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: example-advertisement10
  namespace: metallb-system
spec:
  ipAddressPools:
  - pool1
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: hostB
  interfaces:
  - ens18
```

The above YAML indicates that MetalLB should advertise the VIPs of all IPAddressPools including pool1 from the interface eno1 of all nodes, and also advertise the VIPs in pool1 from ens18 of hostB.

In other words, if MetalLB chooses hostB to announce the VIP of pool1, the Speaker should announce the VIP from the interfaces ens18 and eno1; if it chooses other nodes, the Speaker should announce the VIP only from the interface eno1.
{{% /notice %}}

{{% notice warning %}}
The interface selector won't affect how MetalLB is choosing the leader for a given L2 IP. This means that if it elects a leader where the selected interface is not available, the service won't be announced. The cluster administrator is responsible to use the combination of interfaces selector and node selector to avoid the problem.
{{% /notice %}}
