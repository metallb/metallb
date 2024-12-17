---
title: Usage
weight: 5
---

After MetalLB is installed and configured, to expose a service
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

MetalLB supports `spec.loadBalancerIP` and a custom `metallb.io/loadBalancerIPs`
annotation. The annotation also supports a comma separated list of IPs to be used in case of
Dual Stack services.

Please note that `spec.LoadBalancerIP` is planned to be deprecated in [k8s apis](https://github.com/kubernetes/kubernetes/pull/107235).

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.io/loadBalancerIPs: 192.168.1.100
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

MetalLB also supports requesting a specific address pool, if you want
a certain kind of address but don't care which one exactly. To request
assignment from a specific pool, add the
`metallb.io/address-pool` annotation to your service, with the
name of the address pool as the annotation value. For example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.io/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

## Traffic policies

MetalLB understands and respects the service's `externalTrafficPolicy` option,
and implements different announcements modes depending on the policy and
announcement protocol you select.

### Layer2

When announcing in layer2 mode, one node in your cluster will attract traffic
for the service IP. From there, the behavior depends on the selected traffic
policy.

#### "Cluster" traffic policy

With the default `Cluster` traffic policy, `kube-proxy` on the node that
received the traffic does load balancing, and distributes the traffic to all the
pods in your service.

This policy results in uniform traffic distribution across all pods in
the service. However, `kube-proxy` will obscure the source IP address
of the connection when it does load balancing, so your pod logs will
show that external traffic appears to be coming from the service's
leader node.

#### "Local" traffic policy

With the `Local` traffic policy, `kube-proxy` on the node that received the
traffic sends it only to the service's pod(s) that are on the _same_ node. There
is no "horizontal" traffic flow between nodes.

Because `kube-proxy` doesn't need to send traffic between cluster nodes, your
pods can see the real source IP address of incoming connections.

The downside of this policy is that incoming traffic only goes to some pods in
the service. Pods that aren't on the current leader node receive no traffic,
they are just there as replicas in case a failover is needed.

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
subjected to a second layer of load balancing (provided by
`kube-proxy`), which directs the traffic to individual pods.

This policy results in uniform traffic distribution across all nodes
in your cluster, and across all pods in your service. However, it
results in two layers of load balancing (one at the BGP router, one at
`kube-proxy` on the nodes), which can cause inefficient traffic
flows. For example, a particular user's connection might be sent to
node A by the BGP router, but then node A decides to send that
connection to a pod running on node B.

The other downside of the "Cluster" policy is that `kube-proxy` will
obscure the source IP address of the connection when it does its
load balancing, so your pod logs will show that external traffic
appears to be coming from your cluster's nodes.

#### "Local" traffic policy

With the `Local` traffic policy, nodes will only attract traffic if
they are running one or more of the service's pods locally. The BGP
routers will load balance incoming traffic only across those nodes
that are currently hosting the service. On each node, the traffic is
forwarded only to local pods by `kube-proxy`, there is no "horizontal"
traffic flow between nodes.

This policy provides the most efficient flow of traffic to your
service. Furthermore, because `kube-proxy` doesn't need to send
traffic between cluster nodes, your pods can see the real source IP
address of incoming connections.

The downside of this policy is that it treats each cluster node as one
"unit" of load balancing, regardless of how many of the service's pods
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
[issue 1](https://github.com/metallb/metallb/issues/1) for more
information.

## IPv6 and dual stack services

IPv6 and dual stack services are supported in L2 mode, and in BGP mode only
via the FRR mode.

In order for MetalLB to allocate IPs to a dual stack service, there must be
at least one address pool having both addresses of version v4 and v6.

Note that in case of dual stack services, it is not possible to use
`spec.loadBalancerIP` as it does not allow to request for multiple IPs,
so the annotation `metallb.io/loadBalancerIPs` must be used.

## IP address sharing

By default, Services do not share IP addresses. If you have a need to
colocate services on a single IP, you can enable selective IP sharing
by adding the `metallb.io/allow-shared-ip` annotation to
services.

The value of the annotation is a "sharing key." Services can share an
IP address under the following conditions:

- They both have the same sharing key.
- They request the use of different ports (e.g. tcp/80 for one and
  tcp/443 for the other).
- They both use the `Cluster` external traffic policy, or they both point to the
  _exact_ same set of pods (i.e. the pod selectors are identical).

If these conditions are satisfied, MetalLB _may_ colocate the two
services on the same IP, but does not have to. If you want to ensure
that they share a specific address, use the `spec.loadBalancerIP`
functionality described above.

There are two main reasons to colocate services in this fashion: to
work around a Kubernetes limitation, and to work with limited IP
addresses.

Here is an example configuration of two services that share the same ip address:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-service-tcp
  namespace: default
  annotations:
    metallb.io/allow-shared-ip: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  loadBalancerIP: 1.2.3.4
  ports:
    - name: dnstcp
      protocol: TCP
      port: 53
      targetPort: 53
  selector:
    app: dns
---
apiVersion: v1
kind: Service
metadata:
  name: dns-service-udp
  namespace: default
  annotations:
    metallb.io/allow-shared-ip: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  loadBalancerIP: 1.2.3.4
  ports:
    - name: dnsudp
      protocol: UDP
      port: 53
      targetPort: 53
  selector:
    app: dns
```

This might be useful in case you have more services than
available IP addresses, and you can't or don't want to get more
addresses, the only alternative is to colocate multiple services per
IP address.
