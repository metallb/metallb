---
title: Integrating with Romana
weight: 10
---

Simple setups with [Romana](http://docs.romana.io/welcome.html) don't
require anything special, you can just install and configure MetalLB
as usual and enjoy.

However, if you are using
Romana's
[route publisher addon](http://docs.romana.io/Content/advanced.html#route-publisher-add-on) to
advertise your cluster prefixes over BGP, and also want to use BGP in
MetalLB, you will need a slightly unusual setup.

## The problem

BGP only allows one session to be established per pair of nodes. So,
if Romana's route publisher has a session established with your BGP
router, MetalLB cannot establish its own session – it'll get rejected
as a duplicate by BGP's conflict resolution algorithm.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA
      romanaA
    end
    subgraph ""
      metallbB
      romanaB
    end
    metallbA(MetalLB<br>speaker)-. "LB routes<br>(doesn't work)" .->router(BGP Router)
    romanaA("Romana Route<br>Publisher")-- Cluster routes -->router

    metallbB(MetalLB<br>speaker)-. "LB routes<br>(doesn't work)" .->router
    romanaB(Romana Route<br>Publisher)-- Cluster routes -->router
{{< /mermaid >}}

Fortunately, Romana's route publisher is very configurable, and can be
configured to act as an intermediary for MetalLB. In this
configuration, MetalLB on each node peers with the Romana route
publisher on the same node. The publisher then peers with external
routers, and publishes both Romana's and MetalLB's routes externally.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA(MetalLB<br>speaker)-- LB routes -->romanaA
    end
    subgraph ""
      metallbB(MetalLB<br>speaker)-- LB routes -->romanaB
    end
    romanaA("Romana Route<br>Publisher")-- "Cluster routes +<br>LB routes" -->router(BGP Router)
    romanaB(Romana Route<br>Publisher)-- "Cluster routes +<br> LB routes" -->router
{{< /mermaid >}}

## Configure Romana

Follow the instructions
in
[Romana's documentation](http://docs.romana.io/Content/advanced.html#route-publisher-add-on) to
set up the route publisher addon. In `publisher.conf`, add a neighbor
configuration for MetalLB:

```
protocol bgp metallb {
  local as 1234;
  neighbor 127.0.0.1 as 2345;

  multihop;
  passive;
}
```

Let's walk through that configuration:

- The first two lines configure Romana's local AS number, and the
  remote AS number assigned to MetalLB. The peering address is
  `127.0.0.1`, because the route publisher and MetalLB are both on the
  same machine.
- `multihop` disables some of BIRD's strict address checking, allowing
  BIRD to peer with `127.0.0.1`. Ordinarily, in BIRD's default
  `direct` mode, such a connection would be forbidden.
- `passive` tells BIRD to wait for an incoming peering connection,
  rather than try to connect out. MetalLB doesn't accept connections,
  it only connects out to other things.

The rest of the route publisher configuration is very dependent on
what your goals are, and what your infrastructure looks like. As such,
we can't really offer specific guidance.

If you use the suggested configurations in Romana's documentation, you
will need to change the export filter on your peering sessions with
the outside world, to include routes received from MetalLB. The
default filter, `export where proto = "romana_routes"`, only exports
the routes for cluster IPs. If you're using the suggested
configurations, changing that filter to `export where proto =
"romana_routes" || proto = "metallb"` is all you need.

Before proceeding further, check on your external routers that the BGP
sessions to the route publishers have established, and that you're
receiving the cluster's routes.

## Configure MetalLB

Add a peer to your MetalLB configuration that connects to the Romana
route publisher:

```yaml
peers:
- peer-asn: 1234
  my-asn: 2345
  peer-address: 127.0.0.1
  router-id: 127.0.0.1
```

Again, walking through the configuration:

- `peer-asn` and `my-asn` should be set to match the configuration you
  provided to Romana.
- `peer-address` is `127.0.0.1`, which will make the MetalLB speaker
  on each machine connect to its local Romana route publisher.
- `router-id` is forced to `127.0.0.1`. By default, MetalLB uses the
  node's IP address as the router ID, however Romana uses the same
  thing. BGP can only peer between different router IDs, so we have to
  override the default here.

Push this configuration to your cluster, and MetalLB should connect to
the Romana route publishers. Verify in MetalLB's logs that the
connection is established (it may take up to a minute, depending on
various BGP protocol timers), and verify on the external router(s)
that you're receiving routes for the load-balancer IPs in addition to
the cluster routes.

## Choosing your AS numbers

You must configure your cluster such that at least one peering session
(MetalLB to Romana, or Romana to external routers) is an eBGP
session. That is, you cannot use the same AS number for all three of
MetalLB, Romana, and the external routers. If you do, the external
routers will not receive MetalLB's routes.

BGP session established between the same AS number are internal BGP
(iBGP) sessions, and iBGP sessions are subject to different route
propagation rules than external BGP (eBGP) sessions. Specifically,
routes received from one iBGP peer are not propagated to other iBGP
peers.

In a standard BGP-based backbone, this is expected and normal: the
iBGP peers must either form a complete mesh, or connect in a star
topology to a set of fully meshed route reflectors.

Looking at our diagram again, we are using neither of these
topologies:

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA(MetalLB<br>speaker)-- LB routes -->romanaA
    end
    subgraph ""
      metallbB(MetalLB<br>speaker)-- LB routes -->romanaB
    end
    romanaA("Romana Route<br>Publisher")-- "Cluster routes +<br>LB routes" -->router(BGP Router)
    romanaB(Romana Route<br>Publisher)-- "Cluster routes +<br> LB routes" -->router
{{< /mermaid >}}

We cannot form a complete mesh, because MetalLB and Romana cannot both
peer with the external routers. Likewise, there is no route reflector
in this setup. Therefore, this is not a valid topology for a single AS.

If we nevertheless try to treat it as a valid topology, and configure
all the peerings as iBGP sessions, things will not work. MetalLB will
send its routes to the Romana route publishers, but Romana will not
forward those routes to the external routers.

Fortunately, it's simple to avoid these problems, but not having more
than one peering "hop" within the same AS number. Routes propagate
normally across a single iBGP hop, and also propagate in the intuitive
manner across eBGP sessions. Placing either MetalLB or the external
routers in an AS other than the one Romana is using will make routes
propagate correctly.

To summarize, the following setups will work correctly:

{{<mermaid align="center">}}
graph BT
    subgraph eBGP to router
      subgraph ""
        metallbA(MetalLB<br>speaker)-- iBGP -->romanaA
      end
      romanaA("Romana Route<br>Publisher")-- eBGP -->routerA(BGP Router)
    end
    
    subgraph eBGP to MetalLB
      subgraph ""
        metallbB(MetalLB<br>speaker)-- eBGP -->romanaB
      end
      romanaB("Romana Route<br>Publisher")-- iBGP -->routerB(BGP Router)
    end

    subgraph eBGP to both
      subgraph ""
        metallbC(MetalLB<br>speaker)-- eBGP -->romanaC
      end
      romanaC("Romana Route<br>Publisher")-- eBGP -->routerC(BGP Router)
    end
{{< /mermaid >}}

The only invalid configuration is using iBGP everywhere:

{{<mermaid align="center">}}
graph BT
    subgraph iBGP to both
      subgraph ""
        metallbA(MetalLB<br>speaker)-- iBGP -->romanaA
      end
      romanaA("Romana Route<br>Publisher")-- iBGP -->routerA(BGP Router)
    end
{{< /mermaid >}}
