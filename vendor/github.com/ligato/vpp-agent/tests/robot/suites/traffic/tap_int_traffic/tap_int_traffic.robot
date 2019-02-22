*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot
Resource     ../../../libraries/all_libs.robot
Resource    ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=               common
${ENV}=                     common
${NAME_VPP1_TAP1}=          vpp1_tap1
${NAME_VPP2_TAP1}=          vpp2_tap1
${MAC_VPP1_TAP1}=           12:21:21:11:11:11
${MAC_VPP2_TAP1}=           22:21:21:22:22:22
${IP_VPP1_TAP1}=            10.10.1.1
${IP_VPP2_TAP1}=            20.20.1.1
${IP_LINUX_VPP1_TAP1}=      10.10.1.2
${IP_LINUX_VPP2_TAP1}=      20.20.1.2
${IP_VPP1_TAP1_NETWORK}=    10.10.1.0
${IP_VPP2_TAP1_NETWORK}=    20.20.1.0
${NAME_VPP1_MEMIF1}=        vpp1_memif1
${NAME_VPP2_MEMIF1}=        vpp2_memif1
${MAC_VPP1_MEMIF1}=         13:21:21:11:11:11
${MAC_VPP2_MEMIF1}=         23:21:21:22:22:22
${IP_VPP1_MEMIF1}=          192.168.1.1
${IP_VPP2_MEMIF1}=          192.168.1.2
${PREFIX}=                  24
${UP_STATE}=                up
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
# wait for resync vpps after restart
${RESYNC_WAIT}=        50s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Add VPP1_TAP1 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_VPP1_TAP1}
    Put TAP Interface With IP    node=agent_vpp_1    name=${NAME_VPP1_TAP1}    mac=${MAC_VPP1_TAP1}    ip=${IP_VPP1_TAP1}    prefix=${PREFIX}    host_if_name=linux_${NAME_VPP1_TAP1}
    linux: Set Host TAP Interface    node=agent_vpp_1    host_if_name=linux_${NAME_VPP1_TAP1}    ip=${IP_LINUX_VPP1_TAP1}    prefix=${PREFIX}

Check VPP1_TAP1 Interface Is Created
    ${interfaces}=       vat_term: Interfaces Dump    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_VPP1_TAP1}
    ${actual_state}=    vpp_term: Check TAP interface State    agent_vpp_1    ${NAME_VPP1_TAP1}    mac=${MAC_VPP1_TAP1}    ipv4=${IP_VPP1_TAP1}/${PREFIX}    state=${UP_STATE}

Check Ping Between VPP1 and linux_VPP1_TAP1 Interface
    linux: Check Ping    node=agent_vpp_1    ip=${IP_VPP1_TAP1}
    vpp_term: Check Ping    node=agent_vpp_1    ip=${IP_LINUX_VPP1_TAP1}

Add VPP1_memif1 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MAC_VPP1_MEMIF1}
    Put Memif Interface With IP    node=agent_vpp_1    name=${NAME_VPP1_MEMIF1}    mac=${MAC_VPP1_MEMIF1}    master=true    id=1    ip=${IP_VPP1_MEMIF1}    prefix=24    socket=memif.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_VPP1_MEMIF1}

Add VPP2_TAP1 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_2    mac=${MAC_VPP2_TAP1}
    Put TAP Interface With IP    node=agent_vpp_2    name=${NAME_VPP2_TAP1}    mac=${MAC_VPP2_TAP1}    ip=${IP_VPP2_TAP1}    prefix=${PREFIX}    host_if_name=linux_${NAME_VPP2_TAP1}
    linux: Set Host TAP Interface    node=agent_vpp_2    host_if_name=linux_${NAME_VPP2_TAP1}    ip=${IP_LINUX_VPP2_TAP1}    prefix=${PREFIX}

Check VPP2_TAP1 Interface Is Created
    ${interfaces}=       vat_term: Interfaces Dump    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_2    mac=${MAC_VPP2_TAP1}
    ${actual_state}=    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check TAP interface State    agent_vpp_2    ${NAME_VPP2_TAP1}    mac=${MAC_VPP2_TAP1}    ipv4=${IP_VPP2_TAP1}/${PREFIX}    state=${UP_STATE}

Check Ping Between VPP2 And linux_VPP2_TAP1 Interface
    linux: Check Ping    node=agent_vpp_2    ip=${IP_VPP2_TAP1}
    vpp_term: Check Ping    node=agent_vpp_2    ip=${IP_LINUX_VPP2_TAP1}

Add VPP2_memif1 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_2    mac=${MAC_VPP2_MEMIF1}
    Put Memif Interface With IP    node=agent_vpp_2    name=${NAME_VPP2_MEMIF1}    mac=${MAC_VPP2_MEMIF1}    master=false    id=1    ip=${IP_VPP2_MEMIF1}    prefix=24    socket=memif.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_VPP1_MEMIF1}

Check Ping From VPP1 To VPP2_memif1
    vpp_term: Check Ping    node=agent_vpp_1    ip=${IP_VPP2_MEMIF1}

Check Ping From VPP2 To VPP1_memif1
    vpp_term: Check Ping    node=agent_vpp_2    ip=${IP_VPP1_MEMIF1}

Ping From VPP1 Linux To VPP2_TAP1 And LINUX_VPP2_TAP1 Should Not Pass
    ${status1}=    Run Keyword And Return Status    linux: Check Ping    node=agent_vpp_1    ip=${IP_VPP2_TAP1}
    ${status2}=    Run Keyword And Return Status    linux: Check Ping    node=agent_vpp_1    ip=${IP_LINUX_VPP2_TAP1}
    Should Be Equal As Strings    ${status1}    False
    Should Be Equal As Strings    ${status2}    False

Ping From VPP2 Linux To VPP1_TAP1 And LINUX_VPP1_TAP1 Should Not Pass
    ${status1}=    Run Keyword And Return Status    linux: Check Ping    node=agent_vpp_2    ip=${IP_VPP1_TAP1}
    ${status2}=    Run Keyword And Return Status    linux: Check Ping    node=agent_vpp_2    ip=${IP_LINUX_VPP1_TAP1}
    Should Be Equal As Strings    ${status1}    False
    Should Be Equal As Strings    ${status2}    False

Add Static Route From VPP1 Linux To VPP2
    linux: Add Route    node=agent_vpp_1    destination_ip=${IP_VPP2_TAP1_NETWORK}    prefix=${PREFIX}    next_hop_ip=${IP_VPP1_TAP1}

Add Static Route From VPP1 To VPP2
    Create Route On agent_vpp_1 With IP ${IP_VPP2_TAP1_NETWORK}/${PREFIX} With Next Hop ${IP_VPP2_MEMIF1} And Vrf Id 0

Add Static Route From VPP2 Linux To VPP1
    linux: Add Route    node=agent_vpp_2    destination_ip=${IP_VPP1_TAP1_NETWORK}    prefix=${PREFIX}    next_hop_ip=${IP_VPP2_TAP1}

Add Static Route From VPP2 To VPP1
    Create Route On agent_vpp_2 With IP ${IP_VPP1_TAP1_NETWORK}/${PREFIX} With Next Hop ${IP_VPP1_MEMIF1} And Vrf Id 0
     Sleep     ${SYNC_SLEEP}

Check Ping From VPP1 Linux To VPP2_TAP1 And LINUX_VPP2_TAP1
    linux: Check Ping    node=agent_vpp_1    ip=${IP_VPP2_TAP1}
    linux: Check Ping    node=agent_vpp_1    ip=${IP_LINUX_VPP2_TAP1}

Check Ping From VPP2 Linux To VPP1_TAP1 And LINUX_VPP1_TAP1
    linux: Check Ping    node=agent_vpp_2    ip=${IP_VPP1_TAP1}
    linux: Check Ping    node=agent_vpp_2    ip=${IP_LINUX_VPP1_TAP1}

Remove VPP Nodes
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Create linux_VPP1_TAP1 And linux_VPP2_TAP1 Interfaces After Resync
    linux: Set Host TAP Interface    node=agent_vpp_1    host_if_name=linux_${NAME_VPP1_TAP1}    ip=${IP_LINUX_VPP1_TAP1}    prefix=${PREFIX}
    linux: Set Host TAP Interface    node=agent_vpp_2    host_if_name=linux_${NAME_VPP2_TAP1}    ip=${IP_LINUX_VPP2_TAP1}    prefix=${PREFIX}

Check Linux Interfaces On VPP1 After Resync
    ${out}=    Execute In Container    agent_vpp_1    ip a
    Should Contain    ${out}    linux_${NAME_VPP1_TAP1}

Check Interfaces On VPP1 After Resync
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
    ${int}=    Get Interface Internal Name    node=agent_vpp_1    interface=${NAME_VPP1_MEMIF1}
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    node=agent_vpp_1    interface=${NAME_VPP1_TAP1}
    Should Contain    ${out}    ${int}
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
Check Linux Interfaces On VPP2 After Resync
    ${out}=    Execute In Container    agent_vpp_2    ip a
    Should Contain    ${out}    linux_${NAME_VPP2_TAP1}

Check Interfaces On VPP2 After Resync
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
    ${int}=    Get Interface Internal Name    node=agent_vpp_2    interface=${NAME_VPP2_MEMIF1}
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    node=agent_vpp_2    interface=${NAME_VPP2_TAP1}
    Should Contain    ${out}    ${int}

Add Static Route From VPP1 Linux To VPP2 After Resync
    linux: Add Route    node=agent_vpp_1    destination_ip=${IP_VPP2_TAP1_NETWORK}    prefix=${PREFIX}    next_hop_ip=${IP_VPP1_TAP1}

Add Static Route From VPP2 Linux To VPP1 After Resync
    linux: Add Route    node=agent_vpp_2    destination_ip=${IP_VPP1_TAP1_NETWORK}    prefix=${PREFIX}    next_hop_ip=${IP_VPP2_TAP1}
    Sleep       ${SYNC_SLEEP}

Check Ping From VPP1 Linux To VPP2_TAP1 And LINUX_VPP2_TAP1 After Resync
    linux: Check Ping    node=agent_vpp_1    ip=${IP_VPP2_TAP1}
    linux: Check Ping    node=agent_vpp_1    ip=${IP_LINUX_VPP2_TAP1}

Check Ping From VPP2 Linux To VPP1_TAP1 And LINUX_VPP1_TAP1 After Resync
    linux: Check Ping    node=agent_vpp_2    ip=${IP_VPP1_TAP1}
    linux: Check Ping    node=agent_vpp_2    ip=${IP_LINUX_VPP1_TAP1}

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown