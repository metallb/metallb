---
title: Configuration
weight: 4
---

MetalLB remains idle until configured. This is accomplished by
creating and deploying a config map into the same namespace
(metallb-system) as the deployment.

There is an example config map in
[`manifests/example-config.yaml`](https://raw.githubusercontent.com/metallb/metallb/main/manifests/example-config.yaml),
annotated with explanatory comments.

If you've named the config map `config.yaml`, you can deploy the manifest with `kubectl apply -f config.yaml`.

{{% notice note %}}
If you installed MetalLB with Helm, you will need to change the
namespace of the config map to match the namespace in which MetalLB was
deployed, and change the name of the config map from `config` to
`metallb`.
{{% /notice %}}

The specific configuration depends on the protocol(s) you want to use
to announce service IPs. Jump to:

- [Layer 2 configuration](#layer-2-configuration)
- [BGP configuration](#bgp-configuration)
- [Advanced configuration](#advanced-address-pool-configuration)

## Layer 2 configuration

Layer 2 mode is the simplest to configure: in many cases, you don't
need any protocol-specific configuration, only IP addresses.

Layer 2 mode does not require the IPs to be bound to the network interfaces
of your worker nodes. It works by responding to ARP requests on your local
network directly, to give the machine's MAC address to clients.

For example, the following configuration gives MetalLB control over
IPs from `192.168.1.240` to `192.168.1.250`, and configures Layer 2
mode:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - 192.168.1.240-192.168.1.250
```

{{% notice note %}}
If you installed MetalLB with the `MetalLB Operator`, the addresspool configuration happens via the `AddressPool` CRD:

```yaml
apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  name: addresspool-sample1
  namespace: metallb-system
spec:
  protocol: layer2
  addresses:
    - 172.18.0.100-172.18.0.255
```

{{% /notice %}}

## BGP configuration

For a basic configuration featuring one BGP router and one IP address
range, you need 4 pieces of information:

- The router IP address that MetalLB should connect to,
- The router's AS number,
- The AS number MetalLB should use,
- An IP address range expressed as a CIDR prefix.

As an example, if you want to give MetalLB the range 192.168.10.0/24
and AS number 64500, and connect it to a router at 10.0.0.1 with AS
number 64501, your configuration will look like:

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
      peer-asn: 64501
      my-asn: 64500
    address-pools:
    - name: default
      protocol: bgp
      addresses:
      - 192.168.10.0/24
```

{{% notice note %}}
If you installed MetalLB with the `MetalLB Operator`, the peers configuration happens via the `BGPPeer` CRD:

```yaml
apiVersion: metallb.io/v1beta1
kind: BGPPeer
metadata:
  name: peer-sample1
  namespace: metallb-system
spec:
  peerAddress: 10.0.0.1
  peerASN: 64501
  myASN: 64500
  routerID: 10.10.10.10
  peerPort: 1
  holdTime: "180s"
  keepaliveTime: "180s"
  sourceAddress: "1.1.1.1"
  password: "test"
  nodeSelectors:
  - matchExpressions:
    - key: kubernetes.io/hostname
      operator: In
      values: [hostA, hostB]
```

{{% /notice %}}

### Enabling BFD support for BGP sessions

With the experimental FRR mode, BGP sessions can be backed up by BFD sessions in order to provide a quicker path failure detection than BGP alone provides.

In order to enable BFD, a BFD profile must be added and referenced by a given peer:

```yaml
    peers:
    - my-asn: 64512
      peer-asn: 64513
      peer-address: 172.18.0.7
      peer-port: 179
      bfd-profile: full1
    bfd-profiles:
    - name: full1
      receive-interval: 179
      transmit-interval: 180
      echo-interval: 62
      echo-mode: false
      passive-mode: false
      minimum-ttl: 254

```

### Advertisement configuration

By default, BGP mode advertises each allocated IP to the configured
peers with no additional BGP attributes. The peer router(s) will
receive one `/32` route for each service IP, with the BGP localpref
set to zero and no BGP communities.

You can configure more elaborate advertisements by adding a
`bgp-advertisements` section that lists one or more custom
advertisements.

In addition to specifying localpref and communities, you can use this
to advertise aggregate routes. The `aggregation-length` advertisement
option lets you "roll up" the /32s into a larger prefix. Combined with
multiple advertisement configurations, this lets you create elaborate
advertisements that interoperate with the rest of your BGP network.

For example, let's say you have a leased `/24` of public IP space, and
you've allocated it to MetalLB. By default, MetalLB will advertise
each IP as a /32, but your transit provider rejects routes more
specific than `/24`. So, you need to somehow advertise a `/24` to your
transit provider, but still have the ability to do per-IP routing
internally.

Here's a configuration that implements this:

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
      peer-asn: 64501
      my-asn: 64500
    address-pools:
    - name: default
      protocol: bgp
      addresses:
      - 198.51.100.0/24
      bgp-advertisements:
      - aggregation-length: 32
        localpref: 100
        communities:
        - no-advertise
      - aggregation-length: 24
    bgp-communities:
      no-advertise: 65535:65282
```

With this configuration, if we create a service with IP 198.51.100.10,
the BGP peer(s) will receive two routes:

- `198.51.100.10/32`, with localpref=100 and the `no-advertise`
  community, which tells the peer router(s) that they can use this
  route, but they shouldn't tell anyone else about it.
- `198.51.100.0/24`, with no custom attributes.

With this configuration, the peer(s) will propagate the
`198.51.100.0/24` route to your transit provider, but once traffic
shows up locally, the `198.51.100.10/32` route will be used to forward
into your cluster.

As you define more services, the router will receive one "local" `/32`
for each of them, as well as the covering `/24`. Each service you
define "generates" the `/24` route, but MetalLB deduplicates them all
down to one BGP advertisement before talking to its peers.

The above configuration also showcases the `bgp-communities`
configuration section, which lets you define readable names for BGP
communities that you can reuse in your advertisement
configurations. This is completely optional, you could just specify
`65535:65281` directly in the configuration of the `/24` if you
prefer.

### Limiting peers to certain nodes

By default, every node in the cluster connects to all the peers listed
in the configuration. In more advanced cluster topologies, you may
want each node to connect to different routers. For example, if you
have a "rack and spine" network topology, you likely want each machine
to peer with its top-of-rack router, but not the routers in other
racks.

{{<mermaid align="center">}}
graph BT
    subgraph ""
      metallbA("MetalLB<br>Speaker")
    end
    subgraph ""
      metallbB("MetalLB<br>Speaker")
    end

    subgraph ""
      metallbC("MetalLB<br>Speaker")
    end
    subgraph ""
      metallbD("MetalLB<br>Speaker")
    end

    metallbA-->torA(ToR Router)
    metallbB-->torA(ToR Router)
    metallbC-->torB(ToR Router)
    metallbD-->torB(ToR Router)

    torA-->spine(Spine Router)
    torB-->spine(Spine Router)
{{< /mermaid >}}

You can limit peers to certain nodes by using the `node-selectors`
attribute of peers in the configuration. The semantics of these
selectors are the same as those used elsewhere in Kubernetes, so refer
to
the
[labels documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) on
the Kubernetes website.

For example, this is a (somewhat contrived) definition for a peer that
will only be used by machines:

- With hostname `hostA` or `hostB`, or
- That have the `rack=frontend` label, but _not_ the label `network-speed=slow`:

```yaml
peers:
- peer-address: 10.0.0.1
  peer-asn: 64501
  my-asn: 64500
  node-selectors:
  - match-labels:
      rack: frontend
    match-expressions:
    - key: network-speed
      operator: NotIn
      values: [slow]
  - match-expressions:
    - key: kubernetes.io/hostname
      operator: In
      values: [hostA, hostB]
```

### Configuring the BGP source address

When a host has multiple network interfaces or multiple IP addresses
configured on one interface, the host's TCP/IP stack usually selects
the IP address that is used as the source IP address for outbound
connections automatically. This is true also for BGP connections.

Sometimes, the automatically-selected address may not be the desired
one for some reason. In such cases, MetalLB supports explicitly
specifying the source address to be used when establishing a BGP
session:

```yaml
peers:
- peer-address: 10.0.0.1
  peer-asn: 64501
  my-asn: 64500
  source-address: 10.0.0.2
  node-selectors:
  - match-labels:
      kubernetes.io/hostname: node-1
```

The configuration above tells the MetalLB speaker to check if the
address `10.0.0.2` exists locally on one of the host's network
interfaces, and if so - to use it as the source address when
establishing BGP sessions. If the address isn't found, the default
behavior takes place (that is, the kernel selects the source address
automatically).

{{% notice warning %}}
In most cases the `source-address` field should only be used with
**per-node peers**, i.e. peers with node selectors which select only
one node.

By default, a BGP peer configured under the `peers` configuration
section runs on **all** speaker nodes. It is likely meaningless to use
the `source-address` field in a peer configuration that applies to
more than one node because two nodes in a given network usually
shouldn't have the same IP address.
{{% /notice %}}

## Advanced address pool configuration

### Controlling automatic address allocation

In some environments, you'll have some large address pools of "cheap"
IPs (e.g. RFC1918), and some smaller pools of "expensive" IPs
(e.g. leased public IPv4 addresses).

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
  addresses:
  - 192.168.144.0/20
- name: expensive
  protocol: bgp
  addresses:
  - 42.176.25.64/30
  auto-assign: false
```

Addresses can still be specifically allocated from the "expensive"
pool with the methods described in
the [usage](/usage/#requesting-specific-ips) section.

{{% notice note %}}
To specify a single IP address in a pool, use `/32` in the CIDR notation
(e.g. `42.176.25.64/32`).
{{% /notice %}}

### Handling buggy networks

Some old consumer network equipment mistakenly blocks IP addresses
ending in `.0` and `.255`, because of
misguided
[smurf protection](https://en.wikipedia.org/wiki/Smurf_attack).

If you encounter this issue with your users or networks, you can set
`avoid-buggy-ips: true` on an address pool to mark `.0` and `.255`
addresses as unusable.

## Configuring via the MetalLB Operator

The MetalLB Operator allows to configure MetalLB via CRDs.

The CRDs are mapped to the current MetalLB ConfigMap structure.
