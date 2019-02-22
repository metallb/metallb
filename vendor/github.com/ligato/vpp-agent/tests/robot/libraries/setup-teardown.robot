[Documentation]     Reusable keywords for testsuite setup and teardown

*** Settings ***
Library       String
Library       RequestsLibrary
Library       SSHLibrary            timeout=15s     loglevel=TRACE
#Resource      ssh.robot
Resource      ${ENV}_setup-teardown.robot

*** Variables ***
${VM_SSH_ALIAS_PREFIX}     vm_
${snapshot_num}       0
@{NODES}              

*** Keywords ***
Open SSH Connection
    [Arguments]         ${name}    ${ip}    ${user}    ${pswd}
    Open Connection     ${ip}      ${name}
    Run Keyword If      "${pswd}"!="rsa_id"   Login                              ${user}   ${pswd}
    Run Keyword If      "${pswd}"=="rsa_id"   SSHLibrary.Login_With_Public_Key   ${user}   %{HOME}/.ssh/id_rsa   any

Testsuite Setup
    Discard old results
    #Remove File		${OUTPUTDIR}/ssh.log
    Enable SSH Logging		${OUTPUTDIR}/ssh.log
    Open Connection To Docker Host
    Create Connections For ETCD And Kafka
    #Start Kafka Server
    Start ETCD Server
    #Run Keyword If     "${CTL_TOOL}"== "vppctl"     Start VPP Ctl Container
    Start VPP Ctl Container
    Get Env And SW Version      docker
    Make Datastore Snapshots    startup


Testsuite Teardown
    Make Datastore Snapshots    teardown
    Remove All Nodes
    Stop ETCD Server
    #Run Keyword If     "${CTL_TOOL}"== "vppctl"     Stop VPP Ctl Container
    Stop VPP Ctl Container
    #Stop Kafka Server
    Get Connections
    Close All Connections
    Check Agent Logs For Errors
    #Copy File    ${OUTPUTDIR}/ssh.log	${RESULTS_FOLDER_SUITE}/ssh.log

Test Setup
    Open Connection To Docker Host
    Create Connections For ETCD And Kafka
    #Start Kafka Server
    Start ETCD Server
    #Run Keyword If     "${CTL_TOOL}"== "vppctl"     Start VPP Ctl Container
    Start VPP Ctl Container
    Make Datastore Snapshots    startup

Test Teardown
    Make Datastore Snapshots    teardown
    #Run Keyword If     "${CTL_TOOL}"== "vppctl"     Stop VPP Ctl Container
    Stop VPP Ctl Container
    #Stop Kafka Server
    Stop ETCD Server
    Remove All Nodes
    Get Connections
    Close All Connections

Discard old results
    [Documentation]    Remove and re-create ${RESULTS_FOLDER} and ${RESULTS_FOLDER}/SUTIE_NAME specific folder.
    Remove File         ${RESULTS_FOLDER}/*.txt
    Remove File         ${RESULTS_FOLDER}/*.log
    #Create Directory    ${RESULTS_FOLDER}
    Remove Directory    ${RESULTS_FOLDER_SUITE}           recursive=true
    Create Directory    ${RESULTS_FOLDER}
    Create Directory    ${RESULTS_FOLDER_SUITE}


Log All SSH Outputs
    ...                       Logs all connections outputs
    [Timeout]                 120s
    :FOR    ${id}    IN    @{NODES}
    \    Log ${id} Output
    \    Run Keyword If    "vpp" in "${id}"    Log ${id}_term Output
    \    Run Keyword If    "vpp" in "${id}"    Log ${id}_vat Output          
    Log docker Output

Log ${machine} Output
    [Documentation]         *Log ${machine} Output*
    ...                     Logs actual ${machine} output from begining
    Switch Connection       ${machine}
    ${out}=                 Read                   delay=${SSH_READ_DELAY}s
    Append To File          ${RESULTS_FOLDER}/output_${machine}.log                ${out}
    Append To File          ${RESULTS_FOLDER_SUITE}/output_${machine}.log                ${out}

Get Machine Status
    [Arguments]              ${machine}
    [Documentation]          *Get Machine Status ${machine}*
    ...                      Executing df, free, ifconfig -a, ps -aux... on ${machine}
    Execute On Machine       ${machine}                df
    Execute On Machine       ${machine}                free
    Execute On Machine       ${machine}                ifconfig -a
    Execute On Machine       ${machine}                ps aux
    Execute On Machine       ${machine}                echo $PATH

Get Env And SW Version
    [Arguments]              ${machine}
    [Documentation]          *Get Machine Status ${machine}*
    ...                      Executing df, free, ifconfig -a, ps -aux... on ${machine}
    Execute On Machine       ${machine}                env
    Execute On Machine       ${machine}                curl -V

Open Connection To Docker Host
    Open SSH Connection    docker    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    Get Machine Status     docker
    Execute On Machine     docker    ${DOCKER_COMMAND} images
    Execute On Machine     docker    ${DOCKER_COMMAND} ps -as

Create Connections For ETCD And Kafka
    Open SSH Connection    etcd    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    #Open SSH Connection    kafka    ${DOCKER_HOST_IP}    ${DOCKER_HOST_USER}    ${DOCKER_HOST_PSWD}
    
Make Datastore Snapshots
    [Arguments]            ${tag}=notag
    ${prefix}=             Create Next Snapshot Prefix
    Take ETCD Snapshots    ${prefix}_${tag}

Get ETCD Dump
    [Arguments]    ${machine}=docker
    ${command}=         Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl get --prefix="true" ""
    ${out}=             Execute On Machine    ${machine}    ${command}    log=false
    [Return]            ${out}

Take ETCD Snapshots
    [Arguments]         ${tag}    ${machine}=docker
    ${dump}=            Get ETCD Dump    ${machine}
    Append To File      ${RESULTS_FOLDER}/etcd_dump-${tag}.txt    ${dump}
    Append To File      ${RESULTS_FOLDER_SUITE}/etcd_dump-${tag}.txt    ${dump}
    ${errors}=          Get Lines Containing String    ${dump}    /error/
    ${status}=          Run Keyword And Return Status    Should Be Empty    ${errors}
    Run Keyword If      ${status}==False         Log     Errors detected in keys: ${errors}    level=WARN
    
Create Next Snapshot Prefix
    ${prefix}=          Evaluate    str(${snapshot_num}).zfill(3)
    ${snapshot_num}=    Evaluate    ${snapshot_num}+1
    Set Global Variable  ${snapshot_num}
    [Return]            ${prefix}

Check Agent Logs For Errors
    @{logs}=    OperatingSystem.List Files In Directory    ${RESULTS_FOLDER}/    *_container_agent.log
    :FOR    ${log}    IN    @{logs}
    \    ${data}=    OperatingSystem.Get File    ${RESULTS_FOLDER}/${log}
    \    Should Not Contain    ${data}    exited: agent (exit status
    \    Should Not Contain    ${data}    exited: vpp (exit status
