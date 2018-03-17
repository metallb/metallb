---
title: Usage
weight: 5
---

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

## Traffic policies

MetalLB understands the service's `externalTrafficPolicy` option, and
implements different announcements modes depending on the policy and
announcement protocol you select.

### Layer2

When announcing in layer2 mode, MetalLB forces the traffic policy to
`Cluster`. In this mode, the elected leader node receives all the
inbound traffic, and `kube-proxy` load-balances from there to
individual pods.

This policy results in uniform traffic distribution across all pods in
the service. However, `kube-proxy` will obscure the source IP address
of the connection when it does load-balancing, so your pod logs will
show that external traffic appears to be coming from the cluster's
leader node.

### BGP

When announcing over BGP, MetalLB respects the service's
`externalTrafficPolicy` option, and implements two different
announcement modes depending on what policy you select. If you're
familiar with Google Cloud's Kubernetes load balancers, you can
probably skip this section: MetalLB's behaviors and tradeoffs are
identical.

#### "Cluster" traffic policy

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

#### "Local" traffic policy

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
best mode to use with BGP
announcements. See
[issue 1](https://github.com/google/metallb/issues/1) for more
information.
