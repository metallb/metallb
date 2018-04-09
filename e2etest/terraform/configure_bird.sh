#!/bin/bash

set -e

PROTOCOL=$1
NUM_CLIENTS=$2
shift 2

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

C=0
while [[ $# > 0 ]]; do
    MACHINE_IP=$1
    shift
    for i in `seq 1 $NUM_CLIENTS`; do
        PEER_IP=${MACHINE_IP%?}$((2+i))
        cat >>$CONFIG <<EOF
protocol bgp peer${C} {
  local $MACHINE_IP as 64512;
  neighbor $PEER_IP as 64512;
  passive;
  error wait time 1, 2;
}
EOF
        C=$((C+1))
    done
done

systemctl restart $ENABLE
