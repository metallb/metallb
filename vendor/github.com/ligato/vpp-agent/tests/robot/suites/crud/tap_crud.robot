*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=        common
${ENV}=              common
${NAME_TAP1}=        vpp1_tap1
${NAME_TAP2}=        vpp1_tap2
${MAC_TAP1}=         12:21:21:11:11:11
${MAC_TAP1_2}=       22:21:21:11:11:11
${MAC_TAP2}=         22:21:21:22:22:22
${IP_TAP1}=          20.20.1.1
${IP_TAP1_2}=        21.20.1.2
${IP_TAP2}=          20.20.2.1
${PREFIX}=           24
${MTU}=              4800
${UP_STATE}=         up
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    ${interfaces}=    vpp_term: Show Interfaces    agent_vpp_1

Add TAP1 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_TAP1}
    Put TAP Interface With IP    node=agent_vpp_1    name=${NAME_TAP1}    mac=${MAC_TAP1}    ip=${IP_TAP1}    prefix=${PREFIX}    host_if_name=linux_${NAME_TAP1}

Check TAP1 Interface Is Created
    ${interfaces}=       vat_term: Interfaces Dump    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_TAP1}
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP1}    mac=${MAC_TAP1}    ipv4=${IP_TAP1}/${PREFIX}    state=${UP_STATE}

Add TAP2 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_TAP2}
    Put TAP Interface With IP    node=agent_vpp_1    name=${NAME_TAP2}    mac=${MAC_TAP2}    ip=${IP_TAP2}    prefix=${PREFIX}    host_if_name=linux_${NAME_TAP2}
    Sleep     5

Check TAP2 Interface Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_TAP2}
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP2}    mac=${MAC_TAP2}    ipv4=${IP_TAP2}/${PREFIX}    state=${UP_STATE}

Check TAP1 Interface Is Still Configured
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP1}    mac=${MAC_TAP1}    ipv4=${IP_TAP1}/${PREFIX}    state=${UP_STATE}

Update TAP1 Interface
    Put TAP Interface With IP    node=agent_vpp_1    name=${NAME_TAP1}    mac=${MAC_TAP1_2}    ip=${IP_TAP1_2}    prefix=${PREFIX}    host_if_name=linux_${NAME_TAP1}

Check TAP1_2 Interface Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_TAP1_2}
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP1}    mac=${MAC_TAP1_2}    ipv4=${IP_TAP1_2}/${PREFIX}    state=${UP_STATE}

Check TAP2 Interface Has Not Changed
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP2}    mac=${MAC_TAP2}    ipv4=${IP_TAP2}/${PREFIX}    state=${UP_STATE}

Delete TAP1_2 Interface
    Delete VPP Interface    agent_vpp_1    ${NAME_TAP1}

Check TAP1_2 Interface Has Been Deleted
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_TAP1_2}

Check TAP2 Interface Is Still Configured
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_TAP2}    mac=${MAC_TAP2}    ipv4=${IP_TAP2}/${PREFIX}    state=${UP_STATE}

Show Interfaces And Other Objects After Setup
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_1_term    show br 1 detail
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Write To Machine    agent_vpp_1_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    Execute In Container    agent_vpp_1    ip a

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown


