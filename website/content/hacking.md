---
title: Hacking and Contributing
weight: 50
---

We welcome contributions to MetalLB! Here's some information to get
you started.

## Code of Conduct

This project is released with
a [Contributor Code of Conduct]({{% relref "code-of-conduct.md" %}}). By
participating in this project you agree to abide by its terms.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement. You (or your employer) retain the copyright to your contribution,
this simply gives us permission to use and redistribute your contributions as
part of the project. Head over to <https://cla.developers.google.com/> to see
your current agreements on file or to sign a new one.

You generally only need to submit a CLA once, so if you've already
submitted one (even if it was for a different project), you probably
don't need to do it again. When you submit pull requests, a helpful
Google CLA bot will tell you if you need to sign the CLA.

## Code changes

Before you make significant code changes, please open an issue to
discuss your plans. This will minimize the amount of review required
for pull requests.

All submissions require review. We use GitHub pull requests for this
purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/)
for more information on using pull requests.

## Code organization

MetalLB's code is divided between a number of binaries, and some
supporting libraries. The libraries live in the `internal` directory,
and each binary has its own top-level directory. Here's what we
currently have, relative to the top-level directory:

- `controller` is the cluster-wide MetalLB controller, in charge of
  IP assignment.
- `bgp-speaker` is the per-node daemon that advertises services with
  assigned IPs to configured BGP peers.
- `test-bgp-router` is a small wrapper around
  the
  [BIRD](http://bird.network.cz),
  [Quagga](http://www.nongnu.org/quagga)
  and [GoBGP](https://github.com/osrg/gobgp) open-source BGP routers
  that presents a read-only interface over HTTP. We use it in the
  tutorial, and during development of MetalLB.
- `internal/k8s` contains the bowels of the logic to talk to the
  Kubernetes apiserver to get and modify service information. It
  allows most of the rest of the MetalLB code to be ignorant of the
  Kubernetes client library, other than the objects (Service,
  ConfigMap...) that they manipulate.
- `internal/config` parses and validates the MetalLB configmap.
- `internal/allocator` is the IP address manager. Given pools from the
  MetalLB configmap, it can allocate addresses on demand.
- `internal/bgp` is a _very_ stripped down implementation of BGP. It
  speaks just enough of the protocol to keep peering sessions up, and
  to push routes to the peer.

In addition to code, there's deployment configuration and
documentation:

- `docs` contains various documents, such as a tutorial, an
  installation guide, and this hacking guide.
- `dockerfiles` contains the Docker build configurations that package
  MetalLB into container images. It contains one set of "prod"
  configurations, which is what users of MetalLB install, and one set
  of "dev" configurations which get used during development (more on
  that below).
- `manifests` contains a variety of Kubernetes manifests. The most
  important one is `manifests/metallb.yaml`, which specifies how to
  deploy MetalLB onto a cluster.

## Required software

To develop MetalLB, you'll need a couple of pieces of software:

- [git](https://git-scm.com), the version control system
- The [Go](https://golang.org) programming language (notably the `go`
  tool)
- [Docker](https://www.docker.com/docker-community), the container
  running system
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), the Kubernetes commandline interface
- [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/),
  the Kubernetes sandbox manager (version 0.24 or later)
- [Fabric](http://www.fabfile.org/), the devops scripting toolkit

Optionally, if you want to update the vendored dependency, you'll
need [glide](https://github.com/Masterminds/glide), the Go dependency
manager

## Building the code

Start by cloning the MetalLB repository, with `git clone
https://github.com/google/metallb`.

From there, you can use normal Go commands to build binaries and run
unit tests, e.g. `go install go.universe.tf/metallb/bgp-speaker`, `go
test ./internal/allocator`.

## Testing in Minikube

To really test MetalLB fully, you need to run it in a Kubernetes
cluster, to verify that all the pieces are working together. The
repository has a set of Fabric commands that makes this easy, by
setting up a Minikube sandbox and deploying a production MetalLB
setup, but running your locally built binaries.

### Sandbox setup

Start by running `fab start`. This will:

- Create the Minikube sandbox in a local VM
- Enable the registry addon, so that we can host container images in the sandbox
- Deploy `test-bgp-router`, which sets up BIRD, Quagga and GoBGP routers as a
  pod inside the cluster
- Deploy MetalLB, which will install the `controller` and `bgp-speaker`
- Push a MetalLB configuration that connects MetalLB to the `test-bgp-router`

At this point, your sandbox is running the precompiled version of
MetalLB, pulled from [quay.io](https://quay.io/metallb).

You can inspect the state of the `test-bgp-router` by running
`minikube service test-bgp-router-ui`, which will open a browser tab
that shows you the current BGP connections and routing state, as seen
by the test routers.

### Pushing test binaries

When you're ready to test a local change you've made to MetalLB, run
`fab push`. This will:

- Build all MetalLB binaries (`controller`, `bgp-speaker`, and `test-bgp-router`)
- Build ephemeral container images with those binaries inside
- Push the ephemeral containers to Minikube's internal container registry
- Update the MetalLB deployments and daemonsets to use the ephemeral containers
- Wait for all the pieces of MetalLB to update

Once the push is done, MetalLB will still be running in your Minikube
sandbox, but using binaries built from your local source code instead
of the public images.

*Note for MacOS users:* Since Docker is run inside a virtual machine
in MacOS the local registry won't work out of the box and so won't
```fab push```. Instead it is necessary to add
```docker.for.mac.localhost:5000``` under **Insecure registries** in
your Docker daemon preferences and run ```fab
push:registry=docker.for.mac.localhost:5000```

[![Docker for Mac config](/images/dockerformacconfig.png)](/images/dockerformacconfig.png)

Note that if you push a binary that crash-loops in Kubernetes, the
final waiting stage may never complete, because Fabric is waiting for
a rollout that will never succeed. If that happens, it's safe to
interrupt Fabric and then use `kubectl` to troubleshoot the issue.

If you need to get back to a working configuration, `fab
push_manifests` will revert MetalLB to running from the public quay.io
images. Likewise, `fab push_config` will revert any config changes you
made and go back to the minimalist configuration that `fab start`
installed.

### Sandbox teardown

When you're done with minikube, run `fab stop` to destroy the sandbox.

### Existing users of Minikube

If you're already using minikube, be warned: `fab start` and `fab
stop` will touch the default minikube sandbox. So for example, if you
run `fab stop`, the default Minikube instance will be destroyed, and
take any other things you had in there with it.

### Testing in bigger clusters

If you've outgrown Minikube, you can also use `fab push` against
"real" clusters. The `fab push` command will deploy MetalLB with
custom binaries to whichever cluster `kubectl` is currently pointing
to. If you want to use a cluster other than minikube, select it with
`kubectl config use-context <context name>` before you run `fab push`.

### Peering with real BGP routers

While testing, it might be useful to peer with "real" routers outside
of the cluster, rather than always use the in-cluster
`test-bgp-router`. If you do so, you *need to reconfigure the address
pool from the default config*! The default configuration uses the
`TEST-NET-2` IP range
from [RFC5735](https://tools.ietf.org/html/rfc5735), which is reserved
for use in documentation and example code. It's fine to use it with
our `test-bgp-router`, since they doesn't propagate the addresses
beyond themselves, but if you try injecting those addresses into a
real network, you may run into trouble.
