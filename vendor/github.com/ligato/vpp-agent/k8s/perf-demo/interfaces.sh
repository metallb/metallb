#!/bin/bash

IF1="FortyGigabitEthernet89/0/0"
IF2="FortyGigabitEthernet89/0/1"

old_if1=${IF1//\//\\\/}
old_if2=${IF2//\//\\\/}

new_if1=${1//\//\\\/}
new_if2=${2//\//\\\/}

sed -i "s/${old_if1}/${new_if1}/g" ./scenario1/etcd.txt ./scenario2/etcd.txt ./scenario4/etcd.txt
sed -i "s/${old_if2}/${new_if2}/g" ./scenario1/etcd.txt ./scenario2/etcd.txt ./scenario4/etcd.txt
