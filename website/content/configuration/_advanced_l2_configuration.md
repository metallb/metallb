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
spec:
  ipAddressPools:
  - second-pool
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: NodeC
```

In this way, all the IPs coming from `first-pool` will be reacheable only via `NodeA`
and `NodeB`, and only one of those node will be choosen to expose the IP.

On the other hand, IPs coming from `second-pool` will be exposed always via `NodeC`.
