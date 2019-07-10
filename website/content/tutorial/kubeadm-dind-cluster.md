---
title: BGP on Kubernetes multi-node cluster ( based on kubeadm and DIND )
weight: 30
draft: true
---

In this tutorial, we'll set up a fully containaraized environment with one
docker container to run as an external BGP router and a Kubernetes multi-node
cluster based on
[kubeadm-dind-cluster](https://github.com/kubernetes-sigs/kubeadm-dind-cluster),
configure MetalLB to use them, and create some load-balanced services. We'll be
able to inspect the state of the BGP routers, see that they reflect the intent
that we expressed in Kubernetes and create real traffic from the containers to
these services.

Because this will be a simulated environment using docker containers, this setup
will lets you inspect the routers's state and see what it _would_ do in a real
deployment. Once you've experimented in this setting and are ready to set up
MetalLB on a real cluster, refer to the [installation
guide](https://metallb.universe.tf/installation/) for instructions.

Here is the outline of what we're going to do:

1. Set up a Kubernetes multi-node cluster based on kubeadm-dind-cluster,
2. Set up a test BGP router that we can inspect and use in subsequent steps,
3. Install MetalLB on the cluster,
4. Configure MetalLB to peer with our test BGP router, and give it some IP
   addresses to manage,
5. Create a load-balanced service, and observe how MetalLB sets it up,
6. Tear down the playground.

{{% notice note %}} This tutorial currently only works on amd64 (aka x86_64)
systems, because the test BGP router container image doesn't work on other
platforms yet.  {{% /notice %}}

## Set up a Kubernetes cluster using kubeadm-dind-cluster

If you don't already have a kubeadm-dind-cluster set up, follow the
[instructions](https://github.com/kubernetes-sigs/kubeadm-dind-cluster/blob/master/README.md)
on Github to install a Kubernetes multi-node cluster and get your playground
cluster running. 

You can use preconfigured scripts for Kubernetes versions 1.10 through 1.13. For
example, to start a Kubernetes 1.13 cluster:

``` $ wget
https://github.com/kubernetes-sigs/kubeadm-dind-cluster/releases/download/v0.1.0/dind-cluster-v1.13.sh
$ chmod +x dind-cluster-v1.13.sh

$ # start the cluster $ ./dind-cluster-v1.13.sh up

$ # add kubectl directory to PATH $ export
PATH="$HOME/.kubeadm-dind-cluster:$PATH"

$ kubectl get nodes NAME          STATUS    ROLES     AGE       VERSION
kube-master   Ready     master    4m        v1.13.0 kube-node-1   Ready
<none>    2m        v1.13.0 kube-node-2   Ready     <none>    2m        v1.13.0

$ # k8s dashboard available at
http://localhost:8080/api/v1/namespaces/kube-system/services/kubernetes-dashboard:/proxy
```

## Set up BGP containers

MetalLB exposes load-balanced services using the BGP routing protocol, so we
need a BGP router to talk to. In a production cluster, this would be set up as a
dedicated hardware router (e.g. an Ubiquiti EdgeRouter), or a soft router using
open-source software (e.g. a Linux machine running the
[BIRD](http://bird.network.cz) or [Quagga](http://www.nongnu.org/quagga/)
routing suite).

For this tutorial, we'll deploy an external BGP router on a docker container
that runs Quagga. It will be configured to speak BGP and will configure the
container routing table to forward traffic based on the data they receive. We'll
just interact with the router to see what a real router _would_ do and we'll
generate traffic from the BGP router to analyze the network behavior.

Deploy the external router with `docker` in a different terminal:

`docker run -it --rm --net=kubeadm-dind-net --cap-add=NET_ADMIN
--cap-add=NET_BROADCAST --cap-add=NET_RAW --name extBGP ajnouri/quagga_alpine`

This will create a docker containers with one interface in the same network that 
our kubernetes cluster and will give us a CLI interface to interact with the
router.

```
Hello, this is Quagga (version 0.99.24.1).
Copyright 1996-2005 Kunihiro Ishiguro, et al.

98e84786d9fb#
```

Docker will assign a network address, to our router,  in the same subnet as our
kubernetes cluster dynamically. In order to find out our assigned IP address:

```
98e84786d9fb# show interface
Interface eth0 is up, line protocol detection is disabled
  index 87 metric 0 mtu 1500
  flags: <UP,BROADCAST,RUNNING,MULTICAST>
  HWaddr: 02:42:0a:c0:00:07
  inet 10.192.0.7/24
Interface lo is up, line protocol detection is disabled
  index 1 metric 0 mtu 65536
  flags: <UP,LOOPBACK,RUNNING>
  inet 127.0.0.1/8
```

Obviously, we haven't configured our BGP router and MetalLB isn't connected to
our routers, it's not installed yet! Let's address that. Keep the router CLI
open, we'll come back to it shortly.

## Install MetalLB

MetalLB runs in two parts: a cluster-wide controller, and a per-machine BGP
speaker. Since kubeadm-dind-cluster by default is a Kubernetes cluster with a
single master node and two wokers, we'll end up with the controller and two BGP
speakers.

Install MetalLB by applying the manifest:

`kubectl apply -f
https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml`

This manifest creates a bunch of resources. Most of them are related to access
control, so that MetalLB can read and write the Kubernetes objects it needs to
do its job.

Ignore those bits for now, the two pieces of interest are the "controller"
deployment, and the "speaker" daemonset. Wait for these to start by monitoring
`kubectl get pods -n metallb-system`. Eventually, you should see three running
pods, one for the controller and one speaker per worker nodes (again, the pod name
suffixes will be different on your cluster).

``` 
$kubectl get pods -n metallb-system -o wide
NAME                             READY     STATUS         RESTARTS   AGE       IP           NODE          NOMINATED NODE
controller-765899887-2rdh2       1/1       Running        0          11s       10.244.2.6   kube-node-1   <none>
speaker-b6kng                    1/1       Running        0          12s       10.192.0.3   kube-node-1   <none>
speaker-zfqrn                    1/1       Running        0          12s       10.192.0.4   kube-node-2   <none>
```

That's because the MetalLB installation manifest doesn't come with a
configuration, so both the controller and BGP speaker are sitting idle, waiting
to be told what they should do. Let's fix that!

## Configure the BGP session

We have to configure the BGP connection between MetalLB and our external
router.  For that we need the IP addresses of our BGP router and the MetalLB
speakers, also we have to define our ASN number.

To configure the external router we have to use the CLI and type the following
commands (replace the neighbours IP addresses for the ones used by your
speakers pods).

```
98e84786d9fb# configure terminal
98e84786d9fb(config)# router bgp 64512
98e84786d9fb(config-router)# neighbor 10.192.0.3 remote-as 64512
98e84786d9fb(config-router)# neighbor 10.192.0.4 remote-as 64512
98e84786d9fb(config-router)# exit
98e84786d9fb(config)# exit
```

We have a sample MetalLB configuration in
[`manifests/example-config.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/example-config.yaml).

MetalLB's configuration is a standard Kubernetes ConfigMap, `config` under the
`metallb-system` namespace. It contains two pieces of information: who MetalLB
should talk to, and what IP addresses it's allowed to hand out.

In this configuration, we're setting up a BGP peering with `10.192.0.7` , we
need to replace it by the address of our BGP container interface that we
obtained in previous steps.  We're giving MetalLB 256 IP addresses to use, from
198.51.100.0 to 198.51.100.255. The final section gives MetalLB some BGP
attributes that it should use when announcing IP addresses to our router.

Apply the MetalLB configuration now:

```
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    peers:
    - my-asn: 64512
      peer-asn: 64512
      peer-address: 10.192.0.7
    address-pools:
    - name: my-ip-space
      protocol: bgp
      addresses:
      - 198.51.100.0/24
EOF
```

The configuration should take effect within a few seconds. Check the status on
the external BGP router CLI:

```
98e84786d9fb#   show ip bgp summary
BGP router identifier 10.192.0.7, local AS number 64512
RIB entries 0, using 0 bytes of memory
Peers 2, using 9136 bytes of memory

Neighbor        V         AS MsgRcvd MsgSent   TblVer  InQ OutQ Up/Down  State/PfxRcd
10.192.0.3      4 64512       1       4        0    0    0 00:00:49        0
10.192.0.4      4 64512       1       4        0    0    0 00:00:49        0

Total number of neighbors 2
```

Success! The MetalLB BGP speaker connected to our routers. You can verify this
by looking at the column state and the logs for the BGP speaker. Run `kubectl logs -n metallb-system
-l app=speaker`, and among other log entries, you should find something like:

``` 
{"caller":"main.go:295","configmap":"metallb-system/config","event":"endUpdate","msg":"end of config update","ts":"2019-01-06T23:17:32.544915738Z"}
{"caller":"k8s.go:346","configmap":"metallb-system/config","event":"configLoaded","msg":"config (re)loaded","ts":"2019-01-06T23:17:32.544949542Z"}
struct { Version uint8; ASN16 uint16; HoldTime uint16; RouterID uint32; OptsLen uint8 }{Version:0x4, ASN16:0xfc00, HoldTime:0xb4, RouterID:0xac00007, OptsLen:0x1e}
{"caller":"bgp.go:63","event":"sessionUp","localASN":64512,"msg":"BGP session established","peer":"10.192.0.7:179","peerASN":64512,"ts":"2019-01-06T23:17:32.546157869Z"}
```

However, as the BGP router output shows us, MetalLB is connected, but isn't telling
them about any services yet. That's because all the services we've defined so
far are internal to the cluster. Let's change that!

## Create a load-balanced service

[`manifests/tutorial-2.yaml`](https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml)
contains a trivial service: an nginx pod, and a load-balancer service pointing
at nginx. Deploy it to the cluster now:

`kubectl apply -f
https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml`

Again, wait for nginx to start by monitoring `kubectl get pods`, until you see a
running nginx pod. It should look something like this:

```
NAME                         READY     STATUS    RESTARTS   AGE
nginx-558d677d68-j9x9x       1/1       Running   0          47s
```

Once it's running, take a look at the `nginx` service with `kubectl get service
nginx`:

```
NAME      TYPE           CLUSTER-IP   EXTERNAL-IP    PORT(S)        AGE
nginx     LoadBalancer   10.96.0.29   198.51.100.0   80:32732/TCP   1m
```

We have an external IP! Because the service is of type LoadBalancer, MetalLB
took `198.51.100.0` from the address pool we configured, and assigned it to the
nginx service. You can see this even more clearly by looking at the event
history for the service, with `kubectl describe service nginx`:

```
Type    Reason          Age   From                Message ----    ------
----  ----                ------- Normal  IPAllocated     24m
metallb-controller  Assigned IP "198.51.100.0"
```

We can see the new IP announced in our external BGP router:

```
98e84786d9fb# show ip bgp
BGP table version is 0, local router ID is 10.192.0.7
Status codes: s suppressed, d damped, h history, * valid, > best, = multipath,
              i internal, r RIB-failure, S Stale, R Removed
Origin codes: i - IGP, e - EGP, ? - incomplete

   Network          Next Hop            Metric LocPrf Weight Path
*>i198.51.100.0/32  10.192.0.3                      0      0 ?
* i                 10.192.0.4                      0      0 ?

Total number of prefixes 1
```

Success! MetalLB told our routers that 198.51.100.0 exists on our Kubernetes cluster,
and that the routers should forward any traffic for that IP to us. We can check
it from the external router:

```
98e84786d9fb# start-shell
/ # wget 198.51.100.0
Connecting to 198.51.100.0 (198.51.100.0:80)
index.html           100%
|*************************************************************************************************************************|
612   0:00:00 ETA

```

Also, we can see the traffic sniffing directly in the docker bridge, for that we
first need to get the docker bridge id:

```
docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
58d375b8bce5        bridge              bridge              local
b30bd3913e67        host                host                local
4a32d46106bd        kubeadm-dind-net    bridge              local
25648e75d79a        none                null                local
```

And then we can check the traffic from the external BGP docker container to the
nginx service, the bridge interface is `br-<NETWORK_ID>`:


```
$ sudo tcpdump -ni br-4a32d46106bd tcp port 80
tcpdump: verbose output suppressed, use -v or -vv for full protocol decode
listening on br-4a32d46106bd, link-type EN10MB (Ethernet), capture size 262144
bytes
13:35:44.310903 IP 10.192.0.5.42134 > 198.51.100.0.80: Flags [S], seq
1643200791, win 29200, options [mss 1460,sackOK,TS val 880972715 ecr
0,nop,wscale 7], length 0
13:35:44.311080 IP 198.51.100.0.80 > 10.192.0.5.42134: Flags [S.], seq
351446290, ack 1643200792, win 28960, options [mss 1460,sackOK,TS val 880972715
ecr 880972715,nop,wscale 7], length 0
13:35:44.311132 IP 10.192.0.5.42134 > 198.51.100.0.80: Flags [.], ack 1, win
229, options [nop,nop,TS val 880972715 ecr 880972715], length 0
13:35:44.311230 IP 10.192.0.5.42134 > 198.51.100.0.80: Flags [P.], seq 1:76, ack
1, win 229, options [nop,nop,TS val 880972715 ecr 880972715], length 75: HTTP:
GET / HTTP/1.1
13:35:44.311299 IP 198.51.100.0.80 > 10.192.0.5.42134: Flags [.], ack 76, win
227, options [nop,nop,TS val 880972715 ecr 880972715], length 0
13:35:44.311416 IP 198.51.100.0.80 > 10.192.0.5.42134: Flags [P.], seq 1:234,
ack 76, win 227, options [nop,nop,TS val 880972715 ecr 880972715], length 233:
HTTP: HTTP/1.1 200 OK
13:35:44.311480 IP 10.192.0.5.42134 > 198.51.100.0.80: Flags [.], ack 234, win
237, options [nop,nop,TS val 880972715 ecr 880972715], length 0
13:35:44.311536 IP 198.51.100.0.80 > 10.192.0.5.42134: Flags [FP.], seq 234:846,
ack 76, win 227, options [nop,nop,TS val 880972715 ecr 880972715], length 612:
HTTP
```

## Teardown

If you're not using the kubernetes cluster for anything else, you can clean up
simply by running:

```
$ # stop the cluster
$ ./dind-cluster-v1.13.sh down

$ # remove DIND containers and volumes
$ ./dind-cluster-v1.13.sh clean
```

If you only want to tear down all of MetalLB, as well as our test BGP routers and the
nginx load-balanced service.

`kubectl delete -f
https://raw.githubusercontent.com/google/metallb/master/manifests/metallb.yaml, https://raw.githubusercontent.com/google/metallb/master/manifests/tutorial-2.yaml`

