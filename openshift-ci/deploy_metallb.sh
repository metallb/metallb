#!/usr/bin/bash

metallb_dir="$(dirname $(readlink -f $0))"
source ${metallb_dir}/common.sh

METALLB_OPERATOR_REPO=${METALLB_OPERATOR_REPO:-"https://github.com/openshift/metallb-operator.git"}
METALLB_OPERATOR_BRANCH=${METALLB_OPERATOR_BRANCH:-"main"}
METALLB_IMAGE_BASE=${METALLB_IMAGE_BASE:-$(echo "${OPENSHIFT_RELEASE_IMAGE}" | sed -e 's/release/stable/g' | sed -e 's/@.*$//g')}
METALLB_IMAGE_TAG=${METALLB_IMAGE_TAG:-"metallb"}
METALLB_OPERATOR_IMAGE_TAG=${METALLB_OPERATOR_IMAGE_TAG:-"metallb-operator"}
FRR_IMAGE_TAG=${FRR_IMAGE_TAG:-"metallb-frr"}
export NAMESPACE=${NAMESPACE:-"metallb-system"}

if [ ! -d ./metallb-operator ]; then
	git clone ${METALLB_OPERATOR_REPO}
	cd metallb-operator
	git checkout ${METALLB_OPERATOR_BRANCH}
	cd -
fi
cd metallb-operator

# install yq v4 for metallb deployment
go install -mod='' github.com/mikefarah/yq/v4@v4.13.3

yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="SPEAKER_IMAGE").value|="'${METALLB_IMAGE_BASE}':'${METALLB_IMAGE_TAG}'"' ${metallb_dir}/metallb-operator-deploy/controller_manager_patch.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="CONTROLLER_IMAGE").value|="'${METALLB_IMAGE_BASE}':'${METALLB_IMAGE_TAG}'"' ${metallb_dir}/metallb-operator-deploy/controller_manager_patch.yaml
yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="FRR_IMAGE").value|="'${METALLB_IMAGE_BASE}':'${FRR_IMAGE_TAG}'"' ${metallb_dir}/metallb-operator-deploy/controller_manager_patch.yaml

PATH="${GOPATH}:${PATH}" ENABLE_OPERATOR_WEBHOOK=true KUSTOMIZE_DEPLOY_DIR="../metallb-operator-deploy" IMG="${METALLB_IMAGE_BASE}:${METALLB_OPERATOR_IMAGE_TAG}" make deploy

oc apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
EOF

oc adm policy add-scc-to-user privileged -n metallb-system -z speaker

sudo ip route add 192.168.10.0/24 dev ${BAREMETAL_NETWORK_NAME}
