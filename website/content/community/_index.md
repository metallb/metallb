---
title: Community & Contributing
weight: 7
---

We would love to hear from you! Here are some places you can find us.

## Mailing list

Our mailing list is
[metallb-users@googlegroups.com](https://groups.google.com/forum/#!forum/metallb-users). It's
for discussions around MetalLB usage, community support, and developer
discussion (although for the latter we mostly use GitHub directly).

## Slack

For a more interactive experience, we have the [#metallb slack channel
on k8s.slack.com](https://kubernetes.slack.com/messages/metallb/). If
you're not already logged into the Kubernetes slack organization,
you'll need to [request an invite](http://slack.k8s.io/) before you
can join.

Development of MetalLB is discussed in the [#metallb-dev slack channel
](https://kubernetes.slack.com/messages/metallb-dev/).

## Issue Tracker

Use the [GitHub issue
tracker](https://github.com/metallb/metallb/issues) to file bugs and
features request. If you need support, please send your questions to
the metallb-users mailing list rather than filing a GitHub issue.

# Contributing

We welcome contributions to MetalLB! Here's some information to get
you started.

## Code of Conduct

This project is released with a [Contributor Code of Conduct]({{%
relref "code-of-conduct.md" %}}). By participating in this project you
agree to abide by its terms.

## Code changes

Before you make significant code changes, please consider opening a pull
request with a proposed design in the `design/` directory. That should
reduce the amount of time required for code review. If you don't have a full
design proposal ready, feel free to open an issue to discuss what you would
like to do.

All submissions require review. We use GitHub pull requests for this
purpose. Consult [GitHub
Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](https://github.com/metallb/metallb/blob/main/DCO)
file for details.

## Code organization

MetalLB's code is divided between a number of binaries, and some
supporting libraries. The libraries live in the `internal` directory,
and each binary has its own top-level directory. Here's what we
currently have, relative to the top-level directory:

- `controller` is the cluster-wide MetalLB controller, in charge of
  IP assignment.
- `speaker` is the per-node daemon that advertises services with
  assigned IPs using various advertising strategies.
- `internal/k8s` contains the bowels of the logic to talk to the
  Kubernetes apiserver to get and modify service information. It
  allows most of the rest of the MetalLB code to be ignorant of the
  Kubernetes client library, other than the objects (Service,
  ConfigMap...) that they manipulate.
- `internal/config` parses and validates the MetalLB configmap.
- `internal/allocator` is the IP address manager. Given pools from the
  MetalLB configmap, it can allocate addresses on demand.
- `internal/bgp/native` is a _very_ stripped down implementation of BGP. It
  speaks just enough of the protocol to keep peering sessions up, and
  to push routes to the peer.
- `internal/bgp/frr` contains the code for translating the MetalLB configuration
   to the FRR configuration that is applied to the FRR container, when running
   MetalLB in FRR mode.
- `internal/layer2` is an implementation of an ARP and NDP responder.
- `internal/logging` is a logging shim that redirects both
  Kubernetes's `klog` and Go's standard library `log` output to
  go-kit's structured logger, which is what MetalLB itself uses for
  logging.
- `internal/version` just burns version numbers and git commit
  information into compiled binaries, so that MetalLB can print its
  build information.

In addition to code, there's deployment configuration and
documentation:

- `config/manifests` contains a variety of Kubernetes manifests. The most
  important one is `config/manifests/metallb-native.yaml`, which specifies how to
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
- [kind](https://github.com/kubernetes-sigs/kind), a lightweight Kubernetes cluster running in Docker
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/), the Kubernetes commandline interface
- [Invoke](https://www.pyinvoke.org) to drive the build system

>NOTE: The development environment was tested with **kind `v0.9.0`**. Older
>versions may not work since there have been breaking changes between minor
>versions.

## Building and running the code

Start by fetching the MetalLB repository, with `git clone
https://github.com/metallb/metallb`.

From there, you can use Invoke to build Docker images, push them to
registries, and so forth. `inv -l` lists the available tasks.

To build and deploy MetalLB to a local development environment using a kind
cluster, run `inv dev-env`.

When you're developing, running components at the command line and
having them attach to a cluster might be more convenient than
redeploying them to a cluster over and over.

For the controller, the `-kubeconfig` and `-namespace` command-line flags
are needed. Speakers need those and `-node-name`.

For example:

```bash
metallb$ go run ./controller/main.go ./controller/service.go -namespace metallb-system -kubeconfig $KUBECONFIG

metallb$ go run ./speaker/main.go ./speaker/*controller.go -namespace metallb-system -kubeconfig $KUBECONFIG -node-name node0
```

For development, fork
the [github repository](https://github.com/metallb/metallb), and add
your fork as a remote in `$GOPATH/src/go.universe.tf/metallb`, with
`git remote add fork git@github.com:<your-github-user>/metallb.git`.

## Commit Messages

The following are our commit message guidelines:

- Line wrap the body at 72 characters
- For a more complete discussion of good git commit message practices, see
  <https://chris.beams.io/posts/git-commit/>.

## Extending the end to end test suite

When adding a new feature, or modifying a current one, consider adding a new test
to the test suite located in `/e2etest`.
Each feature should come with enough unit test / end to end coverage to make
us confident of the change.

## The website

The website at <https://metallb.universe.tf> is pinned to the latest
released version, so that users who don't care about ongoing
development see documentation that is consistent with the released
code.

However, there is a version of the website synced to the latest main
branch
at
[https://main--metallb.netlify.com](https://main--metallb.netlify.com). Similarly,
every branch has a published website at `<branch
name>--metallb.netlify.com`. So if you want to view the documentation
for the 0.2 version, regardless of what the currently released version
is, you can
visit
[https://v0.2--metallb.netlify.com](https://v0.2--metallb.netlify.com).

When editing the website, you can preview your changes locally by
installing [Hugo](https://gohugo.io/) and running `hugo server` from
the `website` directory.

## Maintainers

For information about the current maintainers of MetalLB, see the [maintainers
page]({{% relref "maintainers.md" %}}).
