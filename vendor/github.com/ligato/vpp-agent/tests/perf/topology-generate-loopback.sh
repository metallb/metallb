#!/usr/bin/env bash

VSWITCH_NAME="vpp1"
ETCD_CONTAINER="etcd"


for i in $(eval echo {1..$1});do
   echo $i
# VSWITCH - create a loopback interface
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/loop$i "
{
  \"name\": \"loop$i\",
  \"enabled\": true,
  \"mtu\": 1500,
  \"ip_addresses\": [
    \"6.0.0.$i/24\"
  ]
}
"

done

