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

rm -rf metallb-operator-deploy/manifests
rm -rf metallb-operator-deploy/bundle
rm metallb-operator-deploy/bundleci.Dockerfile

cp metallb-operator/bundleci.Dockerfile metallb-operator-deploy 
cp -r metallb-operator/manifests/ metallb-operator-deploy/manifests 
cp -r metallb-operator/bundle/ metallb-operator-deploy/bundle 

cd metallb-operator-deploy

ESCAPED_METALLB_IMAGE=$(printf '%s\n' "${METALLB_IMAGE_BASE}:${METALLB_IMAGE_TAG}" | sed -e 's/[]\/$*.^[]/\\&/g');
find . -type f -name "*clusterserviceversion*.yaml" -exec sed -i 's/quay.io\/openshift\/origin-metallb:.*$/'"$ESCAPED_METALLB_IMAGE"'/g' {} +
ESCAPED_FRR_IMAGE=$(printf '%s\n' "${METALLB_IMAGE_BASE}:${FRR_IMAGE_TAG}" | sed -e 's/[]\/$*.^[]/\\&/g');
find . -type f -name "*clusterserviceversion*.yaml" -exec sed -i 's/quay.io\/openshift\/origin-metallb-frr:.*$/'"$ESCAPED_FRR_IMAGE"'/g' {} +
ESCAPED_OPERATOR_IMAGE=$(printf '%s\n' "${METALLB_IMAGE_BASE}:${METALLB_OPERATOR_IMAGE_TAG}" | sed -e 's/[]\/$*.^[]/\\&/g');
find . -type f -name "*clusterserviceversion*.yaml" -exec sed -i 's/quay.io\/openshift\/origin-metallb-operator:.*$/'"$ESCAPED_OPERATOR_IMAGE"'/g' {} +
find . -type f -name "*clusterserviceversion*.yaml" -exec sed -r -i 's/name: metallb-operator\..*$/name: metallb-operator.v0.0.0/g' {} +

cd -

secret=$(oc -n openshift-marketplace get sa builder -oyaml | grep imagePullSecrets -A 1 | grep -o "builder-.*")

buildindexpod="apiVersion: v1
kind: Pod
metadata:
  name: buildindex
  namespace: openshift-marketplace
spec:
  restartPolicy: Never
  serviceAccountName: builder
  containers:
    - name: priv
      image: quay.io/podman/stable
      command:
        - /bin/bash
        - -c
        - |
          set -xe
          sleep INF
      securityContext:
        privileged: true
      volumeMounts:
        - mountPath: /var/run/secrets/openshift.io/push
          name: dockercfg
          readOnly: true
  volumes:
    - name: dockercfg
      defaultMode: 384
      secret:
        secretName: $secret
"

echo "$buildindexpod" | oc apply -f -

success=0
iterations=0
sleep_time=10
max_iterations=72 # results in 12 minutes timeout
until [[ $success -eq 1 ]] || [[ $iterations -eq $max_iterations ]]
do
  run_status=$(oc -n openshift-marketplace get pod buildindex -o json | jq '.status.phase' | tr -d '"')
   if [ "$run_status" == "Running" ]; then
          success=1
          break
   fi
   iterations=$((iterations+1))
   sleep $sleep_time
done

oc cp metallb-operator-deploy openshift-marketplace/buildindex:/tmp
oc exec -n openshift-marketplace buildindex /tmp/metallb-operator-deploy/build_and_push_index.sh

oc apply -f metallb-operator-deploy/install-resources.yaml

# there is a race in the creation of the pod and the service account that prevents
# the index image to be pulled. Here we check if the pod is not running and we kill it. 
success=0
iterations=0
sleep_time=10
max_iterations=72 # results in 12 minutes timeout
until [[ $success -eq 1 ]] || [[ $iterations -eq $max_iterations ]]
do
  run_status=$(oc -n openshift-marketplace get pod | grep metallbindex | awk '{print $3}')
   if [ "$run_status" == "Running" ]; then
          success=1
          break
   elif [[ "$run_status" == *"Image"*  ]]; then
       echo "pod in bad status try to recreate the image again status: $run_status"
       pod_name=$(oc -n openshift-marketplace get pod | grep metallbindex | awk '{print $1}')
       oc -n openshift-marketplace delete po $pod_name
   fi
   iterations=$((iterations+1))
   sleep $sleep_time
done

if [[ $success -eq 1 ]]; then
  echo "[INFO] index image pod running"
else
  echo "[ERROR] index image pod failed to run"
  exit 1
fi

./wait-for-csv.sh

oc apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: MetalLB
metadata:
  name: metallb
  namespace: metallb-system
spec:
  logLevel: debug
EOF

NAMESPACE="metallb-system"
ATTEMPTS=0

while [[ -z $(oc get endpoints -n $NAMESPACE webhook-service -o jsonpath="{.subsets[0].addresses}" 2>/dev/null) ]]; do
  echo "still waiting for webhookservice endpoints"
  sleep 10
  (( ATTEMPTS++ ))
  if [ $ATTEMPTS -eq 30 ]; then
        echo "failed waiting for webhookservice endpoints"
        exit 1
  fi
done
echo "webhook endpoints avaliable"


sudo ip route add 192.168.10.0/24 dev ${BAREMETAL_NETWORK_NAME}
