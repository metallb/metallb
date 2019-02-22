*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../../variables/${VARIABLES}_variables.robot

Resource     ../../../../libraries/all_libs.robot

Force Tags        sfc     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
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
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent VPP Node    agent_vpp_2
    Add Agent Node    agent_1
    ${DATA_FOLDER}=       Catenate     SEPARATOR=/       ${CURDIR}         ${TEST_DATA_FOLDER}
    Set Suite Variable          ${DATA_FOLDER}
    Start SFC Controller Container With Own Config    basic.conf
    Sleep    ${SYNC_SLEEP}

Check Interfaces Created
    Check Stuff

Check Ping VPP2 -> Agent1
    vpp_term: Check Ping    agent_vpp_2    10.0.0.10

Check Ping Agent1 -> VPP2
    linux: Check Ping    agent_1    10.0.0.1

Remove Agent Nodes
    #Remove All Nodes
    Remove Node    agent_vpp_1
    #Remove Node    agent_vpp_2
    Remove Node    agent_1
    Sleep     ${SYNC_SLEEP}

Start Agent Nodes Again
    #Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    #Add Agent VPP Node    agent_vpp_2
    Add Agent Node    agent_1
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    #Sleep    ${SYNC_SLEEP}

Check Interfaces After Resync
    Check Stuff

Check Ping VPP2 -> Agent1 After Resync
    vpp_term: Check Ping    agent_vpp_2    10.0.0.10

Check Ping Agent1 -> VPP2 After Resync
    linux: Check Ping    agent_1    10.0.0.1

Remove Agent Nodes Again
    #Remove All Nodes
    Remove Node    agent_vpp_1
    #Remove Node    agent_vpp_2
    Remove Node    agent_1
    Sleep     ${SYNC_SLEEP}

Start Agent Nodes Again2
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent Node    agent_1

Check Interfaces After Resync2
    Check Stuff

Check Ping VPP2 -> Agent1 After Resync2
    vpp_term: Check Ping    agent_vpp_2    10.0.0.10

Check Ping Agent1 -> VPP2 After Resync2
    linux: Check Ping    agent_1    10.0.0.1

Remove All Agent Nodes
    Remove All Nodes
    Sleep     ${SYNC_SLEEP}

Start All Agent Nodes
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent VPP Node    agent_vpp_2
    Add Agent Node    agent_1

Check Interfaces After Resync3
    Check Stuff

Check Ping VPP2 -> Agent1 After Resync3
    vpp_term: Check Ping    agent_vpp_2    10.0.0.10

Check Ping Agent1 -> VPP2 After Resync3
    linux: Check Ping    agent_1    10.0.0.1


*** Keywords ***

Check Stuff
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  IF_MEMIF_VSWITCH_agent_vpp_2_vpp2_memif1  role=master  connected=1  enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    IF_AFPIF_VSWITCH_agent_1_agent1_afp1    enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=02:02:02:02:02:02  role=slave  ipv4=10.0.0.1/24  connected=1  enabled=1
    Show Interfaces And Other Objects

Show Interfaces And Other Objects
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_2_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_2_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_2_term    show br
    Write To Machine    agent_vpp_1_term    show br 1 detail
    Write To Machine    agent_vpp_2_term    show br 1 detail
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Write To Machine    agent_vpp_2_term    show vxlan tunnel
    Write To Machine    agent_vpp_1_term    show err
    Write To Machine    agent_vpp_2_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    vat_term: Interfaces Dump    agent_vpp_2
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_2    ip a
    Execute In Container    agent_1    ip a

Suite Cleanup
    Stop SFC Controller Container
    Testsuite Teardown

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
