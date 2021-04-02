# MetalLB operator
A kubernetes controller to configure MetaLLB loadbalancer as a service, base on [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework
## Prerequisites
Need to install the following packages
- kind
- kubebuilder
- kustomize
- kubectl
- go1.16

## Bringup KIND Cluster with multiple nodes
```
kind create cluster --config kind-config.yaml
```
<!---
## How to build the operator, install crd and run it on your desktop
```
make manager
make install
make run
```
-->

## Install MetalLB
```
./install-metallb.sh
```

## Build docker image for metalLB operator and run it as k8s deployment
```
export IMG=quay.io/mmahmoud/metallb-operator:latest
```
<!---
make docker-build
make docker-push
-->
```
make deploy
```

<!---
## How to check CRD configs
```
kubectl get crd
NAME                                             CREATED AT
metallbs.loadbalancer.loadbalancer.operator.io   2021-04-02T14:06:11Z

kubectl get metallb -n metallb-system -o yaml
apiVersion: v1
items:
- apiVersion: loadbalancer.loadbalancer.operator.io/v1
  kind: MetalLB
  metadata:
    annotations:
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"loadbalancer.loadbalancer.operator.io/v1","kind":"MetalLB","metadata":{"annotations":{},"name":"metallb-sample","namespace":"metallb-system"},"spec":{"address-pools":[{"addresses":["172.18.255.100-172.18.255.200"],"name":"default","protocol":"layer2"}]}}
    creationTimestamp: "2021-04-02T14:07:39Z"
    generation: 1
    managedFields:
    - apiVersion: loadbalancer.loadbalancer.operator.io/v1
      fieldsType: FieldsV1
      fieldsV1:
        f:metadata:
          f:annotations:
            .: {}
            f:kubectl.kubernetes.io/last-applied-configuration: {}
        f:spec:
          .: {}
          f:address-pools: {}
      manager: kubectl-client-side-apply
      operation: Update
      time: "2021-04-02T14:07:39Z"
    name: metallb-sample
    namespace: metallb-system
    resourceVersion: "1188"
    uid: fceb3c1e-f307-4a82-a522-41cfddd6eb29
  spec:
    address-pools:
    - addresses:
      - 172.18.255.100-172.18.255.200
      name: default
      protocol: layer2
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
```
-->


<!---
## How to find range of VIPs to use for loadbalancer service 
1- Find the CIDR of the bridge into tthe cluster

ip a s

You'll get output similar to the following

```
4: br-956cdadd3d33: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 02:42:8b:33:c3:fb brd ff:ff:ff:ff:ff:ff
    inet 172.19.0.1/16 brd 172.19.255.255 scope global br-956cdadd3d33
       valid_lft forever preferred_lft forever
    inet6 fc00:f853:ccd:e793::1/64 scope global
       valid_lft forever preferred_lft forever
    inet6 fe80::42:8bff:fe33:c3fb/64 scope link
       valid_lft forever preferred_lft forever
    inet6 fe80::1/64 scope link
       valid_lft forever preferred_lft forever
```
2- Use the sipcalc tool find the range of IP addresses that MetalLB can use.
```
sipcalc 172.19.0.1/16

WHERE

172.19.0.1/16 is the CIDR discovered using ip a s

You'll get put similar to the following:

-[ipv4 : 172.19.0.1/16] - 0

[CIDR]
Host address            - 172.19.0.1
Host address (decimal)  - 2886926337
Host address (hex)      - AC130001
Network address         - 172.19.0.0
Network mask            - 255.255.0.0
Network mask (bits)     - 16
Network mask (hex)      - FFFF0000
Broadcast address       - 172.19.255.255
Cisco wildcard          - 0.0.255.255
Addresses in network    - 65536
Network range           - 172.19.0.0 - 172.19.255.255
Usable range            - 172.19.0.1 - 172.19.255.254
```
-->
## To deploy sample config
- layer2 config example
```
kubectl apply -f config/samples/metallb_v1_layer2_config_example.yaml 
```
- bgp config example
```
kubectl apply -f config/samples/metallb_v1_bgp_config_example.yaml 
```

<!---
check to make sure configmap is created
````bigquery
kubectl get configmap -n metallb-system config -o yaml
apiVersion: v1
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - 172.18.255.1-172.18.255.250
kind: ConfigMap
metadata:
  creationTimestamp: "2021-04-02T01:55:08Z"
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:data:
        .: {}
        f:config: {}
    manager: main
    operation: Update
    time: "2021-04-02T01:55:08Z"
  name: config
  namespace: metallb-system
  resourceVersion: "1191"
  uid: 18fa200d-c5ce-4426-bd8a-2acd8475e741

````
-->

## Use NGINX service to test loadbalancer service
````bigquery
kubectl create deploy nginx --image nginx

kubectl expose deploy nginx --port 80 --type LoadBalancer

kubectl get services

You'll get out put similar to the following:

NAME         TYPE           CLUSTER-IP      EXTERNAL-IP    PORT(S)        AGE
kubernetes   ClusterIP      10.96.0.1       <none>         443/TCP        11m
nginx        LoadBalancer   10.96.148.110   172.19.255.1   80:32326/TCP   9s
Notice that the nginx service publishes an EXTERNAL-IP.
 Run the curl against the EXTERNAL-IP, for example:

curl 172.19.255.1

You'll get output similar to the following:

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