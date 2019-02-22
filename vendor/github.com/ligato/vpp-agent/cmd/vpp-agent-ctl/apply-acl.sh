#!/bin/sh

./vpp-agent-ctl /etc/etcd.conf  -put  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl1 json/acl-allow.json
./vpp-agent-ctl /etc/etcd.conf  -put  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl2 json/acl-deny.json
#./vpp-agent-ctl /etc/etcd.conf  -put  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl3 json/acl-allow-1.3.json
./vpp-agent-ctl /etc/etcd.conf  -put  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl4 json/acl-allow-slave.json
