---
title: Cloud Compatibility
---

**In general, MetalLB is not compatible with cloud providers.**

MetalLB is for bare-metal clusters, and even cloud providers that
offer "dedicated servers" usually don't support the network protocols
that MetalLB requires.

This is an incomplete list of cloud providers and platforms. If your
platform isn't listed here, its support status is unknown, but it's
very likely that the answer is "no". If you know for sure, please send
a pull request to update this list!

| Cloud platform | Supported                                                                                    |
|----------------|----------------------------------------------------------------------------------------------|
| AWS            | No, use EKS                                                                                  |
| Azure          | No, use AKS                                                                                  |
| DigitalOcean   | No, use DigitalOcean Kubernetes                                                              |
| Equinix Metal  | Yes, see [Equinix Metal notes]                                                               |
| Google Cloud   | No, use GKE                                                                                  |
| Hetzner        | Yes, see [Hetzner notes](https://community.hetzner.com/tutorials/install-kubernetes-cluster) |
| OVH            | Yes, when using a vRack                                                                      |
| OpenShift OCP  | Yes, see [OpenShift notes]                                                                   |
| OpenStack      | Yes, see [OpenStack notes]                                                                   |
| Proxmox        | Yes                                                                                          |
| VMware         | Yes                                                                                          |
| Vultr          | Yes                                                                                          |

[use alternatives]: #alternatives
[OpenShift notes]: #metallb-on-openshift-ocp
[OpenStack notes]: #metallb-on-openstack
[Equinix Metal notes]: #metallb-on-equinix-metal

## Why doesn't MetalLB work on (most) cloud platforms?

MetalLB implements load balancers using standard routing
protocols. However, in general, cloud platforms don't implement those
routing protocols in a way that MetalLB can leverage.

In particular:

- BGP support is uncommon, and is usually designed to support inbound
  route advertisement from off-cloud locations (e.g. Google Cloud
  Router). MetalLB's BGP mode therefore either doesn't work at all, or
  doesn't do what most people want - and definitely does it worse than
  the cloud platform's own load-balancer products.
- ARP is emulated by the virtual network layer, meaning that only IPs
  assigned to VMs by the cloud platform can be resolved. This breaks
  MetalLB's L2 mode, which relies on ARP's behavior on normal Ethernet
  networks.
- Even in cases where ARP works normally, the IP allocation process
  for public IPs (often called "floating IPs") doesn't integrate with
  the virtual network in a standard way, so MetalLB can't "grab" the
  IP with ARP and route it to the correct place. Instead, you have to
  talk to the cloud's proprietary API to reroute the floating IP,
  which MetalLB can't do.

The short version is: cloud providers expose proprietary APIs instead
of standard protocols to control their network layer, and MetalLB
doesn't work with those APIs.

## Platform-Specific Notes

### MetalLB on OpenShift OCP

To run MetalLB on OpenShift, two changes are required: changing the
pod UIDs, and granting MetalLB additional networking privileges.

Pods get UIDs automatically assigned based on an OpenShift-managed UID
range, so you have to remove the hardcoded unprivileged UID from the
MetalLB manifests. You can do this by removing the
`spec.template.spec.securityContext.runAsUser` field from both the
`controller` Deployment and the `speaker` DaemonSet.
Also, as the allowed group ID range in Openshift is 5000 through 5999,
you have to remove `spec.template.spec.securityContext.fsGroup` field
as well.

Additionally, you have to grant the `speaker` DaemonSet elevated
privileges, so that it can do the raw networking required to make
load balancers work. You can do this with:

```bash
oc adm policy add-scc-to-user privileged -n metallb-system -z speaker
```

After that, MetalLB should work normally.

### MetalLB on OpenStack

You can run a Kubernetes cluster on OpenStack VMs, and use MetalLB as
the load balancer. However you have to disable OpenStack's ARP
spoofing protection if you want to use L2 mode. You must disable it on
all the VMs that are running Kubernetes.

By design, MetalLB's L2 mode looks like an ARP spoofing attempt to
OpenStack, because we're announcing IP addresses that OpenStack
doesn't know about. There's currently no way to make OpenStack
cooperate with MetalLB here, so we have to turn off the spoofing
protection entirely.

### MetalLB on Equinix Metal

[Equinix Metal](https://deploy.equinix.com) is an unusually "bare metal" cloud
platform, and supports using BGP to advertise and route floating IPs to
machines. As such, MetalLB's BGP mode works great on Equinix Metal! There is
even a [tutorial](https://github.com/equinix-labs/terraform-metal-kubernetes-bgp) written by the
folks at Equinix Metal, that use MetalLB to integrate Kubernetes load balancers
with their BGP infrastructure.

## Alternatives

If MetalLB doesn't work with your cloud platform, you have two main
alternatives.

### Use the platform's load balancer

If your cloud platform has a load-balancer product, you should use
that. It's probably going to be more featureful and higher performance
than MetalLB anyway, and probably has a Kubernetes integration that's
maintained by the cloud provider.

### Use keepalived-vip

keepalived-vip is a simple wrapper around keepalived, which some
people have successfully used to configure virtual IPs with
Kubernetes. The key feature that makes this work is that keepalived
supports shell script hooks when a failover event occurs, so you can
write a custom shell script that talks to your cloud platform's APIs
and do the right thing.

Note that keepalived-vip *by itself* still won't work
properly. Keepalived implements VRRP, which is roughly equivalent to
MetalLB's L2 mode. If MetalLB's L2 mode doesn't work, Keepalived's
VRRP won't either... *but* with the hook shell scripts, you can write
the glue code for the cloud API.

The resulting system is less well integrated with Kubernetes
(LoadBalancer Service objects still won't work), and mostly only makes
sense coupled with an HTTP(S) ingress controller. It's also not very
widely used, so documentation isn't great. That said, a search for
"<your cloud provider> keepalived-vip" should hopefully get you some
useful information to implement this method.

## Can MetalLB support my cloud provider?

In theory, it would be possible for MetalLB to support cloud provider
APIs and provide the same functionality as with standard network
protocols on bare metal.

This is currently out of scope for MetalLB, for one primary reason:
MetalLB has no funding to pay for the cloud resources to test these
integrations. If the cloud providers, or some other sponsor, is
willing to pay for the resources (servers, IPs, ...) required to test
the integration, then we could *potentially* add support to MetalLB.

If you think you can help with getting resources for testing, [file a
bug](https://github.com/metallb/metallb/issues/new) and we can talk
about it!
