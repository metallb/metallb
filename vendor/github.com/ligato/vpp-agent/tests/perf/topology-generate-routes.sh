#!/usr/bin/env bash

VSWITCH_NAME="vpp1"
ETCD_CONTAINER="etcd"


docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif1 '
{
  "name": "memif1",
  "type": 2,
  "enabled": true,
  "memif": {
    "master": true,
    "id": 1,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses" : [
     "8.42.0.2/24"
  ]
}
'


if [ $1 -eq 0 ] ; then
   exit
fi

for i in $(eval echo {1..$1});do
    a=$(($i / 254 + 1))
    b=$(($i % 255))



docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/vrf/0/fib/6.$a.$b.0/24/8.42.0.1 "
{
  \"description\": \"Static route $i\",
  \"dst_ip_addr\": \"6.$a.$b.0/24\",
  \"next_hop_addr\": \"8.42.0.1\",
  \"outgoing_interface\": \"memif1\"

}
"


done

