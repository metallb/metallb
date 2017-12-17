---
title: Tutorial for ARP
weight: 10
---

In this tutorial we deploy MetalLB in a (ARM) cluster and announce a load-balanced IP using ARP. We
assume you have a bare metal [cluster
running](https://blog.hypriot.com/post/setup-kubernetes-raspberry-pi-cluster/).

The nice thing of the ARP protocol is that you don't need any fancy network hardware at all, your
current SOHO router should just do fine.

Here is the outline of what we're going to do:

1. Install MetalLB on the cluster,
1. Configure MetalLB to announce using the ARP and give it some IP addresses to manage,
1. Create a load-balanced service, and observe how MetalLB sets it up.

## Cluster addresses

The Raspberry PI cluster we have build using 192.168.1.0/24 as the address space. The main router
is configured to hand out (DHCP) address in 192.168.1.100-150. As the load-balanced IP addresses need
to be in the same network we'll allocate 192.168.1.240/28 for those.

## Install MetalLB

MetalLB runs in two parts: a cluster-wide controller, and a per-machine BGP/ARP speaker. Since we
have three nodes running in our cluster, we'll end up with the controller and three speakers.

Install MetalLB by applying the manifest:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml`

This manifest creates a bunch of resources. Most of them are related to access control, so that
MetalLB can read and write the Kubernetes objects it needs to do its job.

Ignore those bits for now, the two pieces of interest are the "controller" deployment, and the
"speaker" DaemonSet. Wait for these to start by monitoring `kubectl get pods -n metallb-system`.
Eventually, you should see four running pods, in addition to the BGP router from the previous step
(again, the pod name suffixes will be different on your cluster).

```
controller-74d6b85f86-xw5mx   1/1       Running   0          32m
speaker-kr2ks                 1/1       Running   0          31m
speaker-skfrp                 1/1       Running   0          31m
speaker-zmtb4                 1/1       Running   0          32m
```

Nothing has been announced yet, because we didn't supply a ConfigMap, nor a service with
a load-balanced address.

## Configure MetalLB

We have a sample MetalLB configuration in
[`manifests/example-arp-config.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/example-arp-config.yaml).
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
      protocol: arp
      cidr:
      - 192.168.1.240/28
```

MetalLB's configuration is a standard Kubernetes ConfigMap, `config` under the `metallb-system`
namespace. It contains two pieces of information: what IP addresses it's allowed to hand out and
which protocol to do that with.

In this configuration we tell MetalLB to hand out address from the 192.168.1.240/28 range and use
ARP: `protocol: arp` to do it. If you leave out the protocol bit it will default to BGP and things
will not work. Apply this configuration:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/example-arp-config-1.yaml`

The configuration should take effect within a few seconds. By following the logs we can see what's
going on: `kubectl logs -l app=speaker  -n metallb-system`:

```
I1217 10:18:05.212018       1 leaderelection.go:174] attempting to acquire leader lease...
I1217 10:18:07.312902       1 bgp_controller.go:176] Start config update
I1217 10:18:07.403537       1 bgp_controller.go:243] End config update
I1217 10:18:07.403748       1 arp_controller.go:128] Start config update
I1217 10:18:07.403883       1 arp_controller.go:143] End config update
```

Both the ARP and BGP speakers have seen the configuration, but haven't done anything else, because
there is no service IP to be announced.

## Create a load-balanced service

[`manifests/tutorial-2.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml)
contains a trivial service: an nginx pod, and a load-balancer service pointing at nginx. Deploy it
to the cluster:

`kubectl apply -f https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml`

Wait for nginx to start by monitoring `kubectl get pods`, until you see a running nginx pod. It
should look something like this:

```
NAME                         READY     STATUS    RESTARTS   AGE
nginx-558d677d68-j9x9x       1/1       Running   0          47s
```

Once it's running, take a look at the `nginx` service with `kubectl get service nginx`:

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

MetalLB is sending out unsolicited ARP responses and replies to ARP requests with the MAC address of
the node that has won the master election. It is using the first address of the assigned range
(192.168.1.240).

When you curl <http://192.168.1.240> you should see the default nginx page: "Welcome to nginx!"
