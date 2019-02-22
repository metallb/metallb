#!/bin/sh

./vpp-agent-ctl /etc/etcd.conf  -del  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl1
./vpp-agent-ctl /etc/etcd.conf  -del  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl2
./vpp-agent-ctl /etc/etcd.conf  -del  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl3
#./vpp-agent-ctl /etc/etcd.conf  -del  /vnf-agent/lubuntu1/vpp/config/v1/acl/acl4
