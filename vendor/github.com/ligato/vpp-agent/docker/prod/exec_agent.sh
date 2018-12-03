#!/usr/bin/env bash

set -e

if [ -n "$OMIT_AGENT" ]; then
    echo "Start of vpp-agent disabled (unset OMIT_AGENT to enable it)"
else
    echo "Starting vpp-agent.."
    exec vpp-agent --config-dir=/opt/vpp-agent/dev
fi
