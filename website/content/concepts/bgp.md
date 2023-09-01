---
title: MetalLB in BGP mode
weight: 2
---

In BGP mode, each node in your cluster establishes a BGP peering
session with your network routers, and uses that peering session to
advertise the IPs of external cluster services.

Assuming your routers are configured to support multipath, this
enables true load balancing: the routes published by MetalLB are
equivalent to each other, except for their nexthop. This means that
the routers will use all nexthops together, and load balance between
them.

After the packets arrive at the node, `kube-proxy` is responsible for
the final hop of traffic routing, to get the packets to one specific
pod in the service.

## Load-balancing behavior

The exact behavior of the load balancing depends on your specific
router model and configuration, but the common behavior is to balance
_per-connection_, based on a _packet hash_. What does this mean?

Per-connection means that all the packets for a single TCP or UDP
session will be directed to a single machine in your cluster. The
traffic spreading only happens _between_ different connections, not
for packets within one connection.

This is a _good_ thing, because spreading packets across multiple
cluster nodes would result in poor behavior on several levels:

- Spreading a single connection across multiple paths results in
  packet reordering on the wire, which drastically impacts performance
  at the end host.
- On-node traffic routing in Kubernetes is not guaranteed to be
  consistent across nodes. This means that two different nodes could
  decide to route packets for the same connection to different pods,
  which would result in connection failures.

Packet hashing is how high-performance routers can statelessly spread
connections across multiple backends. For each packet, they extract
some of the fields, and use those as a "seed" to deterministically
pick one of the possible backends. If all the fields are the same, the
same backend will be chosen.

The exact hashing methods available depend on the router hardware and
software. Two typical options are _3-tuple_ and _5-tuple_
hashing. 3-tuple uses `(protocol, source-ip, dest-ip)` as the key,
meaning that all packets between two unique IPs will go to the same
backend. 5-tuple hashing adds the source and destination ports to the
mix, which allows different connections from the same clients to be
spread around the cluster.

In general, it's preferable to put as much _entropy_ as possible into
the packet hash, meaning that using more fields is generally
good. This is because increased entropy brings us closer to the
"ideal" load-balancing state, where every node receives exactly the
same number of packets. We can never achieve that ideal state because
of the problems we listed above, but what we can do is try and spread
connections as evenly as possible, to try and prevent hotspots from
forming.

## Limitations

Using BGP as a load-balancing mechanism has the advantage that you can
use standard router hardware, rather than bespoke
load balancers. However, this comes with downsides as well.

The biggest downside is that BGP-based load balancing does not react gracefully
to changes in the _backend set_ for an address. What this means is
that when a cluster node goes down, you should expect _all_ active
connections to your service to be broken (users will see "Connection
reset by peer").

BGP-based routers implement stateless load balancing. They assign a
given packet to a specific next hop by hashing some fields in the
packet header, and using that hash as an index into the array of
available backends.

The problem is that the hashes used in routers are usually not
_stable_, so whenever the size of the backend set changes (for example
when a node's BGP session goes down), existing connections will be
rehashed effectively randomly, which means that the majority of
existing connections will end up suddenly being forwarded to a
different backend, one that has no knowledge of the connection in
question.

The consequence of this is that any time the IP→Node mapping changes
for your service, you should expect to see a one-time hit where most
active connections to the service break. There's no ongoing packet
loss or blackholing, just a one-time clean break.

Depending on what your services do, there are a couple of mitigation
strategies you can employ:

- Your BGP routers might have an option to use a more stable ECMP
  hashing algorithm. This is sometimes called "resilient ECMP" or
  "resilient LAG". Using such an algorithm hugely reduces the number
  of affected connections when the backend set changes.
- Pin your service deployments to specific nodes, to minimize the pool
  of nodes that you have to be "careful" about.
- Schedule changes to your service deployments during "trough", when
  most of your users are asleep and your traffic is low.
- Split each logical service into two Kubernetes services with
  different IPs, and use DNS to gracefully migrate user traffic from
  one to the other prior to disrupting the "drained" service.
- Add transparent retry logic on the client side, to gracefully
  recover from sudden disconnections. This works especially well if
  your clients are things like mobile apps or rich single-page web
  apps.
- Put your services behind an ingress controller. The ingress
  controller itself can use MetalLB to receive traffic, but having a
  stateful layer between BGP and your services means you can change
  your services without concern. You only have to be careful when
  changing the deployment of the ingress controller itself (e.g. when
  adding more NGINX pods to scale up).
- Accept that there will be occasional bursts of reset
  connections. For low-availability internal services, this may be
  acceptable as-is.

## FRR Mode

MetalLB provides a deployment mode that uses FRR as a backend for the BGP
layer.

When the FRR mode is enabled, the following additional features are available:

- BGP sessions with [BFD support](https://metallb.universe.tf/concepts/bgp/#limitations)
- IPv6 Support for BGP and BFD
- Multi Protocol BGP

Please also note that with the current FRR version is not possible to peer within
the same host, while with the native implementation allows it.

### Limitations of the FRR Mode

Compared to the native implementation, the FRR mode has the following limitations:

- The RouterID field of the BGPAdvertisement can be overridden, but it must be the same for all
the advertisements (there can't be different advertisements with different RouterIDs).

- The myAsn field of the BGPAdvertisement can be overridden, but it must be the same for all
the advertisements (there can't be different advertisements with different myAsn).

- In case a eBGP Peer is multiple hops away from the nodes, the ebgp-multihop flag must be set
to true.
