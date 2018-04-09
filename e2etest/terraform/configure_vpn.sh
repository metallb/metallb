#!/bin/bash

set -e

function config_vpn() {
    PORT=$1
    REMOTE=$2
    cat >/etc/openvpn/${PORT}.conf <<EOF
port ${PORT}
proto udp
dev tun${PORT}
dev-type tap
keepalive 10 60
comp-lzo no
persist-key
persist-tun
verb 3
mute-replay-warnings
EOF
    if [[ ! -z $REMOTE ]]; then
        echo "remote $REMOTE" >>/etc/openvpn/${PORT}.conf
    fi
    systemctl enable openvpn@${PORT}.service
    systemctl start openvpn@${PORT}.service
}

function mk_switch() {
    SWITCH_NUM=$1
    NUM_MACHINES=$2
    
    ip link add br${SWITCH_NUM} type bridge
    ip link set br${SWITCH_NUM} up

    for i in `seq 1 $NUM_MACHINES`; do
        PORT=$((SWITCH_NUM*2000+i))
        ip tuntap add dev tun${PORT} mode tap
        ip link set dev tun${PORT} master br${SWITCH_NUM} mtu 1280 up
        config_vpn ${PORT}
    done
}

function mk_access() {
    SWITCH_NUM=$1
    ADDR=$2
    NETMASK=$3
    REMOTE=$4
    SUFFIX=$(echo $ADDR | tr ':.' '  ' | awk '{print $NF}')
    PORT=$((SWITCH_NUM*2000+SUFFIX))
    ip tuntap add dev tun${PORT} mode tap
    ip addr add ${ADDR}/${NETMASK} dev tun${PORT}
    ip link set dev tun${PORT} mtu 1280 up
    config_vpn $PORT $REMOTE
}

apt -qq -y install openvpn

case $1 in
    switch)
        mk_switch $2 $3
        ;;
    access)
        mk_access $2 $3 $4 $5
        ;;
    *)
        echo "usage: $0 switch <switch-num> <num-machines>"
        echo "usage: $0 access <switch-num> <local-ip> <local-netmask> <remote-ip>"
        exit 1
        ;;
esac
