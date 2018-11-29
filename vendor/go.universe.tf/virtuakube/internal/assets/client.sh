#!/bin/bash

set -euxo pipefail

trap "touch /host/bootscript-done" EXIT

IP=192.168.50.3
ip addr add ${IP}/24 dev ens4
hostnamectl set-hostname client

cat >/etc/bird/bird.conf <<EOF
router id ${IP};

protocol kernel {
  persist;
  merge paths;
  scan time 60;
  import none;
  export all;
}

protocol device {
  scan time 60;
}

protocol direct {
  interface "ens4";
  check link;
  import all;
}

protocol bgp k8smaster {
  local as 64513;
  passive;
  neighbor 192.168.50.1 as 64512;
  next hop self;
  import all;
  export none;
}

protocol bgp k8snode {
  local as 64513;
  passive;
  neighbor 192.168.50.2 as 64512;
  next hop self;
  import all;
  export none;
}
EOF
systemctl restart bird
