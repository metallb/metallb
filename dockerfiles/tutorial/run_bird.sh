#!/bin/bash

# Connections from the BGP speaker pod to the router pod will use pod
# networking exclusively. This is a problem, because to simulate a
# real deployment, we want the BGP session to come from the node's IP,
# not the pod's IP.
#
# Fortunately, iptables to the rescue! We can just source NAT incoming
# connections to the node IP.
iptables -t nat -A INPUT -p tcp --dport 1179 -j SNAT --to $METALLB_NODE_IP

cat >/etc/bird/bird.conf <<EOF
router id 10.0.0.100;
listen bgp port 1179;
protocol device {
}
protocol static {
  route ${METALLB_NODE_IP}/32 via "eth0";
}
protocol bgp minikube {
  local 10.0.0.100 as 64512;
  neighbor $METALLB_NODE_IP as 64512;
  passive;
}
EOF

/bird_spy &

mkdir -p /run/bird
bird -d -c /etc/bird/bird.conf

# testing for now, remove later
sleep 36000
