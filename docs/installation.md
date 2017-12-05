# Installation

Installing MetalLB is very simple: just apply the manifest!

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml`

This will deploy MetalLB to your cluster, under the `metallb-system`
namespace. The components in the manifest are:
- The `metallb-system/controller` deployment. This is the cluster-wide
  controller that handles IP address assignments.
- The `metallb-system/bgp-speaker` daemonset. This is the component
  that peers with your BGP router(s) and announces assigned service
  IPs to the world.
- Service accounts for the controller and BGP speaker, along with the
  RBAC permissions that the components need to function.

The installation manifest does not include a configuration
file. MetalLB's components will still start, but will remain idle
until you define and deploy a configmap.

# Configuration

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
      cidr:
      - 192.168.10.0/24
      advertisements:
      - aggregation-length: 32
```

# Usage

Once MetalLB is installed and configured, to expose a service
externally, simply create it with `spec.type` set to `LoadBalancer`,
and MetalLB will do the rest.

MetalLB respects the `spec.loadBalancerIP` parameter, so if you want
your service to be set up with a specific address, you can request it
by setting that parameter. If MetalLB does not own the requested
address, or if the address is already in use by another service,
assignment will fail and MetalLB will log a warning event visible in
`kubectl describe service <your service name>`.

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

## Example

As an example of how to use these specific request options, consider
an ecommerce site that runs a production environment and multiple
developer sandboxes side by side. The production environment needs
public IP addresses, but the sandboxes can use private IP space,
routed to the developer offices through a VPN.

Additionally, because the production IPs end up hardcoded in various
places (DNS, security scans for regulatory compliance...), we want
specific services to have specific addresses in production. On the
other hand, sandboxes come and go as developers bring up and tear down
environments, so we don't want to manage assignments by hand.

We can translate these requirements into MetalLB fairly
directly. First, we define two address pools, and set BGP attriibutes
to control the visibility of each set of addresses:

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
  cidr:
  - 42.176.25.64/30
  advertisements:
  - aggregation-length: 32

- # On the other hand, the sandbox environment uses private IP space,
  # which is free and plentiful. We give this address pool a ton of IPs,
  # so that developers can spin up as many sandboxes as they need.
  name: sandbox
  cidr:
  - 192.168.144.0/20
  advertisements:
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
