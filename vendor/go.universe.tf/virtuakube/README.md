# Virtuakube

Virtuakube sets up virtual Kubernetes clusters for testing. It has
several advantages compared to minikube or cloud clusters:

 - Support any number of nodes, limited only by system RAM.
 - Can run without root privileges.
 - Can run without internet access.
 - Because it emulates a full ethernet LAN, can be used to test
   networked systems.

It's a very young system, and is being built for the needs of testing
[MetalLB](https://metallb.universe.tf) rather than as a general
purpose virtualized Kubernetes cluster. It's being published
independently of MetalLB in the hopes that it might be useful, but it
requires some effort to use.

Your host machine must have `qemu`, `qemu-img`, and `vde_switch`
installed. Additionally, you must provide the base disk image for the
VMs. Due to its size I cannot host it for free (if you can help with
that, please get in touch!), but you can build your own with `make
build-img`. The image weighs about 1.1G, and requires about double
that to build.

See `examples/simple-cluster` for an example of how to use the API.
