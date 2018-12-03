#!/bin/bash

etcdKey=""
while read line || [[ -n "$line" ]]; do
    if [ "${line:0:1}" != "/" ] ; then
        value="$(echo "$line"|tr -d '\r\n')"
        kubectl exec etcd-server etcdctl -- put --endpoints=[127.0.0.1:22379] $etcdKey "$value"
    else
        etcdKey="$(echo "$line"|tr -d '\r\n')"
    fi
done < "$1"
