[Documentation] Keywords for ssh sessions

*** Settings ***
Library       Collections
Library       RequestsLibrary
Library       SSHLibrary            timeout=15s    loglevel=TRACE
Library       DateTime

*** Variables ***
${timeout_etcd}=      30s

*** Keywords ***
Add Agent Node
    [Arguments]    ${node}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e KAFKA_CONFIG=disabled --sysctl net.ipv6.conf.all.disable_ipv6=0 -it -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE} bash
    #Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -it -p ${${node}_PING_HOST_PORT}:${${node}_PING_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}

Add Agent Node Again
    [Arguments]    ${node}
    Open SSH Connection    ${node}_again    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}_again    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e KAFKA_CONFIG=disabled --sysctl net.ipv6.conf.all.disable_ipv6=0 -it -p ${${node}_AGAIN_REST_API_HOST_PORT}:${${node}_AGAIN_REST_API_PORT} --name ${node}_again ${${node}_DOCKER_IMAGE} bash
    #Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -it -p ${${node}_PING_HOST_PORT}:${${node}_PING_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}_again    ${DOCKER_COMMAND} start ${node}_again
    Append To List    ${NODES}    ${node}_again
    Create Session    ${node}_again    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_again_HOSTNAME}    ${hostname}

Add Agent Node With Kafka
    [Arguments]    ${node}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} --sysctl net.ipv6.conf.all.disable_ipv6=0 -it -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE} bash
    #Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -it -p ${${node}_PING_HOST_PORT}:${${node}_PING_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}

Add Agent Node With Kafka Again
    [Arguments]    ${node}
    Open SSH Connection    ${node}_again    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}_again    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} --sysctl net.ipv6.conf.all.disable_ipv6=0 -it -p ${${node}_AGAIN_REST_API_HOST_PORT}:${${node}_AGAIN_REST_API_PORT} --name ${node}_again ${${node}_DOCKER_IMAGE} bash
    #Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -it -p ${${node}_PING_HOST_PORT}:${${node}_PING_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}_again    ${DOCKER_COMMAND} start ${node}_again
    Append To List    ${NODES}    ${node}_again
    Create Session    ${node}_again    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_again_HOSTNAME}    ${hostname}


Add Agent VPP Node
    [Arguments]    ${node}    ${vswitch}=${FALSE}
    ${add_params}=    Set Variable If    ${vswitch}    --pid=host -v "/var/run/docker.sock:/var/run/docker.sock"    ${EMPTY}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e KAFKA_CONFIG=disabled -e VPP_STATUS_PUBLISHERS=etcd -e INITIAL_LOGLVL=debug --sysctl net.ipv6.conf.all.disable_ipv6=0 -it --privileged -v "${VPP_AGENT_HOST_MEMIF_SOCKET_FOLDER}:${${node}_MEMIF_SOCKET_FOLDER}" -v "${DOCKER_SOCKET_FOLDER}:${${node}_SOCKET_FOLDER}" -p ${${node}_VPP_HOST_PORT}:${${node}_VPP_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${add_params} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    Open SSH Connection    ${node}_term    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Open SSH Connection    ${node}_vat    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    vpp_term: Open VPP Terminal    ${node}
    vat_term: Open VAT Terminal    ${node}
    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}

Add Agent VPP Node With Kafka
    [Arguments]    ${node}    ${vswitch}=${FALSE}
    ${add_params}=    Set Variable If    ${vswitch}    --pid=host -v "/var/run/docker.sock:/var/run/docker.sock"    ${EMPTY}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e VPP_STATUS_PUBLISHERS=etcd -e INITIAL_LOGLVL=debug --sysctl net.ipv6.conf.all.disable_ipv6=0 -it --privileged -v "${VPP_AGENT_HOST_MEMIF_SOCKET_FOLDER}:${${node}_MEMIF_SOCKET_FOLDER}" -v "${DOCKER_SOCKET_FOLDER}:${${node}_SOCKET_FOLDER}" -p ${${node}_VPP_HOST_PORT}:${${node}_VPP_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${add_params} ${${node}_DOCKER_IMAGE}
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    Open SSH Connection    ${node}_term    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Open SSH Connection    ${node}_vat    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    vpp_term: Open VPP Terminal    ${node}
    vat_term: Open VAT Terminal    ${node}
    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}


Add Agent Libmemif Node
    [Arguments]    ${node}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e KAFKA_CONFIG=disabled --sysctl net.ipv6.conf.all.disable_ipv6=0 -it --privileged -v "${VPP_AGENT_HOST_MEMIF_SOCKET_FOLDER}:${${node}_MEMIF_SOCKET_FOLDER}" --name ${node} ${${node}_DOCKER_IMAGE} /bin/bash
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    #${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Sleep     3s
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}
    Open SSH Connection    ${node}_lmterm    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    lmterm: Open LM Terminal    ${node}

Add Agent VPP Node With Physical Int
    [Arguments]    ${node}    ${int_nums}    ${vswitch}=${FALSE}
    ${add_params}=    Set Variable If    ${vswitch}    --pid=host -v "/var/run/docker.sock:/var/run/docker.sock"    ${EMPTY}
    Open SSH Connection    ${node}    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} create -e MICROSERVICE_LABEL=${node} -e KAFKA_CONFIG=disabled --sysctl net.ipv6.conf.all.disable_ipv6=0 -it --privileged -v "${DOCKER_SOCKET_FOLDER}:${${node}_SOCKET_FOLDER}" -p ${${node}_VPP_HOST_PORT}:${${node}_VPP_PORT} -p ${${node}_REST_API_HOST_PORT}:${${node}_REST_API_PORT} --name ${node} ${add_params}  ${${node}_DOCKER_IMAGE}
    ${devs}=               Set Variable    ${EMPTY}
    :FOR    ${int_num}    IN    @{int_nums}
    \    ${devs}=    Set Variable    ${devs}${\n}dev ${DOCKER_PHYSICAL_INT_${int_num}}
    ${data}=               OperatingSystem.Get File      ${CURDIR}/../resources/vpp_physical_int.conf
    ${data}=               Replace Variables             ${data}
    Create File            ${RESULTS_FOLDER}/vpp-${node}.conf    ${data}
    Create File            ${RESULTS_FOLDER_SUITE}/vpp-${node}.conf    ${data}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} cp ${EXECDIR}/${RESULTS_FOLDER}/vpp-${node}.conf ${node}:${VPP_CONF_PATH}
    Execute On Machine     ${node}    ${DOCKER_COMMAND} cp ${EXECDIR}/${RESULTS_FOLDER_SUITE}/vpp-${node}.conf ${node}:${VPP_CONF_PATH}
    Write To Machine       ${node}    ${DOCKER_COMMAND} start ${node}
    Append To List    ${NODES}    ${node}
    Open SSH Connection    ${node}_term    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Open SSH Connection    ${node}_vat    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    vpp_term: Open VPP Terminal    ${node}
    vat_term: Open VAT Terminal    ${node}
    Create Session    ${node}    http://${DOCKER_HOST_IP}:${${node}_REST_API_HOST_PORT}
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec ${node} bash -c 'echo $HOSTNAME'
    Set Suite Variable    ${${node}_HOSTNAME}    ${hostname}

Remove All Nodes
    :FOR    ${id}    IN    @{NODES}
    \    Remove Node    ${id}
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as

Remove All VPP Nodes
    :FOR    ${id}    IN    @{NODES}
    \   Run Keyword If    "vpp" in "${id}"       Remove Node    ${id}
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as

Remove Node
    [Arguments]    ${node}
    ${log}=    Execute On Machine    docker    ${DOCKER_COMMAND} logs --details -t ${node}    log=false
    Append To File    ${RESULTS_FOLDER}/output_${node}_container_agent.log    ${log}
    Append To File    ${RESULTS_FOLDER_SUITE}/output_${node}_container_agent.log    ${log}
    Switch Connection    ${node}
    Close Connection
    Run Keyword If    "vpp" in "${node}"    Remove VPP Connections    ${node}
    Run Keyword If    "libmemif" in "${node}"    Remove LM Connections    ${node}
    Remove Values From List    ${NODES}    ${node}
    Execute On Machine    docker    ${DOCKER_COMMAND} rm -f ${node}

Remove VPP Connections
    [Arguments]    ${node}
    Switch Connection    ${node}_term
    Close Connection
    Switch Connection    ${node}_vat
    Close Connection

Remove LM Connections
    [Arguments]    ${node}
    Switch Connection    ${node}_lmterm
    Close Connection

Check ETCD Running
    ${out}=   Write To Machine    docker     ${DOCKER_COMMAND} exec -it etcd etcdctl version
    Should Contain     ${out}    etcdctl version:
    [Return]           ${out}

Start ETCD Server
    Open SSH Connection    etcd    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine    etcd    ${ETCD_SERVER_CREATE}
    ${out}=  Write To Machine Until String    etcd    ${DOCKER_COMMAND} start -i etcd    etcdmain: ready to serve client requests
#    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec etcd bash -c 'echo $HOSTNAME'
#   etcd nema bash, preto dame hostname natvrdo
    ${hostname}=    Set Variable    etcd
    Set Suite Variable    ${ETCD_HOSTNAME}    ${hostname}
    wait until keyword succeeds    ${timeout_etcd}   5s   Check ETCD Running

Stop ETCD Server
    Execute On Machine    docker    ${ETCD_SERVER_DESTROY}

Execute In Container
    [Arguments]              ${container}       ${command}
    Switch Connection        docker
    ${currdate}=             Get Current Date
    Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
    Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
    ${out}  ${stderr}  ${rc}=      Execute Command    ${DOCKER_COMMAND} exec ${container} ${command}    return_stderr=True    return_rc=True
    ${status}=               Run Keyword And Return Status    Should be Empty    ${stderr}
    Run Keyword If           ${status}==False         Log     One or more error occured during execution of a command ${command} in container ${container}    level=WARN
    Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Response: ${out}${\n}***
    Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Response: ${out}${\n}***
    Run Keyword If           ${status}==False         Append To File           ${RESULTS_FOLDER}/output_${container}.log      *** Error: ${stderr}${\n}
    Run Keyword If           ${status}==False         Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log      *** Error: ${stderr}${\n}
    [Return]                 ${out}

Execute In Container Background
    [Arguments]              ${container}       ${command}
    Switch Connection        docker
    ${currdate}=             Get Current Date
    Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
    Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
    ${out}   ${stderr}=      Execute Command    ${DOCKER_COMMAND} exec -d ${container} ${command}    return_stderr=True
    ${status}=               Run Keyword And Return Status    Should be Empty    ${stderr}
    Run Keyword If           ${status}==False         Log     One or more error occured during execution of a command ${command} in container ${container}    level=WARN
    Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Response: ${out}${\n}
    Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} response: ${out}${\n}
    Run Keyword If           ${status}==False         Append To File           ${RESULTS_FOLDER}/output_${container}.log      *** Error: ${stderr}${\n}
    Run Keyword If           ${status}==False         Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log      *** Error: ${stderr}${\n}
    [Return]                 ${out}

Write To Container Until Prompt
   [Arguments]              ${container}               ${command}               ${prompt}=root@    ${delay}=${SSH_READ_DELAY}
   [Documentation]          *Write Container ${container} ${command}*
   ...                      Writing ${command} to connection with name ${container} and reading until prompt
   ...                      Output log is added to container output log
   Switch Connection        ${container}
   ${currdate}=             Get Current Date
   Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
   Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
   Write                    ${command}
   ${out}=                  Read Until    ${prompt}${${container}_HOSTNAME}    loglevel=TRACE
   ${out2}=                 Read          loglevel=TRACE           delay=${delay}
   Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
   Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
   [Return]                 ${out}${out2}

Write Command to Container
   [Arguments]              ${container}      ${command}       ${delay}=${SSH_READ_DELAY}
   [Documentation]          *Write Container ${container} ${command}*
   ...                      Writing ${command} to connection with name ${container} and reading output
   ...                      Output log is added to container output log
   Switch Connection        ${container}
   ${currdate}=             Get Current Date
   Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
   Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Command: ${command}${\n}
   ${written}=              Write        ${command}
   ${out}=                  Read        loglevel=TRACE    delay=${delay}
   Should Not Contain       ${out}     ${written}               # Was consumed from the output
   ${out2}=                 Read        loglevel=TRACE    delay=${delay}
   Append To File           ${RESULTS_FOLDER}/output_${container}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
   Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
   [Return]                 ${out}${out2}


Start Dev Container
    [Arguments]            ${command}=bash
    Open SSH Connection    dev    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     dev    ${DOCKER_COMMAND} create -it --name dev -e KAFKA_CONFIG=disabled --privileged ${DEV_IMAGE} ${command}
    Write To Machine       dev    ${DOCKER_COMMAND} start -i dev
    Switch Connection      dev
    Set Client Configuration    timeout=600s
    ${hostname}=           Execute On Machine    docker    ${DOCKER_COMMAND} exec dev bash -c 'echo $HOSTNAME'
    Set Suite Variable     ${DEV_HOSTNAME}    ${hostname}

Stop Dev Container
    Execute On Machine    docker    ${DOCKER_COMMAND} rm -f dev

Update Agent In Dev Container
    Write To Container Until Prompt    dev    cd $GOPATH/src/gitlab.cisco.com/ctao/vnf-agent
    Write To Container Until Prompt    dev    git pull
    Write To Container Until Prompt    dev    make
    Write To Container Until Prompt    dev    make install

Start Kafka Server
    Open SSH Connection    kafka    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine    kafka    ${KAFKA_SERVER_CREATE}
    ${out}=   Write To Machine Until String    kafka    ${DOCKER_COMMAND} start -i kafka   INFO success: kafka entered RUNNING state
    ${hostname}=    Execute On Machine    docker    ${DOCKER_COMMAND} exec kafka bash -c 'echo $HOSTNAME'
    Set Suite Variable   ${KAFKA_HOSTNAME}    ${hostname}

Stop Kafka Server
    Execute On Machine    docker    ${KAFKA_SERVER_DESTROY}

Start VPP Ctl Container
    [Arguments]            ${command}=bash
    Open SSH Connection    vpp_agent_ctl    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     vpp_agent_ctl    ${DOCKER_COMMAND} create -it --name vpp_agent_ctl ${VPP_AGENT_CTL_IMAGE_NAME} ${command}
    Write To Machine       vpp_agent_ctl    ${DOCKER_COMMAND} start -i vpp_agent_ctl
    ${hostname}=           Execute On Machine    docker    ${DOCKER_COMMAND} exec vpp_agent_ctl bash -c 'echo $HOSTNAME'
    Set Suite Variable     ${VPP_AGENT_CTL_HOSTNAME}    ${hostname}

Stop VPP Ctl Container
    Execute On Machine    docker    ${DOCKER_COMMAND} rm -f vpp_agent_ctl

Start SFC Controller Container With Own Config
    [Arguments]            ${config}
    Open SSH Connection    sfc_controller    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Execute On Machine     sfc_controller    ${DOCKER_COMMAND} create -it --name sfc_controller ${SFC_CONTROLLER_IMAGE_NAME}
    SSHLibrary.Put_file    ${DATA_FOLDER}/${config}	    /tmp/
    Execute On Machine     sfc_controller    ${DOCKER_COMMAND} cp /tmp/${config} sfc_controller:${SFC_CONTROLLER_CONF_PATH}
    Write To Machine       sfc_controller    ${DOCKER_COMMAND} start -i sfc_controller
    #Sleep                  400s
    ${hostname}=           Execute On Machine    docker    ${DOCKER_COMMAND} exec sfc_controller sh -c 'echo $HOSTNAME'
    Set Suite Variable     ${SFC_CONTROLLER_HOSTNAME}    ${hostname}

Stop SFC Controller Container
    Execute On Machine    docker    ${DOCKER_COMMAND} rm -f sfc_controller
