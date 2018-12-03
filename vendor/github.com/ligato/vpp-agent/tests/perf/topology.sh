#!/usr/bin/env bash

VSWITCH_NAME="vpp1"
RNG_NAME="rng-vpp"
USSCHED_NAME="ussched-vpp"
VNF_NAME="vnf-vpp"
ETCD_CONTAINER="etcd"

# sudo docker run -it --name vpp1 -e MICROSERVICE_LABEL=vpp1 -v/tmp/:/tmp/ --privileged --rm dev_vpp_agent bash
# sudo docker run -it --name rng -e MICROSERVICE_LABEL=rng-vpp -v/tmp/:/tmp/ --privileged --rm dev_vpp_agent
# sudo docker run -it --name ussched -e MICROSERVICE_LABEL=ussched-vpp -v/tmp/:/tmp/ --privileged --rm dev_vpp_agent
# sudo docker run -it --name vnf -e MICROSERVICE_LABEL=vnf-vpp -v/tmp/:/tmp/ --privileged --rm dev_vpp_agent

# VSWITCH - configure physical interface GigabitEthernet0/8/0
# !!! needs to exist and be whitelisted in VPP, e.g. dpdk { dev 0000:00:08.0 } !!!
# This works for my VirtualBox ethernet interface:
# modprobe igb_uio
# vpp unix { interactive } dpdk { dev 0000:00:08.0 uio-driver igb_uio }
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/GigabitEthernet0/8/0 '
{
  "name": "GigabitEthernet0/8/0",
  "type": 1,
  "enabled": true,
  "mtu": 1500,
  "ip_addresses": [
    "8.42.0.2/24"
  ]
}
'

# VSWITCH - create a loopback interface
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/loop1 '
{
  "name": "loop1",
  "enabled": true,
  "phys_address": "8a:f1:be:90:00:dd",
  "mtu": 1500,
  "ip_addresses": [
    "6.0.0.100/24"
  ]
}
'

# VSWITCH - create a vxlan interface
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/vxlan1 '
{
  "name": "vxlan1",
  "type": 5,
  "enabled": true,
  "vxlan": {
    "src_address": "8.42.0.2",
    "dst_address": "8.42.0.1",
    "vni": 13
  }
}
'

# VSWITCH - create a BVI loopback interface for B2 (extension to the cCMTS topology)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/loop-bvi2 '
{
  "name": "loop-bvi2",
  "enabled": true,
  "mtu": 1500,
  "ip_addresses": [
    "10.10.1.1/24"
  ]
}
'

# VSWITCH - add static route to 6.0.0.0/24 via GigabitEthernet0/8/0
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/vrf/0/fib/6.0.0.0/24/8.42.0.1 '
{
  "description": "Static route",
  "dst_ip_addr": "6.0.0.0/24",
  "next_hop_addr": "8.42.0.1",
  "outgoing_interface": "GigabitEthernet0/8/0"
}
'

# VSWITCH - create memif master to RNG (bridge domain B2)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-rng '
{
  "name": "memif-to-rng",
  "type": 2,
  "enabled": true,
  "memif": {
    "master": true,
    "id": 1,
    "socket_filename": "/tmp/memif.sock"
  }
}
'

# RNG - create memif slave to VSWITCH
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${RNG_NAME}/vpp/config/v1/interface/memif-to-vswitch '
{
  "name": "memif-to-vswitch",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": false,
    "id": 1,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "10.10.1.4/24"
  ]
}
'


# VSWITCH - create memif master to USSCHED (bridge domain B2)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-ussched '
{
  "name": "memif-to-ussched",
  "type": 2,
  "enabled": true,
  "memif": {
    "master": true,
    "id": 2,
    "socket_filename": "/tmp/memif.sock"
  }
}
'

# USSCHED - create memif slave to VSWITCH
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${USSCHED_NAME}/vpp/config/v1/interface/memif-to-vswitch '
{
  "name": "memif-to-vswitch",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": false,
    "id": 2,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "10.10.1.3/24"
  ]
}
'

# VSWITCH - create memif to VNF 1 (bridge domain B1)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-vnf-1 '
{
  "name": "memif-to-vnf-1",
  "type": 2,
  "enabled": true,
  "memif": {
    "master": true,
    "id": 3,
    "socket_filename": "/tmp/memif.sock"
  }
}
'

# VNF - create memif slave 1 to VSWITCH
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VNF_NAME}/vpp/config/v1/interface/memif-to-vswitch-1 '
{
  "name": "memif-to-vswitch-1",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": false,
    "id": 3,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "10.10.1.2/24"
  ]
}
'

# VSWITCH - create memif to vnf 2 (bridge domain B2)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-vnf-2 '
{
  "name": "memif-to-vnf-2",
  "type": 2,
  "enabled": true,
  "memif": {
    "master": true,
    "id": 4,
    "socket_filename": "/tmp/memif.sock"
  }
}
'

# VNF - create memif slave 2 to VSWITCH
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VNF_NAME}/vpp/config/v1/interface/memif-to-vswitch-2 '
{
  "name": "memif-to-vswitch-2",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": false,
    "id": 4,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "166.111.8.2"
  ]
}
'

# VSWITCH - create bridge domain B2 (needs to be called after the interfaces have been created)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/bd/B2 '
{
  "name": "B2",
  "flood": true,
  "unknown_unicast_flood": true,
  "forward": true,
  "learn": true,
  "arp_termination": true,
  "interfaces": [
    {
      "name": "memif-to-rng"
    },
    {
      "name": "memif-to-ussched"
    },
    {
      "name": "memif-to-vnf-1"
    },
    {
      "name": "loop-bvi2",
      "bridged_virtual_interface": true
    }
  ]
}
'

# VSWITCH - create bridge domain B1 (needs to be called after the interfaces have been created)
docker exec -it ${ETCD_CONTAINER} etcdctl put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/bd/B1 '
{
  "name": "B1",
  "flood": true,
  "unknown_unicast_flood": true,
  "forward": true,
  "learn": true,
  "interfaces": [
    {
      "name": "memif-to-vnf-2"
    },
    {
      "name": "vxlan1"
    }
  ]
}
'
