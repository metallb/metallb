*** Settings ***

Library      OperatingSystem
Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Suite Setup       Setup
Suite Teardown    Testsuite Teardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common

*** Test Cases ***
List All Loggers
    Get All Loggers on agent_vpp_1

Change Logger Level
    #Change Log Level On agent_vpp_1 From debug To info On defaultLogger
    Change Log Level On agent_vpp_1 From debug To info On vpp-plugin
    Change Log Level On agent_vpp_1 From debug To info On vpp-plugin-if-conf
    ${from_now}=  Get Time     epoch
    Put Loopback Interface With IP    agent_vpp_1    loop0   8a:f1:be:90:00:03    10.1.1.1
    Sleep     5
    ${out}=      Write To Machine    docker     docker logs --since ${from_now} agent_vpp_1
    Should Not Contain     ${out}    level=debug msg="Start processing change for key: vpp/config/${AGENT_VER}/interface/loop0"
    Should Not Contain     ${out}    level=debug msg="MAC address added" MAC address="8a:f1:be:90:00:03"
    Should Not Contain     ${out}    level=debug msg="IP address added." IPAddress=10.1.1.1

Check If Agent Is Live
    Agent liveness Should Be OK On agent_vpp_1

Check If Agent Is Ready
    Agent readiness Should Be OK On agent_vpp_1

Check Agent State Data In ETCD
    Get Agent Status For agent_vpp_1 From ETCD Should be OK

Check Plugins State Data In ETCD
    Get govpp Plugin Status For agent_vpp_1 From ETCD


Change API Port
    [Teardown]    Teardown
    Change API Port From 9191 To 8888 On agent_vpp_1

*** Keywords ***
Setup
    Testsuite Setup
    Add Agent VPP Node    agent_vpp_1

Teardown
    Remove Node    agent_vpp_1

Get All Loggers On ${node}
    ${out}=     rest_api: Get Loggers List    agent_vpp_1
     Should Contain     ${out}    etcd
     Should Contain     ${out}    govpp
     Should Contain     ${out}    http
     Should Contain     ${out}    health-rpc
     Should Contain     ${out}    status-check
     Should Contain     ${out}    kafka
     Should Contain     ${out}    redis
     Should Contain     ${out}    cassandra

Change Log Level On ${node} From ${old_level} To ${expected_level} On ${logger_name}
    ${out}=      rest_api: Get Loggers List   ${node}
    ${level}=    Extract Logger Level         ${logger_name}    ${out}
    Should Be Equal        ${level}    ${old_level}
    Should Not Be Equal    ${level}    ${expected_level}
    rest_api: Change Logger Level      ${node}    ${logger_name}    ${expected_level}
    ${out}=      rest_api: Get Loggers List       ${node}
    ${level}=    Extract Logger Level             ${logger_name}    ${out}
    Should Be Equal        ${level}    ${expected_level}
    Should Not Be Equal    ${level}    ${old_level}

Agent ${ability} Should Be ${expected} On ${node}
    ${expected}=    Set Variable if    "${expected}"=="OK"    1    0
    ${uri}=         Set Variable     /${ability}
    ${out}=         rest_api: Get    ${node}    ${uri}
    #Should Match Regexp    ${out}    \\{\\"build_version\\":\\"[a-f0-9]+\\"\\,\\"build_date\\"\\:\\"\\d{4}\\-\\d{2}\\-\\d{2}\\T\\d{2}\\:\\d{2}\\+\\d{2}\\:\\d{2}\\",\\"state\\"\\:${expected},\\"start_time\\":\\d+,\\"last_change\\":\\d+,\\"last_update\\":\\d+,\\"commit_hash\\":\\"[a-e0-9]+\\"\\}
    Should Match Regexp    ${out}     \\"build_version\\":\\"[a-z0-9_.-]+\\"
    Should Match Regexp    ${out}     \\"build_date\\"\\:\\"\\d{4}\\-\\d{2}\\-\\d{2}\\T\\d{2}\\:\\d{2}\\+\\d{2}\\:\\d{2}\\"
    Should Match Regexp    ${out}     \\"state\\"\\:${expected}
    Should Match Regexp    ${out}     \\"start_time\\":\\d+
    Should Match Regexp    ${out}     \\"last_change\\":\\d+
    Should Match Regexp    ${out}     \\"last_update\\":\\d+
    Should Match Regexp    ${out}     \\"commit_hash\\":\\"[a-f0-9]+\\"

Change API Port From ${old_port} To ${new_port} On ${node}
    Set Test Variable    ${${node}_REST_API_PORT}         ${new_port}
    Set Test Variable    ${${node}_REST_API_HOST_PORT}    ${new_port}
    Should Be Equal      ${${node}_REST_API_PORT}    ${new_port}
    Should Not Be Equal  ${${node}_REST_API_PORT}    ${old_port}
    Remove Node    agent_vpp_1
    Add VPP Agent agent_vpp_1
    Start VPP On Node agent_vpp_1
    Start Agent On ${node} With Port ${new_port}
    Sleep    20    # we need wait until agent is fully loaded
    Agent readiness Should Be OK On ${node}

Get Agent Status For ${node} From ETCD Should be ${expected}
    ${expected}=    Set Variable if    "${expected}"=="OK"    1    0
    ${out}=   Write To Machine    docker     docker exec -it etcd etcdctl get /vnf-agent/${node}/check/status/${AGENT_VER}/agent
    Should Contain         ${out}    ${node}
    #Should Match Regexp    ${out}    \\{\\"build_version\\":\\"[a-f0-9]+\\"\\,\\"build_date\\"\\:\\"\\d{4}\\-\\d{2}\\-\\d{2}\\T\\d{2}\\:\\d{2}\\+\\d{2}\\:\\d{2}\\",\\"state\\"\\:${expected},\\"start_time\\":\\d+,\\"last_change\\":\\d+,\\"last_update\\":\\d+\\}
    Should Match Regexp    ${out}     \\"build_version\\":\\"[a-z0-9_.-]+\\"
    Should Match Regexp    ${out}     \\"build_date\\"\\:\\"\\d{4}\\-\\d{2}\\-\\d{2}\\T\\d{2}\\:\\d{2}\\+\\d{2}\\:\\d{2}\\"
    Should Match Regexp    ${out}     \\"state\\"\\:${expected}
    Should Match Regexp    ${out}     \\"start_time\\":\\d+
    Should Match Regexp    ${out}     \\"last_change\\":\\d+
    Should Match Regexp    ${out}     \\"last_update\\":\\d+
    Should Match Regexp    ${out}     \\"commit_hash\\":\\"[a-f0-9]+\\"
    Sleep    20s
    ${out2}=    Write To Machine    docker    docker exec -it etcd etcdctl get /vnf-agent/${node}/check/status/${AGENT_VER}/agent
    Should Not Be Equal    ${out}    ${out2}

Get ${plugin} Plugin Status For ${node} From ETCD
    ${out}=     Write To Machine    docker     docker exec -it etcd etcdctl get /vnf-agent/${node}/check/status/${AGENT_VER}/plugin/${plugin}
    Should Contain    ${out}    ${node}


Start Agent On ${node} With Port ${port}
    ${out}=    Execute In Container    ${node}    vpp-agent -http-probe-port ${port} --etcd-config=${AGENT_VPP_ETCD_CONF_PATH} --kafka-config=${AGENT_VPP_KAFKA_CONF_PATH} &
    [Return]  ${out}

Add VPP Agent ${node}
#    Log            ${node}
#    Execute On Machine    docker    ${DOCKER_COMMAND} run -e MICROSERVICE_LABEL=${node} -itd --privileged -v "${DOCKER_SOCKET_FOLDER}:${${node}_SOCKET_FOLDER}" -p ${${node}_VPP_HOST_PORT}:${${node}_VPP_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${AGENT_VPP_1_DOCKER_IMAGE} bash
#    Append To List    ${NODES}    ${node}
#    Sleep    5
#    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
#    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
#    Set Suite Variable    ${${node}_HOlSTNAME}    ${hostname}

    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -it --privileged -v "${DOCKER_SOCKET_FOLDER}:${${node}_SOCKET_FOLDER}" -p ${${node}_VPP_HOST_PORT}:${${node}_VPP_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${AGENT_VPP_1_DOCKER_IMAGE} bash
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
#    Open SSH Connection    ${node}_term    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
#    Open SSH Connection    ${node}_vat    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
#    vpp_term: Open VPP Terminal    ${node}
#    vat_term: Open VAT Terminal    ${node}

    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}

Start VPP On Node ${node}
#    Execute In Container    ${node}   vpp unix { cli-listen localhost:5002 } plugins { plugin dpdk_plugin.so { disable } } &
    Execute In Container    ${node}   vpp unix { nodaemon cli-listen 0.0.0.0:5002 cli-no-pager } plugins { plugin dpdk_plugin.so { disable } } &
    Open SSH Connection    ${node}_term    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Open SSH Connection    ${node}_vat    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    vpp_term: Open VPP Terminal    ${node}
    vat_term: Open VAT Terminal    ${node}

