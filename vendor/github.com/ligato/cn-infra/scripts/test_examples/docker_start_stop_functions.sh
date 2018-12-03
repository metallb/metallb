#!/usr/bin/env bash

function startEtcd {
    docker run -p 2379:2379 --name etcd -d -e ETCDCTL_API=3 \
        quay.io/coreos/etcd:v3.1.0 /usr/local/bin/etcd \
             -advertise-client-urls http://0.0.0.0:2379 \
                 -listen-client-urls http://0.0.0.0:2379 > /dev/null
    # dump etcd content to make sure that etcd is ready
    docker exec etcd etcdctl get --prefix ""
    # sometimes etcd needs a bit more time to fully initialize
    sleep 2
}

function stopEtcd {
    docker stop etcd > /dev/null
    docker rm etcd > /dev/null
}

function startKafka {
    docker run -p 2181:2181 -p 9092:9092 --name kafka -d \
        --env ADVERTISED_HOST=0.0.0.0 --env ADVERTISED_PORT=9092 spotify/kafka > /dev/null
    KAFKA_VERSION=$(docker exec kafka /bin/bash -c 'echo $KAFKA_VERSION')
    SCALA_VERSION=$(docker exec kafka /bin/bash -c 'echo $SCALA_VERSION')
    # list kafka topics to ensure that kafka is ready
    docker exec kafka  /opt/kafka_${SCALA_VERSION}-${KAFKA_VERSION}/bin/kafka-topics.sh --list --zookeeper localhost:2181 > /dev/null 2> /dev/null
    # sometimes Kafka needs a bit more time to fully initialize
    sleep 2
}

# startCustomizedKafka takes path to server.properties as the only argument.
function startCustomizedKafka {
    docker create -p 2181:2181 -p 9092:9092 --name kafka \
        --env ADVERTISED_HOST=0.0.0.0 --env ADVERTISED_PORT=9092 spotify/kafka > /dev/null
    KAFKA_VERSION=$(docker inspect -f '{{ .Config.Env }}' kafka |  tr ' ' '\n' | grep KAFKA_VERSION | sed 's/^.*=//')
    SCALA_VERSION=$(docker inspect -f '{{ .Config.Env }}' kafka |  tr ' ' '\n' | grep SCALA_VERSION | sed 's/^.*=//')
    docker cp $1 kafka:/opt/kafka_${SCALA_VERSION}-${KAFKA_VERSION}/config/server.properties
    docker start kafka > /dev/null
    # list kafka topics to ensure that kafka is ready
    docker exec kafka  /opt/kafka_${SCALA_VERSION}-${KAFKA_VERSION}/bin/kafka-topics.sh --list --zookeeper localhost:2181 > /dev/null 2> /dev/null
    # sometimes Kafka needs a bit more time to fully initialize
    sleep 2
}

function stopKafka {
    docker stop kafka > /dev/null
    docker rm kafka > /dev/null
}

function startCassandra {
    docker run -p 9042:9042 --name cassandra01 -d cassandra > /dev/null 2> /dev/null
    # Wait until cassandra is ready to accept a connection.
    for attemptps in {1..20} ; do
        NODEINFO=$(docker exec -it cassandra01 nodetool info)
        if [ $? -eq 0 ]; then
            if [[ ${NODEINFO} == *"Native Transport active: true"* ]]; then
                break
            fi
        fi
    done
    # sometimes Cassandra needs a bit more time to fully initialize
    sleep 2
}

function stopCassandra {
    docker stop cassandra01 > /dev/null
    docker rm cassandra01 > /dev/null
}

function startRedis {
    docker run --name redis -p 6379:6379 -d redis > /dev/null 2> /dev/null
    # sometimes Cassandra needs a bit more time to fully initialize
    # sleep 2
}

function stopRedis {
    docker stop redis > /dev/null
    docker rm redis > /dev/null
}

