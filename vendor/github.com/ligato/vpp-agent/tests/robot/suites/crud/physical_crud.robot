*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv4    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    ${phys_ints}=    Create List    1    2
    Add Agent VPP Node With Physical Int    agent_vpp_1    ${phys_ints}

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Check That Physical Interfaces Exists And Are Not Configured
# int 1
    vpp_term: Interface Is Down    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    ${ipv4_list}=     vpp_term: Get Interface IPs    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    Lists Should Be Equal    ${ipv4_list}    ${EMPTY}
# int 2
    vpp_term: Interface Is Down    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    ${ipv4_list}=     vpp_term: Get Interface IPs    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    Lists Should Be Equal    ${ipv4_list}    ${EMPTY}

Add Physical1 Interface
    Put Physical Interface With IP    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_1_VPP_NAME}    ip=10.11.1.2    prefix=28    mtu=1500

Check That Physical1 Interface Is Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Enabled    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_1_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_1_MAC}    ipv4=10.11.1.2/28    mtu=1500

Add Physical2 Interface
    Put Physical Interface With IP    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_2_VPP_NAME}    ip=20.21.2.3    prefix=24    mtu=2500

Check That Physical2 Interface Is Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Enabled    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_2_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_2_MAC}    ipv4=20.21.2.3/24    mtu=2500

Check That Physical1 Interface Is Still Configured
    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_1_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_1_MAC}    ipv4=10.11.1.2/28    mtu=1500

Update Physical1 Interface
    Put Physical Interface With IP    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_1_VPP_NAME}    ip=30.31.3.3    prefix=26    mtu=1600
    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_1_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_1_MAC}    ipv4=30.31.3.3/26    mtu=1600

Check That Physical2 Interface Is Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_2_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_2_MAC}    ipv4=20.21.2.3/24    mtu=2500

Delete Physical2 Interface
    Delete VPP Interface    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    vpp_term: Interface Is Disabled    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_2_VPP_NAME}

Check That Physical2 Interface Is Unconfigured
    ${ipv4_list}=     vpp_term: Get Interface IPs    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    Lists Should Be Equal    ${ipv4_list}    ${EMPTY}

Check That Physical1 Interface Is Not Affected By Delete Physical2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Physical Interface State    agent_vpp_1    ${DOCKER_PHYSICAL_INT_1_VPP_NAME}    enabled=1    mac=${DOCKER_PHYSICAL_INT_1_MAC}    ipv4=30.31.3.3/26    mtu=1600

Delete Physical1 interface
    Delete VPP Interface    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    vpp_term: Interface Is Disabled    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_1_VPP_NAME}

Check That Physical1 Interface Is Unconfigured
    ${ipv4_list}=     vpp_term: Get Interface IPs    node=agent_vpp_1    interface=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    Lists Should Be Equal    ${ipv4_list}    ${EMPTY}

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

