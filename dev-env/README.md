# dev-env

## Starting the development environment

This directory contains supporting files for the included MetalLB development
environment. The environment is run using the following command from the root
of your git clone, which runs MetalLB in an unconfigured state:

```
inv dev-env
```

For configuring MetalLB to peer with a BGP router running
in a container:

```
inv dev-env --protocol bgp
```

You may also launch a dev environment with layer2, with:

```
inv dev-env --protocol layer2
```

For more information, see help:

```
inv dev-env --help
```

The environment can be cleaned up with:

```
inv dev-env-cleanup
```

## Requirements

* Go 1.15+
* Python 3
* Docker
* [KIND - Kubernetes in Docker](https://kind.sigs.k8s.io/docs/user/quick-start/)
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
* [controller-gen](https://book.kubebuilder.io/reference/controller-gen.html)

You may install the required python modules using the `requirements.txt` in this directory, with:

```
pip install -r requirements.txt
```

## Using the development environment

### BGP

#### IPv4 unicast

This assumes a 3-node `kind` based test cluster as set up by running `inv
dev-env -p bgp`, which sets up the development environment with some
configuration and a BGP router for development and testing purposes.

Along with the cluster, there will be a container named `frr` which is the BGP
router. Note that this only works in an IPv4 configuration for now.

The configuration used for MetalLB can be found in `bgp/config.yaml`. The FRR
configuration is in the `bgp/frr/` directory.

Observe that the BGP speakers have peered with our router:

```
$ docker exec frr vtysh -c "show ip bgp summary"

IPv4 Unicast Summary:
BGP router identifier 172.18.0.5, local AS number 64512 vrf-id 0
BGP table version 1
RIB entries 1, using 192 bytes of memory
Peers 3, using 43 KiB of memory

Neighbor        V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd   PfxSnt
172.18.0.2      4      64512        22        21        0    0    0 00:09:47            1        0
172.18.0.3      4      64512        22        21        0    0    0 00:09:47            1        0
172.18.0.4      4      64512        22        21        0    0    0 00:09:47            1        0

Total number of neighbors 3
```

Create an `nginx` Deployment and a corresponding Service of type LoadBalancer:

```
kubectl apply -f dev-env/testsvc.yaml
```

Observe the IP address allocated to the Service by MetalLB:

```
$ kubectl get svc nginx
NAME    TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)        AGE
nginx   LoadBalancer   10.106.88.254   192.168.10.0   80:30076/TCP   53m
```

Ensure a route to this Service has been published by the MetalLB BGP speaker:

```
$ docker exec frr vtysh -c "show ip bgp detail"
BGP table version is 1, local router ID is 172.18.0.5, vrf id 0
Default local pref 100, local AS 64512
Status codes:  s suppressed, d damped, h history, * valid, > best, = multipath,
               i internal, r RIB-failure, S Stale, R Removed
Nexthop codes: @NNN nexthop's vrf id, < announce-nh-self
Origin codes:  i - IGP, e - EGP, ? - incomplete

   Network          Next Hop            Metric LocPrf Weight Path
*>i192.168.10.0/32  172.18.0.2                      0      0 ?
*=i                 172.18.0.3                      0      0 ?
*=i                 172.18.0.4                      0      0 ?

Displayed  1 routes and 3 total paths
```

Now validate that you can connect to the Service from the router:

```
$ docker exec -it frr bash
bash-5.0# echo "GET /" | nc 192.168.10.0 80
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

#### IPv6 unicast

Bring up the cluster using ipv6 address family
```
inv dev-env -i ipv6 -p bgp -b frr
```

Observe that BGP speakers have BGP sessions with our router:
```
docker exec frr vtysh -c "show bgp ipv6 unicast sum"
BGP router identifier 172.18.0.5, local AS number 64512 vrf-id 0
BGP table version 1
RIB entries 1, using 192 bytes of memory
Peers 3, using 43 KiB of memory
Neighbor              V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd   PfxSnt
fc00:f853:ccd:e793::2 4      64512        25        24        0    0    0 00:21:05            1        0
fc00:f853:ccd:e793::3 4      64512        25        24        0    0    0 00:21:04            1        0
fc00:f853:ccd:e793::4 4      64512        25        24        0    0    0 00:21:05            1        0
Total number of neighbors 3
```

Create IPv6 `nginx` service of type LoadBalancer

```
kubectl apply -f dev-env/testsvc_ipv6.yaml
kubectl get svc
NAME         TYPE           CLUSTER-IP         EXTERNAL-IP            PORT(S)        AGE
kubernetes   ClusterIP      fd00:10:96::1      <none>                 443/TCP        15h
nginx        LoadBalancer   fd00:10:96::d362   fc00:f853:ccd:e799::   80:32587/TCP   15h
```

Ensure a route to this service has been published by MetalLB BGP speaker

```
kubectl exec -it speaker-c9fx8 -n metallb-system -c frr -- vtysh -c "show bgp ipv6 unicast sum"
BGP router identifier 172.18.0.2, local AS number 64512 vrf-id 0
BGP table version 1
RIB entries 1, using 192 bytes of memory
Peers 1, using 14 KiB of memory
Neighbor              V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd   PfxSnt
fc00:f853:ccd:e793::5 4      64512        27        29        0    0    0 00:21:55            0        1
Total number of neighbors 1
```

## Layer 2

This assumes a 3-node `kind` based test cluster as set up by running `inv
dev-env -p layer2`, which sets up the development environment with some
configuration. Note that this only works in an IPv4 configuration for now.

The configuration used for MetalLB can be found in `layer2/config.yaml`.

Create an `nginx` Deployment and a corresponding Service of type LoadBalancer:

```
kubectl apply -f dev-env/testsvc.yaml
```

Observe the IP address allocated to the Service by MetalLB:

```
$ kubectl get svc nginx
NAME    TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)        AGE
nginx   LoadBalancer   10.106.88.254   192.168.10.0   80:30076/TCP   53m
```

Now validate that you can connect to the Service:

```
$ docker exec -it frr bash
bash-5.0# echo "GET /" | nc 192.168.10.0 80
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

## Enabling the apiserver audit logs

To enable the apiserver audit logs, to understand what are the impacts of the controller and the speakers over
the apiserver, the `-t, --with-api-audit` flag must be passed to `inv dev-env`.

When the audit logs are enabled, they can be inspected by running:

```bash
docker exec kind-control-plane cat /var/log/kubernetes/kube-apiserver-audit.log
```
