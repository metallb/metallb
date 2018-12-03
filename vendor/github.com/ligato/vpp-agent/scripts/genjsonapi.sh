#!/bin/bash

VPPDIR=${VPPDIR:-/opt/vpp}
APIDIR=${APIDIR:-/usr/share/vpp/api}

[ -d "${VPPDIR}" ] || {
    echo >&2 "vpp directory not found at: ${VPPDIR}";
    exit 1;
}
mkdir -p ${APIDIR}

find ${VPPDIR} -name \*.api -printf "echo %p - ${APIDIR}/%f.json \
    && ${VPPDIR}/src/tools/vppapigen/vppapigen --includedir ${VPPDIR}/src \
    --input %p --output ${APIDIR}/%f.json JSON\n" | xargs -0 sh -c
