---
title: Issues with Calico
weight: 20
---

Simple setups with [Calico](https://docs.projectcalico.org/) don't
require anything special, you can just install and configure MetalLB
as usual and enjoy.

However, if you are using
Calico's
[external BGP peering capability](https://docs.projectcalico.org/v3.0/usage/configuration/bgp) to
advertise your cluster prefixes over BGP, and also want to use BGP in
MetalLB, you will need to jump through some hoops.

## The problem

BGP only allows one session to be established per pair of nodes. So,
if Calico has a session established with your BGP router, MetalLB
cannot establish its own session – it'll get rejected as a duplicate
by BGP's conflict resolution algorithm.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA
      calicoA
    end
    subgraph ""
      metallbB
      calicoB
    end
    metallbA(MetalLB<br>speaker)-. "LB routes<br>(doesn't work)" .->router(BGP Router)
    calicoA("Calico")-- Cluster routes -->router

    metallbB(MetalLB<br>speaker)-. "LB routes<br>(doesn't work)" .->router
    calicoB(Calico)-- Cluster routes -->router
{{< /mermaid >}}

Unfortunately, Calico does not currently provide the extension points
we would need to make MetalLB coexist peacefully. There
are
[bugs](https://github.com/projectcalico/calico/issues/1603) [filed](https://github.com/projectcalico/calico/issues/1604) with
Calico to add these extension points, but in the meantime, we can only
offer some hacky workarounds.

## Workaround: Peer with spine routers

If you are deploying to a cluster using a traditional "rack and spine"
router architecture, you can work around the limitation imposed by BGP
with some clever choice of peering.

Let's start with the network architecture, and see how we can add in
MetalLB:

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA("MetalLB<br>Speaker")
      calicoA
    end
    subgraph ""
      calicoB
      metallbB("MetalLB<br>Speaker")
    end

    subgraph ""
      metallbC("MetalLB<br>Speaker")
      calicoC
    end
    subgraph ""
      calicoD
      metallbD("MetalLB<br>Speaker")
    end

    calicoA("Calico")-->torA(ToR Router)
    calicoB("Calico")-->torA

    calicoC("Calico")-->torB(ToR Router)
    calicoD("Calico")-->torB
    
    torA-->spine(Spine Router)
    torB-->spine(Spine Router)
{{< /mermaid >}}

In this architecture, we have 4 machines in our Kubernetes cluster,
spread across 2 racks. Each rack has a top-of-rack (ToR) router, and
both ToRs connect to an upstream "spine" router.

The arrows represent BGP peering sessions: Calico has been configured
to not automatically mesh with itself, but to instead peer with the
ToRs. The ToRs in turn peer with the spine, which propagates routes
throughout the cluster.

Ideally, we would like MetalLB to connect to the ToRs in the same way
that Calico does. However, Calico is already "consuming" the one
allowed BGP session between machine and ToR.

The alternative is to make MetalLB peer with the spine router(s):

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA("MetalLB<br>Speaker")
      calicoA
    end
    subgraph ""
      calicoB
      metallbB("MetalLB<br>Speaker")
    end

    subgraph ""
      metallbC("MetalLB<br>Speaker")
      calicoC
    end
    subgraph ""
      calicoD
      metallbD("MetalLB<br>Speaker")
    end

    calicoA("Calico")-->torA(ToR Router)
    calicoB("Calico")-->torA

    calicoC("Calico")-->torB(ToR Router)
    calicoD("Calico")-->torB
    
    torA-->spine(Spine Router)
    torB-->spine(Spine Router)
    
    metallbA-->spine
    metallbB-->spine
    metallbC-->spine
    metallbD-->spine
{{< /mermaid >}}

Properly configured, the spine can redistribute MetalLB's routes to
anyone that needs them. And, because there are no preexisting BGP
sessions between the machines and the spine, there is no conflict
between Calico and MetalLB.

The downside of this option is additional configuration complexity,
and a loss of scalability: instead of scaling the number of spine BGP
sessions by the number of racks in your cluster, you're once again
scaling by the total number of machines. In some deployments, this may
not be acceptable.

In large clusters, another compromise might be to dedicate only
certain racks to externally facing services: constrain the MetalLB
speaker daemonset to schedule only on those racks, and either use the
"Cluster" `externalTrafficPolicy`, or also constrain the pods of the
externally facing services to run on those racks.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA("MetalLB<br>Speaker")
      calicoA
    end

    subgraph ""
      calicoB
    end

    calicoA("Calico")-->torA(ToR Router)
    calicoB("Calico")-->torB(ToR Router)

    torA-->spine(Spine Router)
    torB-->spine(Spine Router)
    
    metallbA-->spine
{{< /mermaid >}}

## Workaround: Router VRFs

If your networking hardware supports VRFs (Virtual Routing and
Forwarding), you may be able to "split" your router in two, and peer
Calico and MetalLB to separate halves of the same router. Then, with
judicious inter-VRF route leaking, you can re-merge the two routing
tables.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA("MetalLB<br>Speaker")
      calicoA("Calico")
    end
    subgraph ""
      calicoB("Calico")
      metallbB("MetalLB<br>Speaker")
    end

    subgraph Router
      torA("Router VRF 1")
      torB("Router VRF 2")
    end

    calicoA-->torA
    calicoB-->torA

    metallbA-->torB
    metallbB-->torB
    
    torB-. "Careful route<br>propagation" .->torA
{{< /mermaid >}}

While this should theoretically work, it hasn't been demonstrated, and
setting it up varies wildly based on which routing software/hardware
you are interfacing with. If you get this working,
please [let us know](https://github.com/metallb/metallb/issues/new),
especially if you have tips on how to make this work best!

## Ideas wanted!

None of these workarounds are very satisfying. Until Calico supports
more advanced configuration for BGP peers, we are stuck doing
acrobatics to integrate the two.

If you have an idea for another workaround that would enable Calico
and MetalLB to coexist nicely,
please [tell us](https://github.com/metallb/metallb/issues/new) !
