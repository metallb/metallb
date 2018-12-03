## Deploying VPP Agent Docker Images Using Kubernetes

This document describes how the development Docker image can be deployed 
on an one-node development Kubernetes cluster using kubeadm][1] with 
[Calico] networking on Linux.

The recipe described here has been tested with kubeadm version 1.6.3.

Init kubeadm master:
```
kubeadm init

cp /etc/kubernetes/admin.conf $HOME/
chown $(id -u):$(id -g) $HOME/admin.conf
export KUBECONFIG=$HOME/admin.conf

kubectl taint nodes --all node-role.kubernetes.io/master-
```

Enable Calico networking:
```
kubectl apply -f http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml
```

Start ETCD on the host 
(a new ETCD instance on port 22379, since already running ETCD deployed 
by kubeadm does not accept remote connections):
```
sudo docker run -p 22379:2379 --name etcd --rm \
    quay.io/coreos/etcd:v3.0.16 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```

Start Kafka on the host:
```
sudo docker run -p 2181:2181 -p 9092:9092 --name kafka --rm \
 --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 spotify/kafka
```

Deploy VNF & vSwitch PODs:
```
kubectl apply -f vswitch-vpp.yaml
kubectl apply -f vnf-vpp.yaml
```

Verify the deployment:
```
kubectl get pods
kubectl describe pods
```

Write some config into ETCD (using etcd.conf that refers to the port 22379):
```
export ETCD_CONFIG=./etcd.conf
../../cmd/vpp-agent-ctl/topology.sh
```

Verify that the VPPs have been configured with some config:
```
kubectl describe pods | grep 'IP:'
IP:		192.168.243.216
IP:		192.168.243.217
```

```
telnet 192.168.243.216 5002
vpp# sh inter addr
local0 (dn):
memif0 (up):
  10.10.1.2/24
memif1 (up):
  166.111.8.2/24
```

```
vpp# sh inter addr
local0 (dn):
loop0 (up):
  6.0.0.100/24
loop1 (up):
  l2 bridge bd_id 1 bvi shg 0
  10.10.1.1/24
memif0 (up):
  l2 bridge bd_id 1 shg 0
memif1 (up):
  l2 bridge bd_id 1 shg 0
memif2 (up):
  l2 bridge bd_id 1 shg 0
memif3 (up):
  l2 bridge bd_id 2 shg 0
vxlan_tunnel0 (up):
  l2 bridge bd_id 2 shg 0
```

Verify logs:
```
kubectl describe pods | grep 'Container ID:'
    Container ID:	docker://8511cbbbecff744a06fae94b861f8030bd7be52d2c2db0533b63ac151d36b13c
    Container ID:	docker://81ed74b291611b4a9f6db2a9d37aba56d4fddc1125400d19fadfc96adb33cc6b
```

```
docker logs 8511cbbbecff744a06fae94b861f8030bd7be52d2c2db0533b63ac151d36b13c
docker logs 81ed74b291611b4a9f6db2a9d37aba56d4fddc1125400d19fadfc96adb33cc6b
```

[1]: https://kubernetes.io/docs/getting-started-guides/kubeadm/
[2]: http://docs.projectcalico.org/v2.2/getting-started/kubernetes/installation/hosted/kubeadm/ 
