# MetalLB Concepts and Limitations

MetalLB hooks into your Kubernetes cluster, and provides a network
load-balancer implementation. In short, it allows you to create
Kubernetes services of type "LoadBalancer" in clusters that don't run
on a cloud provider, and thus cannot simply hook into paid products to
provide load-balancers.

It has two features that work together to provide this service:
address allocation, and external announcement.

# Address allocation

In a cloud-enabled Kubernetes cluster, you request a load-balancer,
and your cloud platform assigns an IP address to you. In a bare metal
cluster, MetalLB is responsible for that allocation.

MetalLB cannot create IP addresses out of thin air, so you do have to
give it _pools_ of IP addresses that it can use. MetalLB will take
care of assigning and unassigning individual addresses as services
come and go, but it will only ever hand out IPs that are part of its
configured pools.

How you get IP address pools for MetalLB depends on your
environment. If you're running a bare metal cluster in a colocation
facility, your hosting provider probably offers IP addresses for
lease. In that case, you would lease, say, a /26 of IP space (64
addresses, and provide that range to MetalLB for cluster services.

Alternatively, your cluster might be purely private, providing
services to a nearby LAN but not exposed to the internet. In that
case, you could pick a range of IPs from one of the private address
spaces (so-called RFC1918 addresses), and assign those to
MetalLB. Such addresses are free, and work fine as long as you're only
providing cluster services to your LAN.

Or, you could do both! MetalLB lets you define as many address pools
as you want, and doesn't care what "kind" of addresses you give it.

# External announcement

Once MetalLB has assigned an external IP address to a service, it
needs to make the network beyond the cluster aware that the IP "lives"
in the cluster.

MetalLB does this by speaking BGP to a nearby network router that you
control, and telling it how to forward traffic to assigned service
IPs.

Again, the specifics of the routers and BGP advertisements depend on
your particular environment, there is no "one size fits all"
solution. These specifics make up the bulk of MetalLB's configuration
file, to give you the flexibility to adapt MetalLB to your cluster.

# Announcement modes

MetalLB respects the service's `externalTrafficPolicy` option, and
implements two different announcement modes depending on what policy
you select. If you're familiar with Google Cloud's Kubernetes load
balancers, you can probably skip this section: MetalLB's behaviors and
tradeoffs are identical.

## "Cluster" traffic policy

With the default `Cluster` traffic policy, every node in your cluster
will advertise to configured BGP routers that they are a valid
destination for your service's IP address. The BGP routers will
load-balance incoming traffic across all nodes in the cluster. On each
node, the traffic is subjected to a second layer of load-balancing
(provided by `kube-proxy`), that actually directs the traffic to
individual pods.

This policy results in uniform traffic distribution across all nodes
in your cluster, and across all pods in your service. However, it
results in two layers of load-balancing (one at the BGP router, one at
`kube-proxy` on the nodes), which can cause inefficient traffic
flows. For example, a particular user's connection might be sent to
node A by the BGP router, but then node A decides to send that
connection to a pod running on node B.

The other downside of the "Cluster" policy is that `kube-proxy` will
obscure the source IP address of the connection when it does its
load-balancing, so your pod logs will show that external traffic
appears to be coming from your cluster's nodes.

## "Local" traffic policy

With the `Local` traffic policy, nodes will only advertise themselves
as a destination to configured BGP routers if they are running one or
more of the service's pods locally. The BGP routers will load-balance
incoming traffic only across those nodes that are currently hosting
the service. On each node, the traffic is forwarded only to local pods
by `kube-proxy`, there is no "horizontal" traffic flow between nodes.

This policy provides the most efficient flow of traffic to your
service. Furthermore, because `kube-proxy` doesn't need to send
traffic between cluster nodes, your pods can see the real source IP
address of incoming connections.

The downside of this policy is that it treats each cluster node as one
"unit" of load-balancing, regardless of how many of the service's pods
are running on that node. This may result in traffic imbalances to
your pods.

For example, if your service has 2 pods running on node A and one pod
running on node B, the `Local` traffic policy will send 50% of the
service's traffic to each _node_. Node A will split the traffic it
receives evenly between its two pods, so the final per-pod load
distribution is 25% for each of node A's pods, and 50% for node B's
pod. In contrast, if you used the `Cluster` traffic policy, each pod
would receive 33% of the overall traffic.

In general, when using the `Local` traffic policy, it's recommended to
finely control the mapping of your pods to nodes, for example
using
[node anti-affinity](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity),
so that an even traffic split across nodes translates to an even
traffic split across pods.

In future, MetalLB might be able to overcome the downsides of the
`Local` traffic policy, in which case it would be unconditionally the
best mode to use in MetalLB-powered clusters. See #1 for more
information.

# Limitations of MetalLB

MetalLB uses the BGP routing protocol to implement
load-balancing. This has the advantage of simplicity, in that you
don't need specialized equipment, but it comes with some downsides as
well.

The biggest is that BGP-based load balancing does not react gracefully
to changes in the _backend set_ for an address. What it effectively
means is that when a cluster node goes down, you should expect _all_
active connections to your service to be broken (users will see
"Connection reset by peer").

BGP-based routers implement stateless load-balancing. They assign a
given packet to a specific next hop by hashing some fields in the
packet header, and using that hash as an index into the array of
available backends.

The problem is that the hashes used in routers are not _stable_, so
whenever the size of the backend set changes (for example when a
node's BGP session goes down), existing connections will be rehashed
effectively randomly, which means that the majority of existing
connections will end up suddenly being forwarded to a different
backend, one that has no knowledge of the connection in question.

The consequence of this is that any time the IP→Node mapping changes
for your service, you should expect to see a one-time hit where most
active connections to the service break. There's no ongoing packet
loss or blackholing, just a one-time clean break.

Depending on what your services do, there are a couple of mitigation
strategies you can employ:
- Pin your service deployments to specific nodes, to minimize the pool
  of nodes that you have to be "careful" about.
- Schedule changes to your service deployments during "trough", when
  most of your users are asleep and your traffic is low.
- Split each logical service into two Kubernetes services with
  different IPs, and use DNS to gracefully migrate user traffic from
  one to the other prior to disrupting the "drained" service.
- Add transparent retry logic on the client side, to gracefully
  recover from sudden disconnections. This works especially well if
  your clients are things like mobile apps or rich single page web
  apps.
- Put your services behind an ingress controller. The ingress
  controller itself can use MetalLB to receive traffic, but having a
  stateful layer between BGP and your services means you can change
  your services without concern. You only have to be careful when
  changing the deployment of the ingress controller itself (e.g. when
  adding more nginx pods to scale up).
- Accept that there will be occasional bursts of reset
  connections. For low-availability internal services, this may be
  acceptable as-is.
