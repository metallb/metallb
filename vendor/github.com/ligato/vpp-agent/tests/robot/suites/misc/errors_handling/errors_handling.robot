*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Force Tags        misc    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${MAC_GOOD}=      a2:01:01:01:01:01
${MAC_BAD1}=       a2:01:01:01:01:01:xy
${MAC_BAD2}=       a2:01:01:01:01:01:zz

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Interface Should Not Be Present
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MAC_GOOD}
    ${int_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/vpp1_memif1
    ${int_error_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/error/vpp1_memif1
    ${out}=    Read Key    ${int_key}
    Should Be Empty    ${out}
    ${out}=    Read Key    ${int_error_key}
    Should Be Empty    ${out}

Add Memif With Wrong MAC
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=${MAC_BAD1}    master=true    id=1    ip=192.168.1.1    prefix=24    socket=default.sock
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MAC_BAD1}
    ${int_error_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/error/vpp1_memif1
    ${out}=    Read Key    ${int_error_key}
    Should Contain    ${out}    error_data

Correct MAC In Memif
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=${MAC_GOOD}    master=true    id=1    ip=192.168.1.1    prefix=24    socket=default.sock
    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_GOOD}
    ${int_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/vpp1_memif1
    ${int_error_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/error/vpp1_memif1
    ${out}=    Read Key    ${int_key}
    Should Not Be Empty    ${out}
    ${out}=    Read Key    ${int_error_key}
    Should Contain    ${out}    error_data

Set Wrong MAC To Memif Again
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=${MAC_BAD2}    master=true    id=1    ip=192.168.1.1    prefix=24    socket=default.sock
    vpp_term: Interface Is Deleted    node=agent_vpp_1    mac=${MAC_GOOD}   
    ${int_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/vpp1_memif1
    ${int_error_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/error/vpp1_memif1
    ${out}=    Read Key    ${int_key}
    Should Contain    ${out}    vpp1_memif1
    ${out}=    Read Key    ${int_error_key}
    Should Contain    ${out}    error_data
    Should Contain    ${out}    ${MAC_BAD1}
    Should Contain    ${out}    ${MAC_BAD2}

Delete Memif
    Delete VPP Interface    node=agent_vpp_1    name=vpp1_memif1
    Sleep    5s
    ${int_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/vpp1_memif1
    ${int_error_key}=    Set Variable    /vnf-agent/agent_vpp_1/vpp/status/${AGENT_VER}/interface/error/vpp1_memif1
    ${out}=    Read Key    ${int_key}
    Should Be Empty    ${out}
    ${out}=    Read Key    ${int_error_key}
    Should Be Empty    ${out}

Show Interfaces And Other Objects After Test
    Sleep    5s
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_1_term    show memif
    Write To Machine    agent_vpp_1_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    Execute In Container    agent_vpp_1    ip a

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

