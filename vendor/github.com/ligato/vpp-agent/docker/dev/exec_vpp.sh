#!/usr/bin/env bash

set -e

VPP_DIR=${VPP_DIR:-/opt/vpp-agent/dev/vpp}

if [ -n "$RUN_VPP_DEBUG" ]; then
    echo "Running VPP in DEBUG mode"
    exec ${VPP_DIR}/build-root/install-vpp_debug-native/vpp/bin/vpp -c /etc/vpp/vpp.conf
else
    echo "Running VPP in RELEASE mode"
    exec ${VPP_DIR}/build-root/install-vpp-native/vpp/bin/vpp -c /etc/vpp/vpp.conf
fi
