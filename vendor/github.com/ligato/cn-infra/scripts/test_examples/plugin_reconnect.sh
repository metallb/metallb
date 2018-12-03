#!/usr/bin/env bash
source scripts/test_examples/docker_start_stop_functions.sh

# In this file there are tested the CN-infra examples.
# At the present time only examples/simple-agent/simple-agent is tested here.

# The executed example does not stop itself (it will be killed at the end of
# this file). This is to alow to process an output of executed example several
# times to monitor reactions of executed example to some outer influences
# (e.g. stop/start of some services executed before the tested example.)

TMP_FILE="/tmp/out"
TMP_FILE2="/tmp/unprocessed"
exitCode=0
PREV_IFS="$IFS"

# tests whether the output of the command contains expected lines
# arguments
# 1st command to run
# 2nd array of expected strings in the command output
# 3rd argument is an optional array of unexpected strings in the command output
function testExpectedMessage {
IFS="
"
    # loop through expected lines
    for i in $2; do
        if grep -- "${i}" "$TMP_FILE2" > /dev/null ; then
            echo "OK - '$i'"
        else
            echo "Not found - '$i'"
            rv=1
        fi
    done

    # loop through unexpected lines
    if [[ ! -z $3 ]] ; then
        for i in $3; do
            if grep -- "${i}" "$TMP_FILE2" > /dev/null ; then
                echo "IS NOT OK - '$i'"
                rv=1
            fi
        done
    fi

    # if an error occurred print the output
    if [[ ! $rv -eq 0 ]] ; then
        cat ${TMP_FILE2}
        exitCode=1
    fi
}

# tests whether the output of the command contains expected lines
# arguments
# 1st command to run
# 2nd array of expected strings in the command output
# 3th argument is an optional array of unexpected strings in the command output
function testOutput {
    IFS="$PREV_IFS"

    #run the command
    if [ -e ${TMP_FILE} ]; then
        # if exists file /tmp/out we assume that the command still runs, do not start it again
        echo "Continue:"
        echo "Testing $1"
        # The command continues to run - the output is still redirected to the file ${TMP_FILE}
    else
        echo "Start:"
        echo "Testing $1"

        $1 > $TMP_FILE 2>&1 &
        CMD_PID=$!
    fi

    # this delay is for leaving enough time for establishing in-between communications after testing event occurs
    sleep 20

    # all unprocessed lines will be copied to the $TMP_FILE2
    cat $TMP_FILE | awk "NR > $processedLines" > $TMP_FILE2

    rv=0
    # let us examine unprocessed lines in $TMP_FILE2
    testExpectedMessage "$1" "$2" "$3" # uses $TMP_FILE2
    echo "##$rv" # function testExpectedMessage modifies this variable ...
    echo "----------------------------------------------------------------"

    # this was processed till now
    processedLines=`wc -l ${TMP_FILE} | cut --delimiter=" " -f1`
    return $exitCode
}

#### Simple-agent with Cassandra and Redis and Kafka and ETCD ####################################

processedLines=0
rm ${TMP_FILE} > /dev/null 2>&1
rm ${TMP_FILE2} > /dev/null 2>&1

startEtcd
startCustomizedKafka examples/kafka-plugin/manual-partitioner/server.properties
startRedis
startCassandra

expected=("Plugin etcd: status check probe registered
Plugin redis: status check probe registered
Plugin cassandra: status check probe registered
Plugin kafka: status check probe registered
All plugins initialized successfully
Agent plugin state update.*plugin=etcd state=ok
Agent plugin state update.*plugin=redis state=ok
Agent plugin state update.*plugin=cassandra state=ok
Agent plugin state update.*plugin=kafka state=ok
")

unexpected=("Redis config not found, skip loading this plugin
cassandra client config not found  - skip loading this plugin
")

cmd="examples/simple-agent/simple-agent --etcd-config=examples/etcd-lib/etcd.conf --kafka-config=examples/kafka-plugin/manual-partitioner/kafka.conf  --redis-config=examples/redis-lib/node-client.yaml --cassandra-config=examples/cassandra-lib/client-config.yaml"
testOutput "${cmd}" "${expected}" "${unexpected}"
# the cmd continues to run - PID of the process is stored in the variable CMD_PID
# we will kill it later after testing of all events ...

# redis start/stop test
echo "Redis is stopped - etcdctl output:"
stopRedis >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/redis
echo

expected=("Agent plugin state update.*Get(/probe-redis-connection) failed: EOF.*status-check.*plugin=redis state=error
")

unexpected=("Agent plugin state update.*plugin=redis state=ok
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT disconnected

echo "Redis is started - etcdctl output:"
startRedis >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/redis
echo

expected=("Agent plugin state update.*plugin=redis state=ok
")

unexpected=("Agent plugin state update.*Get(/probe-redis-connection) failed: EOF.*status-check.*plugin=redis state=error
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT connected AGAIN

# cassandra start/stop test
echo "Cassandra is stopped - etcdctl output:"
stopCassandra >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/cassandra
echo

expected=("Agent plugin state update.*gocql: no hosts available in the pool.*status-check plugin=cassandra state=error
")

unexpected=("Agent plugin state update.*plugin=cassandra state=ok
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT disconnected

echo "Cassandra is started - etcdctl output:"
startCassandra >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/cassandra
echo

expected=("Agent plugin state update.*plugin=cassandra state=ok
")

unexpected=("Agent plugin state update.*gocql: no hosts available in the pool.*status-check plugin=cassandra state=error
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT connected AGAIN

# kafka start/stop test
echo "Kafka is stopped - etcdctl output:"
stopKafka >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/kafka
echo

expected=("Agent plugin state update.*kafka: client has run out of available brokers to talk to (Is your cluster reachable?).*status-check plugin=kafka state=error
")

unexpected=("Agent plugin state update.*plugin=kafka state=ok
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT disconnected

echo "Kafka is started - etcdctl output:"
startKafka >> /dev/null
sleep 3
docker exec -it etcd etcdctl get /vnf-agent/vpp1/check/status/v1/plugin/kafka
echo

expected=("Agent plugin state update.*plugin=kafka state=ok
")

unexpected=("Agent plugin state update.*kafka: client has run out of available brokers to talk to (Is your cluster reachable?).*status-check plugin=kafka state=error
")

testOutput "${cmd}" "${expected}" "${unexpected}"  # cmd unchanged - ASSERT connected AGAIN

if ps -p $CMD_PID > /dev/null; then
    kill $CMD_PID
    echo "Killed $1 (SIGTERM)."
    sleep 3
    if ps -p $CMD_PID > /dev/null; then
        kill -9 $CMD_PID
        echo "Killed $1 (SIGKILL)."
    fi
fi

rm ${TMP_FILE} > /dev/null
rm ${TMP_FILE2} > /dev/null

stopEtcd
stopKafka
stopRedis
stopCassandra

echo "================================================================"


##########################################################################

exit ${exitCode}
