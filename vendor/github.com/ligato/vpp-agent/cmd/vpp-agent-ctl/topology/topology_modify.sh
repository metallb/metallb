#!/usr/bin/env bash

VSWITCH_NAME="vpp1"
RNG_NAME="rng-vpp"
USSCHED_NAME="ussched-vpp"
VNF_NAME="vnf-vpp"

# VSWITCH - change IP & MAC of the loopback interface
vpp-agent-ctl -put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/loop1 - << EOF
{
  "name": "loop1",
  "enabled": true,
  "phys_address": "8a:f1:be:90:00:bb",
  "mtu": 1500,
  "ip_addresses": [
    "6.0.0.101/24"
  ]
}
EOF

# VSWITCH - delete memif master to RNG (bridge domain B2)
vpp-agent-ctl -del /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-rng

# RNG - delete memif slave to VSWITCH
vpp-agent-ctl -del /vnf-agent/${RNG_NAME}/vpp/config/v1/interface/memif-to-vswitch

# VSWITCH - add one more static route
vpp-agent-ctl -put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/vrf/0/fib/20.5.0.0/24/8.42.0.1 - << EOF
{
  "description": "Static route 2",
  "dst_ip_addr": "20.5.0.0/24",
  "next_hop_addr": "8.42.0.1",
  "outgoing_interface": "GigabitEthernet0/8/0"
}
EOF

# VSWITCH - remove deleted interface + BVI interface from bridge domain
vpp-agent-ctl -put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/bd/B2 - << EOF
{
  "name": "B2",
  "flood": true,
  "unknown_unicast_flood": true,
  "forward": true,
  "learn": true,
  "arp_termination": true,
  "interfaces": [
    {
      "name": "memif-to-ussched"
    },
    {
      "name": "memif-to-vnf-1"
    }
  ]
}
EOF
