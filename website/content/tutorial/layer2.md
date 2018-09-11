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
{"branch":"HEAD","caller":"main.go:63","commit":"69a8379e","msg":"MetalLB speaker starting version 0.7.3 (commit 69a8379e, branch HEAD)","ts":"2018-09-11T23:24:25.668134462Z","version":"0.7.3"}
{"caller":"announcer.go:89","event":"createARPResponder","interface":"eth0","msg":"created ARP responder for interface","ts":"2018-09-11T23:24:25.719365888Z"}
{"caller":"announcer.go:98","event":"createNDPResponder","interface":"eth0","msg":"created NDP responder for interface","ts":"2018-09-11T23:24:25.721240251Z"}
{"caller":"announcer.go:89","event":"createARPResponder","interface":"cni0","msg":"created ARP responder for interface","ts":"2018-09-11T23:24:25.849696054Z"}
{"caller":"announcer.go:98","event":"createNDPResponder","interface":"cni0","msg":"created NDP responder for interface","ts":"2018-09-11T23:24:25.850849849Z"}
{"caller":"announcer.go:89","event":"createARPResponder","interface":"veth06468c77","msg":"created ARP responder for interface","ts":"2018-09-11T23:40:27.477873613Z"}
{"caller":"announcer.go:98","event":"createNDPResponder","interface":"veth06468c77","msg":"created NDP responder for interface","ts":"2018-09-11T23:40:27.478660224Z"}
{"caller":"main.go:271","configmap":"metallb-system/config","event":"startUpdate","msg":"start of config update","ts":"2018-09-11T23:27:08.031692769Z"}
{"caller":"main.go:295","configmap":"metallb-system/config","event":"endUpdate","msg":"end of config update","ts":"2018-09-11T23:27:08.032103652Z"}
{"caller":"k8s.go:346","configmap":"metallb-system/config","event":"configLoaded","msg":"config (re)loaded","ts":"2018-09-11T23:27:08.032308547Z"}
{"caller":"main.go:159","event":"startUpdate","msg":"start of service update","service":"default/kubernetes","ts":"2018-09-11T23:27:08.037989654Z"}
{"caller":"main.go:163","event":"endUpdate","msg":"end of service update","service":"default/kubernetes","ts":"2018-09-11T23:27:08.038307829Z"}
{"caller":"main.go:159","event":"startUpdate","msg":"start of service update","service":"kube-system/kube-dns","ts":"2018-09-11T23:27:08.038511995Z"}
{"caller":"main.go:163","event":"endUpdate","msg":"end of service update","service":"kube-system/kube-dns","ts":"2018-09-11T23:27:08.038682775Z"}
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

We have an external IP! Looking through the logs of `speaker` with `kubectl logs -l component=speaker -n metallb-system` we see it happening:

```
{"caller":"main.go:159","event":"startUpdate","msg":"start of service update","service":"default/nginx","ts":"2018-09-11T23:40:26.697718946Z"}
{"caller":"main.go:229","event":"serviceAnnounced","ip":"192.168.1.240","msg":"service has IP, announcing","pool":"my-ip-space","protocol":"layer2","service":"default/nginx","ts":"2018-09-11T23:40:26.69818035Z"}
{"caller":"main.go:231","event":"endUpdate","msg":"end of service update","service":"default/nginx","ts":"2018-09-11T23:40:26.698532015Z"}
{"caller":"announcer.go:89","event":"createARPResponder","interface":"veth06468c77","msg":"created ARP responder for interface","ts":"2018-09-11T23:40:27.477873613Z"}
{"caller":"announcer.go:98","event":"createNDPResponder","interface":"veth06468c77","msg":"created NDP responder for interface","ts":"2018-09-11T23:40:27.478660224Z"}
{"caller":"arp.go:102","interface":"eth0","ip":"192.168.1.240","msg":"got ARP request for service IP, sending response","responseMAC":"ab:cd:ef:ab:cd:ef","senderIP":"192.168.1.1","senderMAC":"12:34:56:78:90:ab","ts":"2018-09-11T23:40:31.63935007Z"}
```

MetalLB is sending out replies to ARP requests with the MAC address of
the node that has won the leader election. It is using the first
address of the assigned range (192.168.1.240).

When you `curl http://192.168.1.240` you should see the default nginx
page: "Welcome to nginx!"
