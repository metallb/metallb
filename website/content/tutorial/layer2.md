---
title: Layer 2 mode tutorial
weight: 20
---

In this tutorial we deploy MetalLB in a cluster and announce a
load-balanced IP using layer 2 mode. We assume you have a bare metal
cluster already running, for example
a
[Raspberry Pi Kubernetes cluster](https://blog.hypriot.com/post/setup-kubernetes-raspberry-pi-cluster/).

The nice thing about layer 2 mode is that you don't need any fancy
network hardware at all, it should just work on any ethernet network.

Here is the outline of what we're going to do:

1. Install MetalLB on the cluster,
1. Configure MetalLB to announce using layer 2 mode and give it some
   IP addresses to manage,
1. Create a load-balanced service, and observe how MetalLB sets it up.

## Cluster addresses

For this tutorial, we'll assume the cluster is set up on a network
using `192.168.1.0/24`. The main router is configured to hand out DHCP
address in the `192.168.1.100—192.168.1.150` range.

We need to allocate another chunk of this IP space for MetalLB
services. We'll use `192.168.1.240-192.168.1.250` for this.

If your cluster is not using the same addresses, you'll need to
substitute the appropriate address ranges in the rest of this
tutorial. We'll point out the places where you need to make edits.

## Install MetalLB

MetalLB runs in two parts: a cluster-wide controller, and a
per-machine protocol speaker.

Install MetalLB by applying the manifest:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml`

This manifest creates a bunch of resources. Most of them are related
to access control, so that MetalLB can read and write the Kubernetes
objects it needs to do its job.

Ignore those bits for now, the two pieces of interest are the
"controller" deployment, and the "speaker" DaemonSet. Wait for these
to start by monitoring `kubectl get pods -n metallb-system`.
Eventually, you should see some running pods.

```
controller-74d6b85f86-xw5mx   1/1       Running   0          32m
speaker-kr2ks                 1/1       Running   0          31m
speaker-skfrp                 1/1       Running   0          31m
speaker-zmtb4                 1/1       Running   0          32m
```

You should see one controller pod, and one speaker pod for each node
in your cluster.

{{% notice "note" %}}
Your pods will have a slightly different names, because the suffix is
randomly generated.
{{% /notice %}}

Nothing has been announced yet, because we didn't supply a ConfigMap, nor a service with
a load-balanced address.

## Configure MetalLB

We have a sample MetalLB configuration in
[`manifests/example-layer2-config.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/example-layer2-config.yaml).
Let's take a look at it before applying it:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: my-ip-space
      protocol: layer2
      addresses:
      - 192.168.1.240-192.168.1.250
```

{{% notice "note" %}}
If you used different IP addresses in your cluster, change the IP
range in this configuration before applying it.
{{% /notice  %}}

MetalLB's configuration is a standard Kubernetes ConfigMap, `config`
under the `metallb-system` namespace. It contains two pieces of
information: what IP addresses it's allowed to hand out and which
protocol to do that with.

In this configuration we tell MetalLB to hand out address from the
`192.168.1.240-192.168.1.250` range, using layer 2 mode (`protocol:
layer2`). Apply this configuration:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/example-layer2-config.yaml`

The configuration should take effect within a few seconds. By
following the logs we can see what's going on: `kubectl logs -l
component=speaker -n metallb-system`:

```
I1217 10:18:05.212018       1 leaderelection.go:174] attempting to acquire leader lease...
I1217 10:18:07.312902       1 bgp_controller.go:176] Start config update
I1217 10:18:07.403537       1 bgp_controller.go:243] End config update
I1217 10:18:07.403748       1 arp_controller.go:128] Start config update
I1217 10:18:07.403883       1 arp_controller.go:143] End config update
```

The speaker has loaded the configuration, but hasn't done anything
else yet, because there are no LoadBalancer services in the cluster.

## Create a load-balanced service

[`manifests/tutorial-2.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml) contains
a trivial service: an nginx pod, and a load-balancer service pointing
at nginx. Deploy it to the cluster:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml`

Wait for nginx to start by monitoring `kubectl get pods`, until you
see a running nginx pod. It should look something like this:

```
NAME                         READY     STATUS    RESTARTS   AGE
nginx-558d677d68-j9x9x       1/1       Running   0          47s
```

Once it's running, take a look at the `nginx` service with `kubectl
get service nginx`:

```
NAME      TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)        AGE
nginx     LoadBalancer   10.102.30.250   192.168.1.240   80:31517/TCP   1d
```

We have an external IP! Looking through the logs of `speaker` we see it happening:

```
I1217 10:18:07.409788       1 arp_controller.go:53] default/nginx: start update
I1217 10:18:07.409867       1 arp_controller.go:96] default/nginx: announcable, making advertisement
I1217 10:18:07.409977       1 arp_controller.go:107] default/nginx: end update
...
I1217 10:19:01.905426       1 leader.go:61] Sending unsolicited ARPs for 1 addresses
I1217 10:19:05.005671       1 arp.go:96] Request: who-has 192.168.1.240?  tell 192.168.1.1 (b4:75:0e:63:b2:20). reply: 192.168.1.240 is-at b8:27:eb:86:e2:85
I1217 10:19:05.105780       1 arp.go:96] Request: who-has 192.168.1.240?  tell 192.168.1.1 (b4:75:0e:63:b2:20). reply: 192.168.1.240 is-at b8:27:eb:86:e2:85
I1217 10:19:05.235623       1 arp.go:96] Request: who-has 192.168.1.240?  tell 192.168.1.1 (b4:75:0e:63:b2:20). reply: 192.168.1.240 is-at b8:27:eb:86:e2:85
```

MetalLB is sending out replies to ARP requests with the MAC address of
the node that has won the leader election. It is using the first
address of the assigned range (192.168.1.240).

When you `curl http://192.168.1.240` you should see the default nginx
page: "Welcome to nginx!"
