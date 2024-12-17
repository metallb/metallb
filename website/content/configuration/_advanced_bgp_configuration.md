---
title: Advanced BGP configuration
weight: 1
---

### Advertisement configuration

By default, BGP mode advertises each allocated IP to the configured
peers with no additional BGP attributes. The peer router(s) will
receive one `/32` route for each service IP, with the BGP localpref
set to zero and no BGP communities.

You can configure more elaborate advertisements with multiple `BGPAdvertisement`s
that lists one or more custom advertisements.

In addition to specifying localpref and communities, you can use this
to advertise aggregate routes. The `aggregation-length` advertisement
option lets you "roll up" the /32s into a larger prefix. Combined with
multiple advertisement configurations, this lets you create elaborate
advertisements that interoperate with the rest of your BGP network.

For example, let's say you have a leased `/24` of public IP space, and
you've allocated it to MetalLB. By default, MetalLB will advertise
each IP as a /32, but your transit provider rejects routes more
specific than `/24`. So, you need to somehow advertise a `/24` to your
transit provider, but still have the ability to do per-IP routing
internally.

Here's a configuration that implements this:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: first-pool
  namespace: metallb-system
spec:
  addresses:
  - 198.51.100.10/24
```

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: local
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
  aggregationLength: 32
  localPref: 100
  communities:
  - 65535:65282
```

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: external
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
  aggregationLength: 24
```

With this configuration, if we create a service with IP 198.51.100.10,
the BGP peer(s) will receive two routes:

- `198.51.100.10/32`, with localpref=100 and the `no-advertise`
  community, which tells the peer router(s) that they can use this
  route, but they shouldn't tell anyone else about it.
- `198.51.100.0/24`, with no custom attributes.

With this configuration, the peer(s) will propagate the
`198.51.100.0/24` route to your transit provider, but once traffic
shows up locally, the `198.51.100.10/32` route will be used to forward
into your cluster.

As you define more services, the router will receive one "local" `/32`
for each of them, as well as the covering `/24`. Each service you
define "generates" the `/24` route, but MetalLB deduplicates them all
down to one BGP advertisement before talking to its peers.

Additionally, we can define [community aliases](#community-aliases) in order
to have descriptive names for the communities, to be used in place of
the two 16 bits format.

### Limiting peers to certain nodes

By default, every node in the cluster connects to all the peers listed
in the configuration. In more advanced cluster topologies, you may want
each machine to peer with its top-of-rack router, but not the routers
in other racks. For example, if you have a "rack and spine" network
topology, you likely want each machine to peer with its top-of-rack
router, but not the routers in other racks.

{{<mermaid align="center">}}
graph BT
    subgraph " "
      metallbA("MetalLB<br>Speaker")
    end
    subgraph "  "
      metallbB("MetalLB<br>Speaker")
    end

    subgraph "   "
      metallbC("MetalLB<br>Speaker")
    end
    subgraph "    "
      metallbD("MetalLB<br>Speaker")
    end

    metallbA-->torA(ToR Router)
    metallbB-->torA(ToR Router)
    metallbC-->torB(ToR Router)
    metallbD-->torB(ToR Router)

    torA-->spine(Spine Router)
    torB-->spine(Spine Router)
{{< /mermaid >}}

You can limit peers to certain nodes by using the `node-selectors`
attribute of peers in the configuration. The semantics of these
selectors are the same as those used elsewhere in Kubernetes, so refer
to the [labels documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) on
the Kubernetes website.

For example, this is a (somewhat contrived) definition for a peer that
will only be used by machines:

- With hostname `hostA` or `hostB`, or
- That have the `rack=frontend` label, but _not_ the label `network-speed=slow`:

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: example
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
  peerPort: 180
  nodeSelectors:
  - matchLabels:
      rack: frontend
    matchExpressions:
    - key: network-speed
      operator: NotIn
      values: [slow]
  - matchExpressions:
    - key: kubernetes.io/hostname
      operator: In
      values: [hostA, hostB]
```

### Announcing the Service from a subset of nodes

It is possible to limit the set of nodes that are advertised as next hops to reach
the service IPs. This is achieved by using the node selector in the `BGPAdvertisement` CR.

{{<mermaid align="center">}}
graph TD
    metallBA-->|connects|torA(Router)
    metallBA-->|announces|torA(Router)
    metallBB-->|connects|torA(Router)
    metallBB-->|announces|torA(Router)
    metallBC-->|connects|torA(Router)

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

In this example, the service is announced only from NodeA and NodeB.
Note that this feature is orthogonal to the [BGP peer node selector](#limiting-peers-to-certain-nodes),
as it's performed at service (`IPAddressPool`) level.

MetalLB will still follow the `BGPPeer` configuration
to choose if a certain node must be peered with a certain router.

In order to limit the set of nodes for a given advertisement, the node selector must be set:

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
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

In this way, all the IPs coming from `first-pool` will be reachable only via `NodeA`
and `NodeB`.

### Announcing the Service to a subset of peers

By default, every service IP is advertised to all the connected peers. It is possible
to limit the set of peers a service IP is advertised to. This is achieved by using
the peers in the `BGPAdvertisement` CR.

{{<mermaid align="center">}}
graph TD
    PoolA("PoolA<br>198.51.100.10/24")
    style PoolA fill:orange

    metalLB("MetalLB Speaker")-->|announces|PeerA
    metalLB("MetalLB Speaker")-->|announces|PeerB
    metalLB("MetalLB Speaker")-->|announces|PeerB
    metalLB("MetalLB Speaker")-->|announces|PeerC

    linkStyle 0 stroke-width:2px,fill:none,stroke:orange;
    linkStyle 1 stroke-width:2px,fill:none,stroke:orange;
    linkStyle 2 stroke-width:2px,fill:none,stroke:lightgreen;
    linkStyle 3 stroke-width:2px,fill:none,stroke:lightgreen;

    PoolB("PoolB<br>198.51.200.10/24")
    style PoolB fill:lightgreen

{{< /mermaid >}}

In this example, a service IP from PoolA is announced only to PeerA and PeerB,
while a service IP from PoolB is announced only to PeerB and PeerC.

In order to limit the set of peers for a given advertisement, the peers must be set:

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: example
  namespace: metallb-system
spec:
  ipAddressPools:
  - PoolA
  peers:
  - PeerA
  - PeerB
```

In this way, all the IPs coming from `PoolA` will be advertised only to `PeerA` and `PeerB`.

### Configuring the BGP source address

When a host has multiple network interfaces or multiple IP addresses
configured on one interface, the host's TCP/IP stack usually selects
the IP address that is used as the source IP address for outbound
connections automatically. This is true also for BGP connections.

Sometimes, the automatically-selected address may not be the desired
one for some reason. In such cases, MetalLB supports explicitly
specifying the source address to be used when establishing a BGP
session:

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: example
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
  peerPort: 179
  sourceAddress: 172.30.0.2
  nodeSelectors:
  - matchLabels:
      kubernetes.io/hostname: node-1
```

The configuration above tells the MetalLB speaker to check if the
address `172.30.0.2` exists locally on one of the host's network
interfaces, and if so - to use it as the source address when
establishing BGP sessions. If the address isn't found, the default
behavior takes place (that is, the kernel selects the source address
automatically).

{{% notice warning %}}
In most cases the `source-address` field should only be used with
**per-node peers**, i.e. peers with node selectors which select only
one node.

By default, a BGP peer configured under the `peers` configuration
section runs on **all** speaker nodes. It is likely meaningless to use
the `source-address` field in a peer configuration that applies to
more than one node because two nodes in a given network usually
shouldn't have the same IP address.
{{% /notice %}}

### Community Aliases

It's possible to define aliases for BGP Communities used when advertising. This is done by using
the `Community` CRD that allows to associate a name to a given BGP community:

```yaml
apiVersion: metallb.io/v1beta1
kind: Community
metadata:
  name: communities
  namespace: metallb-system
spec:
  communities:
  - name: vpn-only
    value: 1234:1
  - name: NO_ADVERTISE
    value: 65535:65282
```

After defining an alias, it can be used in the `BGPAdvertisement` in place of its
two 16 bits number format:

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  name: local
  namespace: metallb-system
spec:
  ipAddressPools:
  - first-pool
  aggregationLength: 32
  localPref: 100
  communities:
  - vpn-only
```

### Peering and annoucing via a VRF

It's possible to establish a BGP connection using interfaces having a [linux vrf](https://docs.kernel.org/networking/vrf.html)
as master. In order to do so, the `vrf` field of the `BGPPeer` structure must be set:

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: example
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
  peerPort: 179
  vrf: "red"
```

By setting a vrf, MetalLB will establish the bgp / bfd session using the interfaces
having the given VRF as master, and announce the services through the interface the
session is established from.

{{% notice note %}}
MetalLB will attract the traffic toward the interface in the VRF, but some setup on
the host network is required in order to allow the traffic to reach the CNI.
This falls outside of the responsabilities of MetalLB.
{{% /notice %}}

### Configuring with FRR-K8s

When deploying MetalLB with the, the [FRR-K8s api](https://github.com/metallb/frr-k8s/blob/main/API-DOCS.md)
can be used in combination with MetalLB's one.

In this mode, MetalLB will generate [FRRConfiguration](https://github.com/metallb/frr-k8s/blob/main/API-DOCS.md#frrconfiguration)
instance for each node where a speaker is running on.

The configuration generated by MetalLB can be merged with other `FRRConfiguration`s created by the
user (or by other controllers), provided that [the configuration compatibility guidelines](https://github.com/metallb/frr-k8s/blob/main/README.md#how-multiple-configurations-are-merged-together) are honored.

For example, we can enable a MetalLB `BGPPeer` to receive incoming prefixes:

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: peer
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
---
apiVersion: frrk8s.metallb.io/v1beta1
kind: FRRConfiguration
metadata:
  name: with-recv
  namespace: metallb-system
spec:
  bgp:
    routers:
    - asn: 64512
      neighbors:
      - address: 172.30.0.3
        asn: 64512
        toReceive:
          allowed:
            mode: all
```
### Graceful Restart

BGP Graceful Restart (GR) functionality [(RFC-4724)](https://datatracker.ietf.org/doc/html/rfc4724) defines the mechanism
that allows the BGP routers to continue to forward data packets along known
routes while the routing protocol information is being restored. GR can be
applied when the control plane is independent from the forwarding plane, which
is the case for the most Kubernetes clusters. This feature was added to
minimize network disruptions during upgrades.

GR can be applied per BGP neighbor by setting the field `enableGracefulRestart`
to true, note that this field is immutable. For example

```yaml
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: example
  namespace: metallb-system
spec:
  myASN: 64512
  peerASN: 64512
  peerAddress: 172.30.0.3
  enableGracefulRestart: true
```

#### GR With BFD

According to the [RFC-5881/BFD Shares Fate with the Control
Plane](https://datatracker.ietf.org/doc/html/rfc5882#section-4.3.2), BFD and
Graceful Restart can work together, but is implementation specific. It is up to
vendor's recommendation and needs to be tested.
