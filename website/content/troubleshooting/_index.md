---
title: Troubleshooting MetalLB
weight: 6
---

## General concepts

MetalLB's purpose is to attract traffic directed to the LoadBalancer IP to the cluster nodes.
Once the traffic lands on a node, MetalLB's responsibility is finished and the rest should be handled by
the cluster's CNI.

Because of that, **being able to reach the LoadBalancerIP from one of the nodes doesn't prove that MetalLB
is working** (or that it is working partially). It actually proves that the CNI is working.

Also, please be aware that **pinging the service IP won't work**. You must access the service to ensure
that it works. As much as this may sound obvious, check also if your application is behaving correctly.

### Components responsibility

MetalLB is composed by two components:

- The `controller` is in charge of assigning IPs to the services
- the `speaker`s are in charge of announcing the services via L2 or BGP

If we want to understand why a service is not getting an IP, the component to check is the controller.

On the other hand, if we want to understand why a service with an assigned IP is not being advertised, the speakers
are the components to check.

## Valid Configuration

In order to work properly, MetalLB must be fed with a valid configuration.

The MetalLB's configuration is the composition of multiple resources, such as `IPAddressPools`, `L2Advertisements`,
`BGPAdvertisements` and so on. The validation is done to the configuration as a whole, and because of that it doesn't
make sense to say that a single piece of the configuration is not valid.

The MetalLB's behavior with regards to an invalid configuration is to **mark it as stale and keep working
with the last valid one**.

Each single component validates the configuration for the part that is relevant to its function, so it might
happen that the controller validates a configuration that the speaker marks as not valid.

### Checking if a configuration is valid

There are two ways to see if a configuration is not valid:

- check for errors in the logs of the given component. Config errors are on the form `failed to parse the configuration`
plus other insights about the failure.
- look at the `metallb_k8s_client_config_stale_bool` metric on Prometheus, which tells if the given component
is running on a stale (obsolete) configuration

Note: the fact that the logs contain an `invalid configuration` log does not necessarily mean that the last loaded
configuration is not valid.

## Troubleshooting IP assignment

The controller performs the IP allocation to the services and it logs any possible issue.

Things that may cause the assignment not to work are:

- there is at least one IPAddressPool compatible with the service (including the selectors!)
- if the service is dual stack, there is at least one IPAddressPool compatible with both IPV4
and IPV6 addresses
- if the service asks for a specific IP, an IPAddressPool providing that IP exists and its selectors
are compatible with the service
- if the service asks for a specific IP used also by other services, make sure that they respect the
sharing properties described in the [official docs](https://metallb.io/usage/#ip-address-sharing).

## Troubleshooting service advertisements

### General concepts

Each speaker publishes an `announcing from node "xxx" with protocol "bgp"` event associated with the
service it is announcing.

In case of L2, only one speaker will announce the service, while in case of BGP multiple speakers
will announce the service from multiple nodes.

A `kubectl describe svc <service-name>` will show the events related to the service, and with that the
speaker(s) that are announcing the services.

A given speaker won't advertise the service if:

- there are no active endpoints backing the service
- the service has `externalTrafficPolicy=local` and there are no running endpoints on the speaker's node
- there are no L2Advertisements / BGPAdvertisements matching the speaker node (if node selectors are specified)
- the Kubernetes API reports "network not available" on the speaker's node

## MetalLB is not advertising my service from my control-plane nodes or from my single node cluster

Make sure your nodes are not labeled with the
[node.kubernetes.io/exclude-from-external-load-balancers](https://kubernetes.io/docs/reference/labels-annotations-taints/#node-kubernetes-io-exclude-from-external-load-balancers) label.
MetalLB honors that label and won't announce any service from such nodes. One way to circumvent
the issue is to provide the speakers with the `--ignore-exclude-lb` flag (either from Helm or via Kustomize).

## MetalLB says it advertises the service but reaching the service does not work

### Checking the L2 advertisement works

In order to have MetalLB advertise via L2, an **L2Advertisement instance must be created**. This is different from the original
MetalLB configuration so please follow the docs and ensure you created one.

`arping <loadbalancer ip>` from a host on the same L2 subnet will show what mac address is associated with
the Loadbalancer IP by MetalLB.

```bash
$ arping -I ens3 192.168.1.240
ARPING 192.168.1.240 from 192.168.1.35 ens3
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  1.077ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  1.321ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  0.883ms
Unicast reply from 192.168.1.240 [FA:16:3E:5A:39:4C]  0.968ms
^CSent 4 probes (1 broadcast(s))
Received 4 response(s)
```

By design, MetalLB replies with the MAC address of the interface it received the ARP request from.

#### Check if the ARP requests reach the node

`tcpdump` can be used to see if the ARP requests land on the node:

```bash
$ tcpdump -n -i ens3 arp src host 192.168.1.240
tcpdump: verbose output suppressed, use -v or -vv for full protocol decode
listening on ens3, link-type EN10MB (Ethernet), capture size 262144 bytes
17:04:40.667263 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:41.667485 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:42.667572 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
17:04:43.667545 ARP, Reply 192.168.1.240 is-at fa:16:3e:5a:39:4c, length 46
^C
4 packets captured
6 packets received by filter
0 packets dropped by kernel
```

If no replies are received, it might be that ARP requests are blocked somehow. Anti MAC spoofing mechanisms are
a pretty common reason for that.

In order to understand if this is the case, you must use TCPDump on the node that where the speaker elected to
announce the service is running and see if the ARP requests are making through. At the same time, you need to
check on the host if the ARP replies are coming back.

Additionally, the speaker produces a `got ARP request for service IP, sending response` debug log whenever it receives
an ARP request.

If multiple MACs are returned for the same LB IP, this might be because:

- Multiple speakers are replying to the ARP requests, because some kind of brain split scenario. This should be visible
in the logs of the speakers
- The IP assigned to the service was associated also with some other interface in the L2 segment
- The CNI might be replying to ARP requests associated to the LB IP

If the L2 interface selector is used but there are no compatible interfaces on the node elected, MetalLB will
produce an event on the Service which should be visible when doing `kubectl describe svc <service-name>`.

#### Using WiFi and can't reach the service?

Some devices (such as Raspberry Pi) do not respond to ARP requests when using WiFi. This can lead to a situation where the service is initially reachable, but breaks shortly afterwards.
At this stage attempting to arping will result in a timeout and the service will not be reachable.

One workaround is to enable promiscuous mode on the interface: `sudo ifconfig <device> promisc`. For example: `sudo ifconfig wlan0 promisc`


### Checking if the BGP advertisement works

In order to have MetalLB advertise via BGP, a **BGPAdvertisement instance must be created**.

Advertising via BGP means announcing the nodes as the next hop for the route.

Among the speakers that are supposed to advertise the IP, pick one and check if:

- The BGP session is up
- The IP is being advertised

The status of the session can be seen via the `metallb_bgp_session_up` metric.

#### With native mode

The information can be found on the logs of the speaker container of the speaker pod, which will produce logs
like `BGP session established` or `BGP session down`. It will also log `failed to send BGP update` in case of
advertisement failure.

#### With FRR

The FRR container in the speaker pod can be queried in order to understand the status of the session / advertisements.

Useful commands are:

- `vtysh show running-conf` to see the current FRR configuration
- `vtysh show bgp neigh <neigh-ip>` to see the status of the session. Established is the value related to a healthy BGP
session
- `vtysh show ipv4 / ipv6` to see the status of the advertisements

#### Invalid FRR Configuration (FRR Mode)

The FRR configuration that the speaker produces might be invalid. When this is the case, the speaker container
will produce a `reload error` log.

Also, the logs of the `reloader` might show if the configuration file was invalid.

#### If the BGP session is not established but the configuration looks fine

Things to check are:

- The parameters of the BGP session (including ASNs and passwords)
- The logs of the FRR container
- Use TCPDump on the nodes to see what is happening to the BGP session (remember to use the right port in case you overrode it!)

#### Check the routing table on the router

If the configuration and the logs look fine on MetalLB's side, another thing to check is the routing table
on the routers corresponding to the `BGPPeer`s.

### Troubleshooting when the service is announced correctly but still not reachable

Networking is complex and there are multiple places where it might fail. TCPDump might help
to understand where the traffic stops working.

In order to narrow down the issue, TCPDump can be used:

- on the node
- on the endpoint pod

If the traffic doesn't reach the node, it might be a network infrastructure or a MetalLB issue.
If the traffic reaches the node but not the pod, this is likely to be a CNI issue.

#### Intermittent traffic

Sometimes the LoadBalancer service is intermittent. This might be because of multiple factors:

- The entry point is different every time (especially with BGP, where we expose ECMP routes)
- The L2 leader for a given LB IP is bouncing from one node to another
- The endpoints are constantly restarting (all, or just some)
- Some endpoints are replying correctly, some others are not

#### Limiting the scope of the issue

Sometimes, narrowing down the scope helps. With many nodes and many endpoints is hard to find the right place
where to dump the traffic.

One way to make the triaging simpler is to limit the advertisement **to one single node** (using the node selector
in the BGPAdvertisement / L2Advertisement) and to limit the service's endpoints to one single pod.

By doing this, the path of the traffic will be clear and it will be easy to understand if the issue
is caused only by a subset of the nodes or when the pod lives on a different node, for example.

#### MetalLB and asymmetric return path

[Reverse path filtering](https://tldp.org/HOWTO/Adv-Routing-HOWTO/lartc.kernel.rpf.html) is a linux protection
mechanism which causes packets that should not be coming from an interface to be dropped (i.e., there is no route to the ip of
the client sending those packets through that interface).

This is common with BGP (and less so with L2 mode) because MetalLB will advertise routes to the service but the nodes
won't learn how to reach the client:

- The client from a different subnet tries to reach the service exposed by MetalLB
- The packets reach the node on an interface different from the default gateway
- The node does not have a route back to the client through that interface

Depending on the value of the `rp_filter` associated to the interface, the packets are dropped.

There are several ways to solve this issue:

- Add static routes to your node
- Use the [frr-k8s variant](https://metallb.io/concepts/bgp/index.html#frr-k8s-mode) to let your fabric send those
routes to the nodes
- Use source based routing (or something that leverages that) to steer the reply traffic towards the right interface.
This is the approach used by the [metallb node route agent](https://github.com/travisghansen/metallb-node-route-agent) (note that the project is not affiliated to metallb).

## Collecting information for a bug report

If after following the suggestions of this guide, a MetalLB bug is the primary suspect, you need to file a
bug report with some information.

In order to provide a meaningful bug report, the information on which phase of advertising a service of type
LoadBalancer is failing is greatly appreciated. Some good examples are:

- MetalLB is not replying to ARP requests
- The service IP is not advertised via BGP
- The service is being advertised but only from a subset of the nodes
- The service is advertised but I don't see the traffic with tcpdump

Some bad examples are:

- I can't curl the service
- MetalLB doesn't work

Additionally, the following info must be provided:

### The logs and the yaml output of all the CRs

Setting the loglevel to debug will allow MetalLB to provide more information.

Both the helm chart and the MetalLB operator provide a convenient way to set the loglevel to debug:

- The helm chart provides a `.controller.loglevel` and a `.speaker.loglevel` field
- The MetalLB Operator provides a loglevel field in the MetalLB CRD

When deploying manually via the raw manifests, the parameter on the speaker / controller container must
be set manually.

### How to fetch the logs and the CRs

A convenience script that fetches all the required information [can be found here](https://raw.githubusercontent.com/metallb/metallb/main/troubleshooting/collect.sh).

Additionally, the status of the service and of the endpoints must be provided:

```bash
kubectl get endpointslices <my-service> -o yaml
kubectl get svc <my_service> -o yaml
```

### How to debug the speaker and the controller containers

Due to the fact that both the speaker and the controller containers are based on a distroless image, an ephemeral container should be used to debug inside them: 

```bash
kubectl debug -it -n metallb-system -c <ephemeral container name> --target=speaker --image=<ephemeral image name> <speaker pod>
```
