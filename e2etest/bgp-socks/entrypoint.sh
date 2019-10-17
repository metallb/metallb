#!/bin/bash

set -euxo pipefail

MY_IP=$(ip -br addr show eth0 | awk '{print $3}' | cut -f1 -d'/')
echo $MY_IP >/ip

function bgp() {
    mkdir /run/bird
    
    BIRD_CONF=/etc/bird/bird.conf

    cat >$BIRD_CONF <<EOF
router id $MY_IP;

protocol kernel {
  scan time 10;
  import none;
  export all;
}

protocol device {
  scan time 10;
}

EOF
    idx=1
    for ip in ${IPS:-}; do
        cat >>$BIRD_CONF <<EOF
protocol bgp bgp$idx {
  local as 64512;
  neighbor $ip as 64513;
  next hop self;
  passive;
  import all;
  export none;
}

EOF
        idx=$((idx+1))
    done

    cat >>/etc/supervisor/supervisord.conf <<EOF
[program:bgp]
command=/usr/sbin/bird -d -c /etc/bird/bird.conf
user=root

EOF
}

function socks() {
    cat >/etc/danted.conf <<EOF
logoutput: stderr
internal: eth0 port = 1080
external: eth0
socksmethod: username none
clientmethod: none
user.privileged: root
user.unprivileged: root
client pass {
  from: 0.0.0.0/0 to: 0.0.0.0/0
  log: connect error
}
socks pass {
  from: 0.0.0.0/0 to: 0.0.0.0/0
  command: bind connect udpassociate
  log: error
}
EOF

    cat >>/etc/supervisor/supervisord.conf <<EOF
[program:socks]
command=/usr/sbin/danted -f /etc/danted.conf
user=root

EOF
}

if [[ -n ${RUN_BGP:-} ]]; then
    bgp
fi
if [[ -n ${RUN_SOCKS:-} ]]; then
    socks
fi

exec supervisord -c /etc/supervisor/supervisord.conf -n
