---
title: Community
weight: 5
---

We would love to hear from you! Here are some places you can find us.

## Mailing list

Our mailing list
is
[metallb-users@googlegroups.com](https://groups.google.com/forum/#!forum/metallb-users). It's
for discussions around MetalLB usage, community support, and developer
discussion (although for the latter we mostly use Github directly).

## Slack

_Under construction! We're waiting for the Kubernetes Slack admins to
create the channel. Please use other communication methods for now._

<!-- For a more interactive experience, we have
the
[#metallb slack channel on k8s.slack.com](https://kubernetes.slack.com/messages/metallb/). If
you're not already logged into the Kubernetes slack organization,
you'll need to [request an invite](http://slack.k8s.io/) before you
can join. -->


## IRC

If you prefer a more classic chat experience, we're also on `#metallb`
on the Freenode IRC network. You can use
Freenode's
[web client](http://webchat.freenode.net?randomnick=1&channels=%23metallb&uio=d4) if
you don't already have an IRC client.

## Issue Tracker

Use
the [GitHub issue tracker](https://github.com/google/metallb/issues)
to file bugs and features request. If you need support, please send
your questions to the metallb-users mailing list rather than filing a
GitHub issue.

# Contributing

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
- `speaker` is the per-node daemon that advertises services with
  assigned IPs using various advertising strategies.
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

- `manifests` contains a variety of Kubernetes manifests. The most
  important one is `manifests/metallb.yaml`, which specifies how to
  deploy MetalLB onto a cluster.
- `website` contains the website for MetalLB. The `website/content`
  subdirectory is where all the pages live, in Markdown format.

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

Optionally, if you want to update the vendored dependency, you'll
need [glide](https://github.com/Masterminds/glide), the Go dependency
manager

## Building the code

Start by fetching the MetalLB repository, with `go get
go.universe.tf/metallb`.

From there, you can use normal Go commands to build binaries and run
unit tests, e.g. `go install go.universe.tf/metallb/bgp-speaker`, `go
test ./internal/allocator`.

For development, fork
the [github repository](https://github.com/google/metallb), and add
your fork as a remote in `$GOPATH/src/go.universe.tf/metallb`, with
`git remote add fork git@github.com:<your-github-user>/metallb.git`.

**Note**: the repository must be checked out at
`$GOPATH/src/go.universe.tf/metallb`. The source code uses canonical
import paths, so if you check it out at
`$GOPATH/src/github.com/google/metallb` or similar, it will fail to
compile.

## Testing in Minikube

To really test MetalLB fully, you need to run it in a Kubernetes
cluster, to verify that all the pieces are working together. The
repository has a set of Fabric commands that makes this easy, by
setting up a Minikube sandbox and deploying a production MetalLB
setup, but running your locally built binaries.

### Sandbox setup

Start by running `make start-minikube`. This will:

- Create the Minikube sandbox in a local VM
- Enable the registry addon, so that we can host container images in the sandbox
- Deploy `test-bgp-router`, which sets up BIRD, Quagga and GoBGP routers as a
  pod inside the cluster
- Deploy MetalLB, which will install the `controller` and `speaker`
- Push a MetalLB configuration that connects MetalLB to the `test-bgp-router`

At this point, your sandbox is running the precompiled version of
MetalLB, pulled from [Docker Hub](https://hub.docker.com/u/metallb/).

You can inspect the state of the `test-bgp-router` by running
`minikube service test-bgp-router-ui`, which will open a browser tab
that shows you the current BGP connections and routing state, as seen
by the test routers.

### Pushing test binaries

When you're ready to test a local change you've made to MetalLB, you
can build and deploy MetalLB containers to your sandbox. First, if
you're using minikube, leave `make proxy-to-registry` running in a
second terminal. This will make the cluster's internal registry
available on `localhost`, so that we can push to it.

To deploy your changes, run `make push`. This will:

- Build all MetalLB binaries (`controller`, `speaker`, and `test-bgp-router`)
- Build ephemeral container images with those binaries inside
- Push the ephemeral containers to Minikube's internal container registry
- Update the MetalLB deployments and daemonsets to use the ephemeral containers
- Wait for all the pieces of MetalLB to update

Once the push is done, MetalLB will still be running in your Minikube
sandbox, but using binaries built from your local source code instead
of the public images.

*Note for MacOS users:* Since Docker is run inside a virtual machine
in MacOS the local registry won't work out of the box. To make it work
you have to add `docker.for.mac.localhost:5000` under **Insecure
registries** in your Docker daemon preferences. Once you've done that,
`make push` should work.

[![Docker for Mac config](/images/dockerformacconfig.png)](/images/dockerformacconfig.png)

If you need to get back to a working configuration, `make
push-manifests` will revert MetalLB to running from the public Docker
Hub images and the config from the repository.

### Sandbox teardown

When you're done with minikube, run `minikube delete` to destroy the
sandbox.

### Existing users of Minikube

If you're already using minikube, be warned: `make start-minikube`
will touch the default minikube sandbox, and so may interfere with
other experiments you have going on.

## Testing outside of Minikube

You can also use `make push` on clusters other than minikube. `make
push` will deploy to whichever cluster your `kubectl` is currently
pointing to.

If your cluster has a local registry, usage instructions are exactly
the same as with minikube: leave `make proxy-to-registry` running in a
secondary terminal, and then `make push` each time you want to test
your changes.

If you want to use an external registry, you can specify it with the
`REGISTRY` make variable. For example, `make push REGISTRY=danderson`
will push the docker images
to
[danderson's account on docker hub](https://hub.docker.com/u/danderson/),
and make the cluster pull from there as well.

## Cross compiling

Released versions of MetalLB (0.3.0 and later) use multi-architecture
images, and so should work on all platforms supported by
kubernetes. However, the dev builds made by `make push` only build for
one architecture, to save time.

By default, `make push` builds binaries for amd64 (aka x86_64). If you
want to test on a different architecture (for example a raspberry pi
cluster), you can select the architecture of the dev builds by setting
the `ARCH` make variable to your desired architecture, one of amd64,
arm, arm64, ppc64le, s390x. For example, `make push ARCH=arm` will
build and deploy containers that work on ARM machines.

## Build customizations

You can write custom make configuration options to
`Makefile.defaults`, and they will be included as defaults for all
builds. For example, if you normally build with go1.10beta1 and push
arm64 binaries to a custom registry, you can use the following
Makefile.defaults:

```make
GOCMD=go1.10beta1
ARCH=arm64
REGISTRY=my-cool-images
```

To see a list of customizable options and what they do, look at the
top of `Makefile`.

## Peering with real BGP routers

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

## The website

The website at https://metallb.universe.tf is pinned to the latest
released version, so that users who don't care about ongoing
development see documentation that is consistent with the released
code.

However, there is a version of the website synced to the latest master
branch
at
[https://master--metallb.netlify.com](https://master--metallb.netlify.com). Similarly,
every branch has a published website at `<branch
name>--metallb.netlify.com`. So if you want to view the documentation
for the 0.2 version, regardless of what the currently released version
is, you can
visit
[https://v0.2--metallb.netlify.com](https://v0.2--metallb.netlify.com).

When editing the website, you can preview your changes locally by
installing [Hugo](https://gohugo.io/) and running `hugo server` from
the `website` directory.
