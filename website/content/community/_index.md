---
title: Community & Contributing
weight: 7
---

We would love to hear from you! Here are some places you can find us.

## Mailing list

Our mailing list
is
[metallb-users@googlegroups.com](https://groups.google.com/forum/#!forum/metallb-users). It's
for discussions around MetalLB usage, community support, and developer
discussion (although for the latter we mostly use Github directly).

## Slack

For a more interactive experience, we have
the
[#metallb slack channel on k8s.slack.com](https://kubernetes.slack.com/messages/metallb/). If
you're not already logged into the Kubernetes slack organization,
you'll need to [request an invite](http://slack.k8s.io/) before you
can join.

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
