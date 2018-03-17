#!/bin/bash

set -e

PROTOCOL=$1
MACHINE_IP=$2
NUM_CLIENTS=$3

case $PROTOCOL in
    ipv4)
        ENABLE=bird
        DISABLE=bird6
        CONFIG=/etc/bird/bird.conf
    ;;
    ipv6)
        ENABLE=bird6
        DISABLE=bird
        CONFIG=/etc/bird/bird6.conf
    ;;
    *)
        echo "Bad command"
        exit 1
        ;;
esac

systemctl stop $DISABLE
systemctl disable $DISABLE

cat >$CONFIG <<EOF
router id 1.2.3.4;
log stderr all;
debug protocols all;
protocol device {
}
protocol kernel {
  export all;
}
EOF

for i in `seq 1 $NUM_CLIENTS`; do
    PEER_IP=${MACHINE_IP%?}$((2+i))
    cat >>$CONFIG <<EOF
protocol bgp peer${i} {
  local $MACHINE_IP as 64512;
  neighbor $PEER_IP as 64512;
  passive;
  error wait time 1, 2;
}
EOF
done

systemctl restart $ENABLE
