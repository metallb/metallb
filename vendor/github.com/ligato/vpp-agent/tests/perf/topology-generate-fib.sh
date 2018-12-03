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

docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/bd/B1 '
{
  "name": "B1",
  "flood": true,
  "unknown_unicast_flood": true,
  "forward": true,
  "learn": true,
  "interfaces": [
    {
      "name": "memif1"
    }
  ]
}
'

if [ $1 -eq 0 ] ; then
   exit
fi

for i in $(eval echo {1..$1});do
    a=$(($i / 16 / 16 / 16 % 16))
    b=$(($i / 16 / 16 % 16))
    c=$(($i / 16 % 16))
    d=$(($i % 16))
    hexchars="0123456789ABCDEF"

    w=${hexchars:$a:1}
    x=${hexchars:$b:1}
    y=${hexchars:$c:1}
    z=${hexchars:$d:1}


docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/bd/B1/fib/62:89:C6:A3:$w$x:$y$z "
{
  \"phys_address\": \"62:89:C6:A3:$w$x:$y$z\",
  \"bridge_domain\": \"B1\",
  \"outgoing_interface\": \"memif1\",
  \"static_config\": true,
  \"bridged_virtual_interface\": false
}

"


done

