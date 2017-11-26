# MetalLB requirements

MetalLB requires the following to function:

- A [Kubernetes](https://kubernetes.io) cluster, running Kubernetes
  1.8.0 or later, that does not already have network load-balancing
  functionality.
- One or
  more [BGP](https://en.wikipedia.org/wiki/Border_Gateway_Protocol)
  capable routers that support 4-byte AS numbers
  ([RFC 6793](https://tools.ietf.org/html/rfc6793)).
- Some IPv4 addresses for MetalLB to hand out.

## Kubernetes

MetalLB is a Kubernetes addon, so you need a Kubernetes cluster to run
it.

MetalLB provides a load-balancer implementation to the cluster. Due to
Kubernetes's current design, it must be the only load-balancer
implementation running in the cluster. In particular, this means that
MetalLB is not compatible with most cloud Kubernetes solutions.

MetalLB is a good fit for bare metal (aka "on premises") clusters:
- [Kubeadm](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/)-managed clusters
- [Tectonic](https://coreos.com/tectonic/) on bare metal
- [Minikube](https://github.com/kubernetes/minikube) sandboxes

If you're using a cloud provider as a pure VM host, MetalLB _can_ be
made to work, but requires disabling the cloud platform
integrations. While there are some use cases that could benefit from
this, it is _not recommended_ unless you know what you're doing.
- [Google Compute Engine](https://kubernetes.io/docs/getting-started-guides/gce/)
- [Azure ACS-Engine](https://github.com/Azure/acs-engine/blob/master/docs/kubernetes.md)
- [Amazon Elastic Compute Cloud](https://kubernetes.io/docs/getting-started-guides/aws/)

Fully hosted cloud Kubernetes solutions to not allow disabling the cloud platform integrations, meaning MetalLB will _not_ work with:
- [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/)
- [Azure Container Service](https://azure.microsoft.com/en-us/services/container-service/)

## BGP router

MetalLB uses BGP to attract service traffic into the cluster, so you
need one or more BGP-capable routers outside the cluster for MetalLB
to talk to.

Before using MetalLB, we suggest that you become familiar with the
basics of BGP. There are many online tutorials and classes you can use
(if you know of a particularly good one, please file an issue and
we'll link it here!).

If you're running Kubernetes in a datacenter or colocation facility,
you likely already have a BGP-capable router that you can use to
integrate with MetalLB. If you're not sure, talk to your network
administrator or hosting provider.

You can also use a regular Linux machine as a BGP router, using
the [Quagga](http://www.nongnu.org/quagga/)
or [BIRD](http://bird.network.cz/) software BGP implementations.

## IPv4 addresses

MetalLB handles IPv4 address allocation within your cluster, but it
cannot materialize IP addresses out of thin air. You need to tell
MetalLB what IP addresses it can use, which means you need to have
some available IP address space.

For private clusters, you can simply allocate one or more ranges from
the RFC1918 private ranges, which are:
- 10.0.0.0/8
- 172.16.0.0/12
- 192.168.0.0/16

For public clusters, you'll need to purchase or lease some IPs from
a [LIR](https://en.wikipedia.org/wiki/Local_Internet_registry) (ISP,
hosting provider...). Again, if you're not sure, you should talk to
your network administrator or hosting provider about this.

In both cases, it is your responsibility to ensure that the address
space you provide to MetalLB is for its exclusive use. If you tell
MetalLB to advertise IP space that is in use by someone else, your
services won't work properly.
