#!/usr/bin/env bash
source scripts/test_examples/docker_start_stop_functions.sh

# In this file there are tested the CN-infra examples.
# These examples are located in the folder examples.
# The function for testing output of executed example - testOutput - can be used
# in two modes - depending on the way how executed exampes works:
# - the executed example will stop its run itself (no need to use 4th parameter)
# - the executed example does not stop itself (it has to be killed after some
#   time - for this is used the 4th parameter of the function testOutput).

TMP_FILE="/tmp/out"
exitCode=0
PREV_IFS="$IFS"
RUNTIME_LIMIT=5

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
        if grep -- "${i}" "$TMP_FILE" > /dev/null ; then
            echo "OK - '$i'"
        else
            echo "Not found - '$i'"
            rv=1
        fi
    done

    # loop through unexpected lines
    if [[ ! -z $3 ]] ; then
        for i in $3; do
            if grep -- "${i}" "$TMP_FILE" > /dev/null ; then
                echo "IS NOT OK - '$i'"
                rv=1
            fi
        done
    fi

    # if an error occurred print the output
    if [[ ! $rv -eq 0 ]] ; then
        cat ${TMP_FILE}
        exitCode=1
    fi
}

# kills the process started by the command if it runs beyond runtime limit
# tests whether the output of the command contains expected lines
# arguments
# 1st command to run
# 2nd array of expected strings in the command output
# 3rd argument is mandatory command runtime limit
# 4th argument is an optional array of unexpected strings in the command output
function testOutput {
    IFS="$PREV_IFS"
    echo "Testing $1"

    #run the command
    $1 > $TMP_FILE 2>&1 &
    CMD_PID=$!
    sleep $3

    rv=0
    if ps -p $CMD_PID > /dev/null; then
        kill $CMD_PID
        echo "Killed $1."
        sleep 3
        if ps -p $CMD_PID > /dev/null; then
            kill -9 $CMD_PID
            echo "Test $1 has not terminated before runtime limit."
            cat ${TMP_FILE}
            exitCode=1
        else
            testExpectedMessage "$1" "$2" "$4"
        fi
    else
        testExpectedMessage "$1" "$2" "$4"
    fi
    echo "##$rv" # function testExpectedMessage modifies this variable ...
    echo "================================================================"
    rm $TMP_FILE
    return $exitCode
}

# Here were functions which made start/stop various docker containers
# They were moved to the file docker_start_stop_functions.sh to avoid
# of duplicating of the same code.

#### Cassandra ###########################################################

startCassandra

expected=("Successfully written
Successfully queried
Successfully queried with AND
Successfully queried with Multiple AND
Successfully queried with IN
")

cmd="examples/cassandra-lib/cassandra-lib examples/cassandra-lib/client-config.yaml"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT

#### Configs #############################################################

expected=("Loaded plugin config - found external configuration examples/configs-plugin/example.conf
Plugin Config {Field1:external value, Sleep:0s}
")

cmd="examples/configs-plugin/configs-plugin --config-dir=examples/configs-plugin --example-config=example.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT

#### Datasync ############################################################

startEtcd

expected=("KeyValProtoWatcher subscribed
Write data to /vnf-agent/vpp1/api/v1/example/db/simple/index
Update data at /vnf-agent/vpp1/api/v1/example/db/simple/index
Event arrived to etcd eventHandler, key /vnf-agent/vpp1/api/v1/example/db/simple/index, update: false
Event arrived to etcd eventHandler, key /vnf-agent/vpp1/api/v1/example/db/simple/index, update: true
")

unexpected=("etcd/datasync example failed
")

cmd="examples/datasync-plugin/datasync-plugin --etcd-config=examples/datasync-plugin/etcd.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT

stopEtcd

#### Etcd-lib ##########################################################

startEtcd

expected=("Saving  /phonebook/Peter
")

cmd="examples/etcd-lib/editor/editor --cfg examples/etcd-lib/etcd.conf  put  Peter Company 0907"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT

stopEtcd

#### Flags-lib ###########################################################

expected=("Registering flags...
Printing flags...
testFlagString:'mystring'
testFlagInt:'1122'
testFlagInt64:'-3344'
testFlagUint:'112'
testFlagUint64:'7788'
testFlagBool:'true'
testFlagDur:'5s'
")

cmd="examples/flags-lib/flags-lib --ep-string mystring --ep-uint 112"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT

#### Kafka-lib ###########################################################

startKafka

expected=("Kafka connecting
Consuming started
Sync published
Message is stored in topic(test)/partition(0)/offset(1)
")

testOutput examples/kafka-lib/mux/mux "${expected}" $RUNTIME_LIMIT

stopKafka

#### Kafka-plugin manual-partitioner #####################################

startCustomizedKafka examples/kafka-plugin/manual-partitioner/server.properties

# Let us test the running with non-existant offset parameter
expected=("Offset: 18, message count: 0
Error loading core: plugin kafka: AfterInit error 'kafka server: The requested offset is outside the range of offsets maintained by the server for the given topic/partition.'
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf  --offsetMsg 18 --messageCount 0"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test the running without parameters - in example are generated 10 Kafka Messages to both topics but the consumed is only 5 messages from each topic beginning with offset 5
expected=("offset arg not set, using default value
messageCount arg not set, using default value
Offset: 0, message count: 10
All plugins initialized successfully
Sending 10 sync Kafka notifications (protobuf) ...
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '5', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '9', key: 'proto-key',
Sending 10 async Kafka notifications (protobuf) ...
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '0', key: 'async-proto-key'
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '4', key: 'async-proto-key'
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '5', key: 'async-proto-key'
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '9', key: 'async-proto-key'
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '5', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '9', key: 'async-proto-key',
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - in example no new messages generated - we are consummed all beginning with offset 5 till 9  (which were generated before)
expected=("offset arg not set, using default value
Offset: 0, message count: 0
All plugins initialized successfully
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '5', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '9', key: 'async-proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '5', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '9', key: 'proto-key',
Sending 0 sync Kafka notifications (protobuf) ...
Sending 0 async Kafka notifications (protobuf) ...
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd='examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf -messageCount=0'
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - in example one new message generated
expected=("offset arg not set, using default value
Offset: 0, message count: 1
All plugins initialized successfully
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '5', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '9', key: 'async-proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '5', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '9', key: 'proto-key',
Sending 1 sync Kafka notifications (protobuf) ...
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '10', key: 'proto-key',
Sending 1 async Kafka notifications (protobuf) ...
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '10', key: 'async-proto-key'
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '10', key: 'async-proto-key',
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd='examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf -messageCount=1'
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - in example one new message generated - with offset 11 for both topics and we display all messages from offset 8
expected=("Offset: 8, message count: 1
All plugins initialized successfully
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '8', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '9', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '10', key: 'async-proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '8', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '9', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '10', key: 'proto-key',
Sending 1 sync Kafka notifications (protobuf) ...
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '11', key: 'proto-key',
Sending 1 async Kafka notifications (protobuf) ...
Async message successfully delivered, topic 'example-async-topic', partition '2', offset '11', key: 'async-proto-key'
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '11', key: 'async-proto-key',
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '7', key: 'async-proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '7', key: 'proto-key',
")

cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf --messageCount 1 --offsetMsg 8"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - in example no new messages generated - we want to list all latest messages
expected=("Offset: -1, message count: 0
All plugins initialized successfully
Sending 0 sync Kafka notifications (protobuf) ...
Sending 0 async Kafka notifications (protobuf) ...
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

# this is not working  cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf --messageCount 0 -offsetMsg=\"latest\""
cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf --messageCount 0 -offsetMsg=latest"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - in example no new messages generated - we want to list all oldest messages
expected=("Offset: -2, message count: 0
All plugins initialized successfully
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '0', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '2', offset '11', key: 'async-proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '0', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '11', key: 'proto-key',
Sending 0 sync Kafka notifications (protobuf) ...
Sending 0 async Kafka notifications (protobuf) ...
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf --messageCount 0 -offsetMsg=oldest"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"


# Let us test - in example no new messages generated - wrong value of parameter offsetMsg
expected=("
Error loading core: plugin kafka-example: Init error 'incorrect sync offset value wronginput
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/manual-partitioner/manual-partitioner --kafka-config examples/kafka-plugin/manual-partitioner/kafka.conf --messageCount 0 -offsetMsg=wronginput"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

stopKafka

#### Kafka-plugin hash-partitioner #######################################

RUNTIME_LIMIT=10 # the tests of hash-partitioner were killed prematurally

startKafka

# Let us test the running without parameters - in example are generated 10 Kafka Messages to both topics
expected=("messageCount arg not set, using default value
Sending 10 sync Kafka notifications (protobuf) ...
Sending 10 async Kafka notifications (protobuf) ...
Async message successfully delivered, topic 'example-async-topic', partition '0', offset '0', key: 'async-proto-key',
Async message successfully delivered, topic 'example-async-topic', partition '0', offset '9', key: 'async-proto-key',
All plugins initialized successfully
Received Kafka Message, topic 'example-sync-topic', partition '0', offset '0', key: 'proto-key',
Received Kafka Message, topic 'example-sync-topic', partition '0', offset '9', key: 'proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '0', offset '0', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '0', offset '9', key: 'async-proto-key',
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/hash-partitioner/hash-partitioner --kafka-config examples/kafka-plugin/hash-partitioner/kafka.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - now let us test the messageCount (it relates to both topics)
expected=("Message count: 0
All plugins initialized successfully
Sending Kafka notification (protobuf)
Sending 0 sync Kafka notifications (protobuf) ...
Sending 0 async Kafka notifications (protobuf) ...
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd='examples/kafka-plugin/hash-partitioner/hash-partitioner --kafka-config examples/kafka-plugin/hash-partitioner/kafka.conf  -messageCount=0'
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

# Let us test - now let us test the messageCount (it relates to both topics)
expected=("Message count: 1
All plugins initialized successfully
Sending Kafka notification (protobuf)
Sending 1 sync Kafka notifications (protobuf) ...
Received Kafka Message, topic 'example-sync-topic', partition '0', offset '10', key: 'proto-key',
Sending 1 async Kafka notifications (protobuf) ...
Async message successfully delivered, topic 'example-async-topic', partition '0', offset '10', key: 'async-proto-key',
Received async Kafka Message, topic 'example-async-topic', partition '0', offset '10', key: 'async-proto-key',
Sync watcher closed
Async watcher closed
")

unexpected=("Error while stopping watcher
")

cmd='examples/kafka-plugin/hash-partitioner/hash-partitioner --kafka-config examples/kafka-plugin/hash-partitioner/kafka.conf  -messageCount=1'
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

stopKafka

#### Kafka-plugin post-init-consumer #######################################

#startKafka
startCustomizedKafka examples/kafka-plugin/manual-partitioner/server.properties

expected=("All plugins initialized successfully
Starting 'post-init' manual Consumer
Sending 10 Kafka notifications (protobuf) ...
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '0', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '1', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '8', key: 'proto-key',
Received sync Kafka Message, topic 'example-sync-topic', partition '1', offset '9', key: 'proto-key',
Post-init watcher closed
")

unexpected=("Error while stopping watcher
")

cmd="examples/kafka-plugin/post-init-consumer/post-init-consumer --kafka-config examples/kafka-plugin/post-init-consumer/kafka.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

stopKafka

#### Logs-lib ############################################################

expected=("Started observing beach
A group of walrus emerges from the ocean
The group's number increased tremendously!
Temperature changes
It's over 9000!
The ice breaks!
")
testOutput examples/logs-lib/basic/basic "${expected}" $RUNTIME_LIMIT

expected=("DEBUG componentXY
WARN componentXY
ERROR componentXY
")
testOutput examples/logs-lib/custom/custom "${expected}" $RUNTIME_LIMIT

#### Logs-plugin #########################################################

expected=("Debug log example
Info log example
Warn log example
Error log example
Stopping agent...
")

testOutput examples/logs-plugin/logs-plugin "${expected}" $RUNTIME_LIMIT

#### Simple-agent ########################################################

expected=("ETCD config not found, skip loading this plugin
kafka config not found  - skip loading this plugin
Redis config not found, skip loading this plugin
cassandra client config not found  - skip loading this plugin
All plugins initialized successfully
")

unexpected=("")

testOutput examples/simple-agent/simple-agent "${expected}" $RUNTIME_LIMIT "${unexpected}"

#### Simple-agent with Kafka and ETCD ####################################

startEtcd
startKafka

expected=("Plugin etcd: status check probe registered
Plugin kafka: status check probe registered
Redis config not found, skip loading this plugin
cassandra client config not found  - skip loading this plugin
All plugins initialized successfully
")

unexpected=("")

cmd="examples/simple-agent/simple-agent --etcd-config=examples/datasync-plugin/etcd.conf --kafka-config examples/kafka-plugin/hash-partitioner/kafka.conf"
testOutput "${cmd}" "${expected}" $RUNTIME_LIMIT "${unexpected}"

stopEtcd
stopKafka

##########################################################################

exit ${exitCode}
