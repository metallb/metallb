#!/bin/bash

echo "***********************************************************************"
echo "*** If you are running this script to troubleshoot an issue, please ***"
echo "*** consider setting the MetalLB loglevel to debug                  ***"
echo "***********************************************************************"

METALLB_NS="${METALLB_NS:-metallb-system}"
OUTPUT="./report"

function get_metallb_crs() {
    declare -a METALLB_CRDS=("bgppeers" "bfdprofiles" "bgpAdvertisements" "ipaddresspools" "l2advertisements" "communities")
    mkdir -p ${OUTPUT}/crds/

    for CRD in "${METALLB_CRDS[@]}"; do
        kubectl get "${CRD}" -n ${METALLB_NS} -o yaml > "${OUTPUT}/crds/${CRD}.yaml"
    done
}

function gather_frr_logs() {
    declare -a FILES_TO_GATHER=("frr.conf" "frr.log" "daemons" "vtysh.conf")
    declare -a COMMANDS=("show running-config" "show bgp ipv4" "show bgp ipv6" "show bgp neighbor" "show bfd peer")
    LOGS_DIR="${OUTPUT}/pods/${1}/frr/logs"
    mkdir -p ${LOGS_DIR}

    for FILE in "${FILES_TO_GATHER[@]}"; do
        kubectl -n ${METALLB_NS} exec ${1} -c frr -- sh -c "cat /etc/frr/${FILE}" > ${LOGS_DIR}/${FILE}
    done

    for COMMAND in "${COMMANDS[@]}"; do
        echo "###### ${COMMAND}" >> ${LOGS_DIR}/dump_frr
        echo "$( kubectl -n ${METALLB_NS} exec ${1} -c frr -- vtysh -c "${COMMAND}")" >> ${LOGS_DIR}/dump_frr
    done
}

echo "collecting crds.."
get_metallb_crs

# speaker pods are responsible for IP advertisement
SPEAKER_PODS="${@:-$(kubectl -n ${METALLB_NS} get pods -l component=speaker -o jsonpath='{.items[*].metadata.name}')}"
FIRST_SPEAKER=$(echo $SPEAKER_PODS | cut -d" " -f1)

SPEAKER_CONTAINERS=$(kubectl get pod ${FIRST_SPEAKER} -n ${METALLB_NS} -o jsonpath='{.spec.containers[*].name}')

FRR_MODE=false

if [[ $SPEAKER_CONTAINERS == *"frr"* ]]; then
    echo "FRR mode detected"
    FRR_MODE=true
fi

for SPEAKER_POD in ${SPEAKER_PODS[@]}; do
    echo "collecting logs for $SPEAKER_POD.."

    mkdir -p ${OUTPUT}/pods/${SPEAKER_POD}
    kubectl -n ${METALLB_NS} logs ${SPEAKER_POD} -c speaker > ${OUTPUT}/pods/${SPEAKER_POD}/speaker.log
    if [ "$FRR_MODE" = true ]; then    
        gather_frr_logs ${SPEAKER_POD}
        kubectl -n ${METALLB_NS} logs ${SPEAKER_POD} -c reloader > ${OUTPUT}/pods/${SPEAKER_POD}/reloader.log
        kubectl -n ${METALLB_NS} logs ${SPEAKER_POD} -c frr > ${OUTPUT}/pods/${SPEAKER_POD}/frr.log
        kubectl -n ${METALLB_NS} logs ${SPEAKER_POD} -c frr-metrics > ${OUTPUT}/pods/${SPEAKER_POD}/frr-metrics.log
    fi
done 

CONTROLLER_PODS="${@:-$(kubectl -n ${METALLB_NS} get pods -l component=controller -o jsonpath='{.items[*].metadata.name}')}"

for CONTROLLER in ${CONTROLLER_PODS[@]}; do
    echo "collecting logs for $CONTROLLER.."

    mkdir -p ${OUTPUT}/pods/${CONTROLLER}
    kubectl -n ${METALLB_NS} logs ${CONTROLLER} -c controller > ${OUTPUT}/pods/${CONTROLLER}/controller.log
done

tar cfvz metallb_report.tgz ${OUTPUT}
rm -rf ${OUTPUT}
