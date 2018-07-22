#!/usr/bin/env bash

# List of functions which should not be used with remarkable reasons
FUNCS=(
# On a 32-bit platform, int type is not big enough to convert into uint32 type.
# strconv.Atoi() should be replaced by strconv.ParseUint() or
# strconv.ParseInt().
'strconv\.Atoi'
)

SCRIPT_DIR=`dirname $0`

RESULT=0

for FUNC in ${FUNCS[@]}
do
    for GO_PKG in $(go list github.com/osrg/gobgp/... | grep -v '/vendor/')
    do
        grep ${FUNC} -r ${GOPATH}/src/${GO_PKG}
        if [ $? -ne 1 ]
        then
            RESULT=1
        fi
    done
done

exit $RESULT