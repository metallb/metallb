# BGP Graceful Restart

This is not a design document but extra documenation for therefore
Graceful Restart feature.

### Introduction

BGP Graceful Restart (GR) functionality
[(RFC-4724)](https://datatracker.ietf.org/doc/html/rfc4724) defines the mechanism
that allows the BGP routers to continue to forward data packets along known
routes while the routing protocol information is being restored.  GR can be
applied when the control plane is independent from the forwarding plane and
therefore a restart of the control plane can happen without affecting
forwarding. This is the case for a most Kubernetes clusters where the control
plane is a host-networked process (FRR) and the forwarding plane
is the primary network CNI (Calico, Cilium, OVNK etc). This feature is
implemented in MetalLB to minimize network disruptions during planned pod
restarts that take place due to upgrades.

### Configuring GR

GR can be applied per BGP neighbor by setting the field `enableGracefulRestart`
to true.

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

### GR is Immutable Field

GR is a capability that can only be applied in the OPEN message during BGP
handshake. This is defined the BGP protocol and cannot be changed. MetalLB does
not have a user facing mechanism to reset BGP session neither is resetting
internally the peering. Therefore the option was either allow the configuration
to pass through and warn the user to `reset BGP` peering externally (e.g. by executing a
BGP command the external router) or to make it immutable and therefore
user must delete/create peers. The latter option was preferred.

{{% notice info %}}
BGP GR requires both ends to be well configured, therefore is recommended
to verify in the external peer that BGP has GR enabled

```
$show bgp neighbor <peer>
...
    Graceful Restart Capability: advertised and received
      Remote Restart timer is 120 seconds
      Address families by peer:
        IPv4 Unicast(preserved)
  Graceful restart information:
    End-of-RIB send: IPv4 Unicast
    End-of-RIB received: IPv4 Unicast
    Local GR Mode: Helper*

    Remote GR Mode: Restart
...
```
{{% /notice %}}

### GR Internal Parameters

GR has a number of internal parameters, the `traditional` defaults of FRR are kept
with one exception.

* [`bgp graceful-restart restart-time`](https://docs.frrouting.org/en/latest/bgp.html#clicmd-bgp-graceful-restart-restart-time-0-4095) is 120 seconds (by FRR).
* [`no bgp graceful-restart notifications`](https://docs.frrouting.org/en/latest/bgp.html#clicmd-bgp-graceful-restart-notification) no GR support for BGP NOTIFICATION messages (by FRR)
* [`no bgp hard-administrative-reset`](https://docs.frrouting.org/en/latest/bgp.html#clicmd-bgp-hard-administrative-reset) (by FRR).
* [`bgp long-lived-graceful-restart stale-time 0`](https://docs.frrouting.org/en/latest/bgp.html#clicmd-bgp-long-lived-graceful-restart-stale-time-1-16777215) BGP long-lived graceful restart (LLGR/[RFC-9494](https://datatracker.ietf.org/doc/rfc9494/)) is disabled (by FRR).
* [`bgp graceful-restart preserve-fw-state`]( https://docs.frrouting.org/en/latest/bgp.html#bgp-gr-preserve-forwarding-state) Forwarding State (F) bit is set (by MetalLB).

### When GR is Triggered

In context of [Kubernetes](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination), 
when the pod that runs the FRR process stops, it means that the API asks
kubelet to stop a pod and kubelet does that by sending a SIGTERM to the `pid 0`
of the pod. When FRR process receives the signal, it closes the TCP connection,
and that event triggers GR timer to the external peer. That is the desired
behavior when our Daemonset pod stops for an update, but it might be less
desirable if user reduces the set of node the pod can run or user removes
MetalLB instance when using operator. Nevertheless impact should be low because
dataplane continues to work. No other case has been identified where GR is triggered.

{{% notice info %}}
When no TCP packet is lost, the external peer should start the GR timer, nevertheless
that needs to validated/tested in the specific vendor device. Example logs from FRR

```
$ cat frr.log
...
2024/11/18 10:01:50.140 BGP: [NJ2F2-2W769] 172.18.0.2 [Event] BGP connection closed fd 23
2024/11/18 10:01:50.140 BGP: [NTX3S-9Q8YV] 172.18.0.2 [Event] BGP error 5 on fd 23
2024/11/18 10:01:50.140 BGP: [ZWCSR-M7FG9] 172.18.0.2 [FSM] TCP_connection_closed (Established->Clearing), fd 23
2024/11/18 10:01:50.140 BGP: [RPZW2-39GTY] 172.18.0.2(frr-k8s-control-plane) graceful restart timer started for 120 sec
2024/11/18 10:01:50.140 BGP: [TK2B6-ZF4MR] 172.18.0.2(frr-k8s-control-plane) graceful restart stalepath timer started for 360 sec
...
```
{{% /notice %}}


### GR and Cordoned Node

GR is orthogonal to any Kubernetes node admin [operation](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/)
and the expected behavior does not change. When admin drains/cordons a node from the cluster, the node will be
labeled as Unschedulable. When that happens all routes towards that node will
be removed as normal procedure\*. The BGP peering will remain because Daemonsets
pods continue to run on Unschedulable nodes (`--ignore-daemonsets`). The BGP
peer will not have anymore routes to the node, so UDP packets will be
redirected to other nodes, existing TCP connections will break\*\*, but no new
traffic will be blackholed. After all that, the node might be hard rebooted or removed,
then the Daemonset pod will be removed, BGP will be properly terminated
(TCP connection close), GR will start on the peer but no routes exists to be staled.

\* If service is of traffic policy "Local", then the routes will be removed
faster because any pod that is endpoint to the service will be removed from the node.

\*\* TCP connection could remain established if the service is of traffic
policy "Cluster" and if Kubernetes primary CNI plugin implements consistent-hashing
[[Cilium]](https://docs.cilium.io/en/stable/network/kubernetes/kubeproxy-free/#maglev-consistent-hashing).

**Note:** when BGP peers are removed or added, rebalancing might take place due to
the load balancing hashing mechanism of the external peer, e.g. client A
that had a connection to node X can switch to node Y just because node Z was
removed from the cluster.


### GR and BFD

According to the [RFC-5881/BFD Shares Fate with the Control
Plane](https://datatracker.ietf.org/doc/html/rfc5882#section-4.3.2), 

> If BFD shares fate with the control plane on either system (the "C"
   bit is clear in either direction), a BFD session failure cannot be
   disentangled from other events taking place in the control plane. In
   many cases, the BFD session will fail as a side effect of the restart
   taking place. As such, it would be best to avoid aborting any
   Graceful Restart taking place, if possible (since otherwise BFD and
   Graceful Restart cannot coexist).

and therefore GR and BFD can work together, and the helper router should
ignore the BFD messages during GR timer (during the green box bellow).


{{<mermaid align="center">}}
sequenceDiagram
participant kubelet
participant A as K8S FRR
participant B as External Peer
A-->>B:  Sends Graceful Restart Capability in BGP OPEN message

kubelet->>A: SIGTERM
A->>+B: TCP Close
rect rgb(192,255,193)
Note right of B: graceful restart timer started for 120 sec
Note right of B: BFD events are ignored
B-->>B:  neigh went from Established to Clearing
Note right of B: Routes are stale
A->>A: Restarting 
A->>+B: BGP Peering
B-->>B:   A went from OpenConfirm to Established
end
Note over A,B: BGP Established
Note right of B: graceful restart timer stops
Note right of B: BFD events are NOT ignored
A->>+B: Update/End-Of Rib
Note right of B: Routes are NOT stale
{{< /mermaid >}}

{{% notice warning %}}
Whether BFD and GR can be used together is implementation specific.
It is up to vendor's recommendation and needs to be tested. For example Juniper
suggests not to combine them
[link](https://www.juniper.net/documentation/us/en/software/junos/bgp/topics/topic-map/bfd-for-bgp-session.html).

One consideration to be taken into account is that doing GR/BFD between routers
that are placed in the middle of a large BGP network is different than doing GR/BFD between server
and ToR/DCGW routers.
{{% /notice %}}

### Know Issue

There are two known issues which have been fixed upstream but not being used yet by MetalLB.

- There is warning message `Graceful restart configuration changed, reset this peer to take effect` that appears
in reloader which has been fixed upstream [issue](https://github.com/FRRouting/frr/issues/15880)
- BFD removes routes momentarily [issue](https://github.com/FRRouting/frr/issues/17337)
