#!/bin/bash

set -euxo pipefail

CONTROLLER_HOSTNAME="$(hostname | cut -f1 -d'-')-controller"

cat >/tmp/kubeadm.conf <<EOF
apiVersion: kubeadm.k8s.io/v1alpha3
kind: JoinConfiguration
token: "000000.0000000000000000"
discoveryTokenUnsafeSkipCAVerification: true
discoveryTokenAPIServers:
- ${CONTROLLER_HOSTNAME}.local:6443
nodeRegistration:
  kubeletExtraArgs:
    node-ip: $(head -1 /host/ip)
EOF
kubeadm join --config=/tmp/kubeadm.conf
#kubeadm join --token=000000.0000000000000000 --discovery-token-unsafe-skip-ca-verification ${CONTROLLER_HOSTNAME}.local:6443
