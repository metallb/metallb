# MetalLB

MetalLB is a load-balancer implementation for bare
metal [Kubernetes](https://kubernetes.io) clusters, using standard
routing protocols.

[![Project maturity: beta](https://img.shields.io/badge/maturity-beta-orange.svg)](https://metallb.universe.tf/concepts/maturity/) [![license](https://img.shields.io/github/license/metallb/metallb.svg?maxAge=2592000)](https://github.com/metallb/metallb/blob/main/LICENSE) [![CircleCI](https://img.shields.io/circleci/project/github/metallb/metallb.svg)](https://circleci.com/gh/metallb/metallb) [![Containers](https://img.shields.io/badge/containers-ready-green.svg)](https://hub.docker.com/u/metallb) [![Go report card](https://goreportcard.com/badge/github.com/metallb/metallb)](https://goreportcard.com/report/github.com/metallb/metallb)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/5391/badge)](https://bestpractices.coreinfrastructure.org/projects/5391)

Check out [MetalLB's website](https://metallb.universe.tf) for more
information.

# WARNING

Although the main branch has been relatively stable in the past, please be aware that it is the development branch.

Consuming manifests from main may result in unstable / non backward compatible deployments. We strongly suggest consuming a stable branch, as
described in the [official docs](https://metallb.universe.tf/installation/).

# Contributing

We welcome contributions in all forms. Please check out
the
[hacking and contributing guide](https://metallb.universe.tf/community/#contributing)
for more information.

Participation in this project is subject to
a [code of conduct](https://metallb.universe.tf/community/code-of-conduct/).

One lightweight way you can contribute is
to
[tell us that you're using MetalLB](https://github.com/metallb/metallb/issues/5),
which will give us warm fuzzy feelings :).

# Reporting security issues

You can report security issues in the github issue tracker. If you
prefer private disclosure, please email to all of the maintainers:

- fpaoline@redhat.com
- rbryant@redhat.com

We aim for initial response to vulnerability reports within 48
hours. The timeline for fixes depends on the complexity of the issue.
