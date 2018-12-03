#!/usr/bin/env bash

VSWITCH_NAME="vpp1"
RPD_NAME="rpd"

#
#          memif: 8.42.0.2       memif: 8.42.0.1
# +---------------+                   +---------------+
# |               |                   |               |
# |               |                   |               |
# |    VSWITCH    +-------------------+      RPD      |
# |               |                   |               |
# |               |                   |               |
# +---------------+                   +---------------+
# route: 112.1.1.3/32 via 8.42.0.1                 loop: 112.1.1.3
#

# VSWITCH - add static route to 112.1.1.3/32 via 8.42.0.1
vpp-agent-ctl -put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/vrf/0/fib/112.1.1.3/32/8.42.0.1 - << EOF
{
  "description": "Static route",
  "dst_ip_addr": "112.1.1.3/32",
  "next_hop_addr": "8.42.0.1"
}
EOF

# VSWITCH - create memif master to RPD
vpp-agent-ctl -put /vnf-agent/${VSWITCH_NAME}/vpp/config/v1/interface/memif-to-rpd - << EOF
{
  "name": "memif-to-rpd",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": true,
    "key": 1,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "8.42.0.2/24"
  ]
}
EOF

# RPD - create memif slave to VSWITCH
vpp-agent-ctl -put /vnf-agent/${RPD_NAME}/vpp/config/v1/interface/memif-to-vswitch - << EOF
{
  "name": "memif-to-vswitch",
  "type": 2,
  "enabled": true,
  "mtu": 1500,
  "memif": {
    "master": false,
    "key": 1,
    "socket_filename": "/tmp/memif.sock"
  },
  "ip_addresses": [
    "8.42.0.1/24"
  ]
}
EOF

# RPD - create a loopback interface
vpp-agent-ctl -put /vnf-agent/${RPD_NAME}/vpp/config/v1/interface/loop1 - << EOF
{
  "name": "loop1",
  "enabled": true,
  "mtu": 1500,
  "phys_address": "8a:f1:be:90:00:dd",
  "ip_addresses": [
    "112.1.1.3/24"
  ]
}
EOF
