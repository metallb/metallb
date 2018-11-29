#!/bin/bash

set -euxo pipefail

VERSION="v1.12.2"

cat >/tmp/kubeadm.conf <<EOF
apiVersion: kubeadm.k8s.io/v1alpha3
kind: InitConfiguration
bootstrapTokens:
- token: "000000.0000000000000000"
  ttl: "24h"
apiEndpoint:
  advertiseAddress: $(head -1 /host/ip)
nodeRegistration:
  kubeletExtraArgs:
    node-ip: $(head -1 /host/ip)
---
apiVersion: kubeadm.k8s.io/v1alpha3
kind: ClusterConfiguration
networking:
  podSubnet: "10.42.0.0/16"
kubernetesVersion: "${VERSION}"
clusterName: "virtuakube"
apiServerCertSANs:
- "127.0.0.1"
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
resolvConf: /run/systemd/resolve/resolv.conf
EOF
echo "export KUBECONFIG=/etc/kubernetes/admin.conf" >/etc/profile.d/k8s.sh
kubeadm init --config=/tmp/kubeadm.conf
export KUBECONFIG=/etc/kubernetes/admin.conf
kubectl taint nodes --all node-role.kubernetes.io/master-
kubectl apply -f /host/addons.yaml
cp $KUBECONFIG /host/kubeconfig.tmp
mv -f /host/kubeconfig.tmp /host/kubeconfig
