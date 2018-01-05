---
title: Usage
weight: 4
---

## Configuration

To configure MetalLB, write a config map to `metallb-system/config`

There is an example configmap in [`manifests/example-config.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/example-config.yaml),
annotated with explanatory comments.

For a basic configuration featuring one BGP router and one IP address
range, you need 4 pieces of information:

- The router IP address that MetalLB should connect to,
- The router's AS number,
- The AS number MetalLB should use,
- The IP address range expressed as a CIDR prefix.

As an example, if you want to give MetalLB the range 192.168.10.0/24
and AS number 42, and connect it to a router at 10.0.0.1 with AS
number 100, your configuration will look like:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    peers:
    - peer-address: 10.0.0.1
      peer-asn: 100
      my-asn: 42
    address-pools:
    - name: default
      protocol: bgp
      cidr:
      - 192.168.10.0/24
```

## Simple balancers

Once MetalLB is installed and configured, to expose a service
externally, simply create it with `spec.type` set to `LoadBalancer`,
and MetalLB will do the rest.

MetalLB attaches informational events to the services that it's
controlling. If your LoadBalancer is misbehaving, run `kubectl
describe service <service name>` and check the event log.

## Requesting specific IPs

MetalLB respects the `spec.loadBalancerIP` parameter, so if you want
your service to be set up with a specific address, you can request it
by setting that parameter. If MetalLB does not own the requested
address, or if the address is already in use by another service,
assignment will fail and MetalLB will log a warning event visible in
`kubectl describe service <service name>`.

MetalLB also supports requesting a specific address pool, if you want
a certain kind of address but don't care which one exactly. To request
assignment from a specific pool, add the
`metallb.universe.tf/address-pool` annotation to your service, with the
name of the address pool as the annotation value. For example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

## Control automatic address pool allocation

In some environments, you'll have some large address pools of "cheap" IPs
(e.g. RFC1918), and some smaller pools of "expensive" IPs (e.g. public
IPv4 addresses leased on the grey market).

By default, MetalLB will allocate IPs from any configured address pool
with free addresses. This might end up using "expensive" addresses for
services that don't require it.

To prevent this behaviour you can disable automatic allocation for a pool
by setting the `auto-assign` flag to `false`:

```yaml
# Rest of config omitted for brevity
address-pools:
- name: cheap
  protocol: bgp
  cidr:
  - 192.168.144.0/20
- name: expensive
  protocol: bgp
  cidr:
  - 42.176.25.64/30
  auto-assign: false
```

Addresses can still be specifically allocated from the "expensive" pool
with the methods described in the "Requesting specific IPs" section above.

## Traffic policies

MetalLB respects the service's `externalTrafficPolicy` option, and
implements two different announcement modes depending on what policy
you select. If you're familiar with Google Cloud's Kubernetes load
balancers, you can probably skip this section: MetalLB's behaviors and
tradeoffs are identical.

### "Cluster" traffic policy

With the default `Cluster` traffic policy, every node in your cluster
will attract traffic for the service IP. On each node, the traffic is
subjected to a second layer of load-balancing (provided by
`kube-proxy`), which directs the traffic to individual pods.

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

### "Local" traffic policy

With the `Local` traffic policy, nodes will only attract traffic if
they are running one or more of the service's pods locally. The BGP
routers will load-balance incoming traffic only across those nodes
that are currently hosting the service. On each node, the traffic is
forwarded only to local pods by `kube-proxy`, there is no "horizontal"
traffic flow between nodes.

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

## Example

As an example of how to use all of MetalLB's options, consider an
ecommerce site that runs a production environment and multiple
developer sandboxes side by side. The production environment needs
public IP addresses, but the sandboxes can use private IP space,
routed to the developer offices through a VPN.

Additionally, because the production IPs end up hardcoded in various
places (DNS, security scans for regulatory compliance...), we want
specific services to have specific addresses in production. On the
other hand, sandboxes come and go as developers bring up and tear down
environments, so we don't want to manage assignments by hand.

We can translate these requirements into MetalLB. First, we define two
address pools, and set BGP attributes to control the visibility of
each set of addresses:

```yaml
# Rest of config omitted for brevity
communities:
  # Our datacenter routers understand a "VPN only" BGP community.
  # Announcements tagged with this community will only be propagated
  # through the corporate VPN tunnel back to developer offices.
  vpn-only: 1234:1
address-pools:
- # Production services will go here. Public IPs are expensive, so we leased
  # just 4 of them.
  name: production
  protocol: bgp
  cidr:
  - 42.176.25.64/30

- # On the other hand, the sandbox environment uses private IP space,
  # which is free and plentiful. We give this address pool a ton of IPs,
  # so that developers can spin up as many sandboxes as they need.
  name: sandbox
  protocol: bgp
  cidr:
  - 192.168.144.0/20
  bgp-advertisements:
  - communities:
    - vpn-only
```

In our Helm charts for sandboxes, we tag all services with the
annotation `metallb.universe.tf/address-pool: sandbox`. Now, whenever
developers spin up a sandbox, it'll come up on some IP address within
192.168.144.0/20.

For production, we set `spec.loadBalancerIP` to the exact IP address
that we want for each service. MetalLB will check that it makes sense
given its configuration, but otherwise will do exactly as it's told.

## Limitations

MetalLB uses the BGP routing protocol to implement
load-balancing. This has the advantage of simplicity, in that you
don't need specialized equipment, but it comes with some downsides as
well.

The biggest is that BGP-based load balancing does not react gracefully
to changes in the _backend set_ for an address. When a cluster node
goes down, you should expect _all_ active connections to your service
to be broken (users will see "Connection reset by peer").

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

The consequence of this is that any time the IP-to-Node mapping
changes for your service, you should expect to see a one-time hit
where most active connections to the service break. There's no ongoing
packet loss or blackholing, just a one-time clean break.

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
