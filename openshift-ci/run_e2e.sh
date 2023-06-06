#!/usr/bin/bash

metallb_dir="$(dirname $(readlink -f $0))"
source ${metallb_dir}/common.sh

METALLB_REPO=${METALLB_REPO:-"https://github.com/openshift/metallb.git"}
BACKWARD_COMPATIBLE_RELEASE=${BACKWARD_COMPATIBLE_RELEASE:-"release-4.10"}

# add firewalld rules
sudo firewall-cmd --zone=libvirt --permanent --add-port=179/tcp
sudo firewall-cmd --zone=libvirt --add-port=179/tcp
sudo firewall-cmd --zone=libvirt --permanent --add-port=180/tcp
sudo firewall-cmd --zone=libvirt --add-port=180/tcp
# BFD control packets
sudo firewall-cmd --zone=libvirt --permanent --add-port=3784/udp
sudo firewall-cmd --zone=libvirt --add-port=3784/udp
# BFD echo packets
sudo firewall-cmd --zone=libvirt --permanent --add-port=3785/udp
sudo firewall-cmd --zone=libvirt --add-port=3785/udp
# BFD multihop packets
sudo firewall-cmd --zone=libvirt --permanent --add-port=4784/udp
sudo firewall-cmd --zone=libvirt --add-port=4784/udp

# need to skip L2 metrics / node selector test because the pod that's running the tests is not 
# same subnet of the cluster nodes, so the arp request that's done in the test won't work.
# Also, skip l2 interface selector as it's not supported d/s currently.
# Skip route injection after setting up speaker. FRR is not refreshed.
SKIP="L2 metrics|L2 Node Selector|L2-interface selector|MetalLB allows adding extra FRR configuration.*after"
if [ "${IP_STACK}" = "v4" ]; then
	SKIP="$SKIP|IPV6|DUALSTACK"
	export PROVISIONING_HOST_EXTERNAL_IPV4=${PROVISIONING_HOST_EXTERNAL_IP}
	export PROVISIONING_HOST_EXTERNAL_IPV6=1111:1:1::1
elif [ "${IP_STACK}" = "v6" ]; then
	SKIP="$SKIP|IPV4|DUALSTACK"
	export PROVISIONING_HOST_EXTERNAL_IPV6=${PROVISIONING_HOST_EXTERNAL_IP}
	export PROVISIONING_HOST_EXTERNAL_IPV4=1.1.1.1
elif [ "${IP_STACK}" = "v4v6" ]; then
	SKIP="$SKIP|IPV6"
	export PROVISIONING_HOST_EXTERNAL_IPV4=${PROVISIONING_HOST_EXTERNAL_IP}
	export PROVISIONING_HOST_EXTERNAL_IPV6=1111:1:1::1
fi
echo "Skipping ${SKIP}"

# Let's enforce failing when running the tests
set -e

pip3 install --user -r ./../dev-env/requirements.txt
# Install ginkgo CLI.
go install github.com/onsi/ginkgo/v2/ginkgo@v2.4.0
export PATH=${PATH}:${HOME}/.local/bin
export CONTAINER_RUNTIME=podman
export RUN_FRR_CONTAINER_ON_HOST_NETWORK=true
inv e2etest --kubeconfig=$(readlink -f ../../ocp/ostest/auth/kubeconfig) \
	--service-pod-port=8080 --system-namespaces="metallb-system" --skip-docker \
	--ipv4-service-range=192.168.10.0/24 --ipv6-service-range=fc00:f853:0ccd:e799::/124 \
	--prometheus-namespace="openshift-monitoring" \
	--local-nics="_" --node-nics="_" --skip="${SKIP}"

# This checks if conversion webhooks work and if metallb is compatible with the CRDs
# in the operator. We clone the 4.10 version of metallb and run the E2E tests in
# operator mode. We run few significative tests that cover all the crds.
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
EOF

git clone -b ${BACKWARD_COMPATIBLE_RELEASE} ${METALLB_REPO}

metallb_root=$(dirname $metallb_dir )
# We need to invert the order as deleting a used bfd profile is not allowed.
patch metallb/e2etest/pkg/config/update.go < "$metallb_root"/e2etest/backwardcompatible/patchfile

rm -rf e2etest # we want to make sure we are not running current e2e by mistake
cd metallb
FOCUS="\"L2.*should work for ExternalTrafficPolicy=Cluster\"\|\"BGP.*A service of protocol load balancer should work with.*IPV4 - ExternalTrafficPolicyCluster$\"\|\"BFD.*IPV4 - full params$\""
inv e2etest --kubeconfig=$(readlink -f ../../../ocp/ostest/auth/kubeconfig) \
	--service-pod-port=8080 --system-namespaces="metallb-system" --skip-docker \
	--ipv4-service-range=192.168.10.0/24 --ipv6-service-range=fc00:f853:0ccd:e799::/124 \
	--focus="${FOCUS}" --use-operator
