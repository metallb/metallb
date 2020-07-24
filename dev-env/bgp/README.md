# BGP test env

This assumes a 3-node `kind` based test cluster as set up by running `inv
dev-env -p bgp`, which sets up the development environment with some
configuration and a BGP router for development and testing purposes.

Along with the cluster, there will be a container named `frr` which is the BGP
router.  Note that this only works in an IPv4 configuration for now.

The configuration used for MetalLB can be found in `config.yaml`.  The FRR
configuration is in the `frr/` directory.

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

Create an `nginx` Deployment and a corresponding Service LoadBalancer:

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
