Thanks for filing an issue! A few things before we get started:

1. This is not the place to ask for troubleshooting. Unfortunately
   troubleshooting LB issues in k8s takes a lot of time and effort
   that we simply don't have. If your load-balancers aren't working,
   please don't file an issue unless you have identified a _specific_
   bug to fix. Otherwise, please ask for help in the `#metallb`
   channel on the Kubernetes Slack.
2. Please search the bug tracker for duplicates before filing your
   bug.
3. If you're reporting a bug, please specify all of the following:
     - The bug itself, as detailed as you can.
     - Version of MetalLB
     - Version of Kubernetes
     - Name and version of network addon (e.g. Calico, Weave...)
     - Whether you've configured kube-proxy for iptables or ipvs mode
