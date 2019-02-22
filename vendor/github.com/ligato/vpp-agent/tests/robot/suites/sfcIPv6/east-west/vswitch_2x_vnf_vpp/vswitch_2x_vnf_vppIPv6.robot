*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../../variables/${VARIABLES}_variables.robot

Resource     ../../../../libraries/all_libs.robot

Force Tags        sfc     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${IP_1}=               fd30::1:b:0:0:1
${IP_2}=               fd30::1:b:0:0:10


*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3
    ${DATA_FOLDER}=       Catenate     SEPARATOR=/       ${CURDIR}         ${TEST_DATA_FOLDER}
    Set Suite Variable          ${DATA_FOLDER}
    Start SFC Controller Container With Own Config    basicIPv6.conf


Check Memifs On Vswitch
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  IF_MEMIF_VSWITCH_agent_vpp_2_vpp2_memif1  role=master  connected=1  enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  IF_MEMIF_VSWITCH_agent_vpp_3_vpp3_memif1  role=master  connected=1  enabled=1

Check Memif Interface On VPP2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=02:02:02:02:02:02  role=slave  ipv6=${IP_1}/64  connected=1  enabled=1

Check Memif Interface On VPP3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_3  vpp3_memif1  role=slave  ipv6=${IP_2}/64  connected=1  enabled=1

Show Interfaces And Other Objects
    Check Stuff

Check Ping Agent2 -> Agent3
    vpp_term: Check Ping    agent_vpp_2    ${IP_2}

Check Ping Agent3 -> Agent2
    vpp_term: Check Ping    agent_vpp_3    ${IP_1}

Remove Agent Nodes
    Remove All Nodes

Start Agent Nodes Again
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3


Check Memifs On Vswitch After Resync
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  IF_MEMIF_VSWITCH_agent_vpp_2_vpp2_memif1  role=master  connected=1  enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  IF_MEMIF_VSWITCH_agent_vpp_3_vpp3_memif1  role=master  connected=1  enabled=1

Check Memif Interface On VPP2 After Resync
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=02:02:02:02:02:02  role=slave  ipv6=${IP_1}/64  connected=1  enabled=1

Check Memif Interface On VPP3 After Resync
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_3  vpp3_memif1  role=slave  ipv6=${IP_2}/64  connected=1  enabled=1

Show Interfaces And Other Objects After Resync
    Check Stuff

Check Ping Agent2 -> Agent3 After Resync
    vpp_term: Check Ping    agent_vpp_2    ${IP_2}

Check Ping Agent3 -> Agent2 After Resync
    vpp_term: Check Ping    agent_vpp_3    ${IP_1}

*** Keywords ***
Check Stuff
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2
    vpp_term: Show Interfaces    agent_vpp_3
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_2_term    show int addr
    Write To Machine    agent_vpp_3_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_2_term    show h
    Write To Machine    agent_vpp_3_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_2_term    show br
    Write To Machine    agent_vpp_3_term    show br
    Write To Machine    agent_vpp_1_term    show br 1 detail
    Write To Machine    agent_vpp_2_term    show br 1 detail
    Write To Machine    agent_vpp_3_term    show br 1 detail
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Write To Machine    agent_vpp_2_term    show vxlan tunnel
    Write To Machine    agent_vpp_3_term    show vxlan tunnel
    Write To Machine    agent_vpp_1_term    show err
    Write To Machine    agent_vpp_2_term    show err
    Write To Machine    agent_vpp_3_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    vat_term: Interfaces Dump    agent_vpp_2
    vat_term: Interfaces Dump    agent_vpp_3
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_2    ip a
    Execute In Container    agent_vpp_3    ip a

Suite Cleanup
    Stop SFC Controller Container
    Testsuite Teardown

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown