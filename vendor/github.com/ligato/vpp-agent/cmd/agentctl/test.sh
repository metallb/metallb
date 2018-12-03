#!/usr/bin/env bash

./agentctl -l vpp1 put int << EOF
{
  "name": "GigabitEthernet0/8/1",
  "type": 1,
  "enabled": true,
  "ip": {
    "enabled": true,
    "mtu": 1500,
    "address": [
      {
        "ip": "8.42.0.2",
        "prefix_length": 24
      }
    ]
  }
}
EOF

./agentctl put int vxlan -l vpp2 -n cvok --phy-addr aa:22:33:44:55:ff --ipv4-addr 10.20.30.157/32 arg1 \
    --ipv4-mtu 1500 --ipv4-enabled=true --src-addr 10.1.1.123 --dst-addr 10.2.1.145 --vni 5000

./agentctl put int veth -l vpp2 -n veth-1  --peer-if-name peerHost2

./agentctl put int veth -l vpp2 -name veth-2 --phy-addr aa:22:33:44:55:ff --ipv4-addr 10.20.30.157/32 arg1\
    --ipv4-mtu 1500 --ipv4-enabled=true --peer-if-name peerHost

./agentctl put i l -l vpp2 -n lpbk8 --ipv4-addr 10.20.30.46/32 --ipv4-mtu 1500 --ipv4-enabled=true

./agentctl put i --label vpp2 ethernet --name eth1 --phy-addr aa:bb:cc:dd:ee:ff --ipv4-addr 10.20.30.46/32 \
    --ipv4-mtu 1500 --ipv4-enabled=true

