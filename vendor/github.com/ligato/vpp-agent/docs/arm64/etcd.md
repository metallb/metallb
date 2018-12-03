
## Running Etcd Server on Local Host - ARM64 platform

You can run an ETCD server in a separate container on your local
host as follows:
```
sudo docker run -p 2379:2379 --name etcd -e ETCDCTL_API=3 -e ETCD_UNSUPPORTED_ARCH=arm64 \
    quay.io/coreos/etcd:v3.3.8-arm64 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```
The ETCD server will be available on your host OS IP (most likely 
`172.17.0.1` in the default docker environment) on port `2379`.

Call the agent via ETCD using the testing client:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -tap
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -tapd
```
**Note for ARM64:**

Check for proper etcd ARM64 docker image in the [official repository](https://quay.io/repository/coreos/etcd?tag=latest&tab=tags).
Currently you must use the parameter "-e ETCD_UNSUPPORTED_ARCH=arm64".
```
```
