
# MLB-0002: LoadBalancerIP DNS source

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories (Optional)](#user-stories-optional)
    - [Story 1](#story-1)
    - [Story 2](#story-2)
  - [Notes/Constraints/Caveats (Optional)](#notesconstraintscaveats-optional)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
<!-- /toc -->

## Summary


Metalb is designed to be a create and manage local load balancer. But the BGP protocol can be used to create global load balancer using the concept of equal cost multi path (ECMP).

It would be great if it was possible to use Metallb to create global load balancers.

Global Load Balancers support the use case of deploying the sam applications in active/active mode across different datacenters.

## Motivation


To support global load balancing for those scenario in which metallb and BGP is viable, without having to rely on external infrastructure.

### Goals

- Enhance Metallb with the ability to coordinate with other Metallb instances to create global load balancers
- Allow tenants of a clusters to self-service global load balancers.

### Non-Goals

- support for advanced global load balancing strategies
- support for BGP-router initiated health checks

## Proposal

It is simple to show that if two LoadBalancer Services in two different clusters are created with the same VIP and Metallb is configured with BGP, the traffic will be load balanced across the two clusters.

One option would be to have the tenants requesting the LoadBalancer Services to input the same VIP across the multi0ple clusters that need to be globally load balanced.

This would mean, though, that the tenants would have to have to be involved in the IPAM process for the VIPs, which is not desirable.

That said at the moment there is no simple way to coordinate different instances of Metallb to pick the sam VIP for the same LoadBalancer service when deployed across different clusters.

So some coordination is needed between the clusters across which one needs to establish global load balancing. And yet the metallb operator is cluster-bound and cannot "see" beyond the cluster it's running on.

We propose to use DNS as a means of coordination between the various metallb instances.

It should be possible to express the desire to have the VIP of a LoadBalancer service assigned by doing a dns lookup with a given FQDN.

For example if a LoadBalancer service is annotated with: `metallb.io/assign-from-fqdn: myglobal.service.example.io`

The controller will try to resolve the name and proceed only when the name can be resolved to a VIP. At that point the controller will assign the VIP to the service (checking, obviously, that it belong to the authorized CIRDS and that it is available).

This simple change seems enough to implement rudimentary BGP-based global load balancers. In the future perhaps it might be expanded with more load balancing strategies.

### User Stories (Optional)


#### Story 1 -- DNS driven allocation

In this example we assume taht the DNS entry for the globally load balanced service has been created by some extewrnal process, at this point we can proceed with the following defintions:

cluster1:

```yaml
apiVersion: v1
kind: Service
metadata:
 name: my-service
 annotations:
   metallb.io/assign-from-fqdn: myglobal.service.example.io
spec:
 selector:
   app.kubernetes.io/name: MyApp
 ports:
   - protocol: TCP
     port: 80
     targetPort: 9376
 type: LoadBalancer
```

cluster2:

```yaml
apiVersion: v1
kind: Service
metadata:
 name: my-service
 annotations:
   metallb.io/assign-from-fqdn: myglobal.service.example.io
spec:
 selector:
   app.kubernetes.io/name: MyApp
 ports:
   - protocol: TCP
     port: 80
     targetPort: 9376
 type: LoadBalancer
```

both service's VIPs are assigned by resolving the FQDN from the DNS. They will have the same VIP, implementing the BGP-based global load balancer.

#### Story 2 - external-DNS driver allocation

In this use story, we assume that th external-DNS operator is available and correctly configured. Then we can do the following:

cluster1:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    external-dns.alpha.kubernetes.io/hostname: myglobal.service.example.io
spec:
  selector:
    app.kubernetes.io/name: MyApp
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9376
  type: LoadBalancer
```

cluster2

```yaml
apiVersion: v1
kind: Service
metadata:
 name: my-service
 annotations:
   metallb.io/assign-from-fqdn: myglobal.service.example.io
spec:
 selector:
   app.kubernetes.io/name: MyApp
 ports:
   - protocol: TCP
     port: 80
     targetPort: 9376
 type: LoadBalancer
```

the service in cluster 1 is assigned by metallb with the usual means. Then external-dns creates the DNS entry in the managed DNS. After that the service in cluster 2 can resolve the fqdn and assigns the VIP, which is going to be the same as for cluster1.

### Notes/Constraints/Caveats (Optional)

VIPs assigned via DNS will still belong to an address pool. A series of check will occur before the IP returned by the DNS will be actually assigned. At a minium the following checks:
- the IP belongs to allowed CIDRs
- the IP is available.

A possible best practice to mitigate the risk of failure of these checks is to dedicate an IPaddress pool just for the globally load balanced services. This way the pool usage (taken IPs vs free IPs) will be the same across the clusters.

Also because it's possible that the DNS entry changes, the controller will have to check every so often with the DNS to ensure that the IP is still correct. The check frequency should be driven by the DNS record TTL.

### Risks and Mitigations

A split brain scenario is possible if different instances of metallb see different IP resolution for the same load balancer service. This might occur because of cache propagation issues. Either way, eventually teh system will reconcile to the correct state.

If an instance of MetalLB cannot reach the DNS and the VIP has already been assigned, no change should be done. If the VIP has not been assigned, the VIP should still not be assigned.

## Design Details


This `metallb.io/assign-from-fqdn` is the proposed annotation for triggering assigning the IP via DNS.