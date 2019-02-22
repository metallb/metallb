*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../../variables/${VARIABLES}_variables.robot

Resource     ../../../../libraries/all_libs.robot

Force Tags        traffic     IPv4    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${RESYNC_SLEEP}=       15s

${AGENT1_VETH_MAC}=    02:00:00:00:00:01
${AGENT2_VETH_MAC}=    02:00:00:00:00:02
${AGENT3_VETH_MAC}=    02:00:00:00:00:03

${VARIABLES}=       common
${ENV}=             common


*** Test Cases ***
Configure Environment
    [Tags]    setup
    ${DATA_FOLDER}=       Catenate     SEPARATOR=/       ${CURDIR}         ${TEST_DATA_FOLDER}
    Set Suite Variable          ${DATA_FOLDER}
    Configure Environment 4     veth_basic.conf
    Sleep    ${SYNC_SLEEP}
    Show Interfaces And Other Objects

Check Stuff At Beginning
    Check Stuff

Check Ping At Beginning
    Check all Pings

Remove VPP And Two Nodes
    Remove Node     agent_vpp_1
    Remove Node     node_1
    Remove Node     node_2
    Remove Node     node_3
    Sleep    ${SYNC_SLEEP}

Start VPP And Two Nodes
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Add Agent Node    node_1
    Add Agent Node    node_2
    Add Agent Node    node_3
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Resync
    Check Stuff

Check Ping After Resync
    Check all Pings

Remove VPP
    Remove Node     agent_vpp_1
    Sleep    ${SYNC_SLEEP}

Start VPP
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After VPP Restart
    Check Stuff

Check Ping After VPP Restart
    Check all Pings

Remove Node1
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start Node1
    Add Agent Node    node_1
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 Restart
    Check Stuff

Check Ping After Node1 Restart
    Check all Pings

Remove Node1 Again
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start Node1 Again
    Add Agent Node    node_1
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 Restart Again
    Check Stuff

Check Ping After Node1 Restart Again
    Check all Pings

Remove Node2
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start Node2
    Add Agent Node    node_2
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node2 Restart
    Check Stuff

Check Ping After Node2 Restart
    Check all Pings

Remove Node2 Again
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start Node2 Again
    Add Agent Node    node_2
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node2 Restart Again
    Check Stuff

Check Ping After Node2 Restart Again
    Check all Pings

Remove Node 1 and Node2
    Remove Node     node_1
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start Node 1 and Node2
    Add Agent Node    node_1
    Add Agent Node    node_2
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 and Node2 Restart
    Check Stuff

Check Ping After Node1 and Node2 Restart
    Check all Pings

Remove Node 1 and Node2 Again
    Remove Node     node_1
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start Node 1 and Node2 Again
    Add Agent Node    node_1
    Add Agent Node    node_2
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 and Node2 Restart Again
    Check Stuff

Check Ping Ater Node1 and Node2 Restart Again
    Check all Pings

Remove Node 1 and Node2 Again 2
    Remove Node     node_2
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start Node 1 and Node2 Again 2
    Add Agent Node    node_2
    Add Agent Node    node_1
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 and Node2 Restart Again 2
    Check Stuff

Check Ping After Node1 and Node2 Restart Again 2
    Check all Pings

Remove Node 1 and Node2 Again 3
    Remove Node     node_2
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start Node 1 and Node2 Again 3
    Add Agent Node    node_1
    Add Agent Node    node_2
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 and Node2 Restart Again 3
    Check Stuff

Check Ping After Node1 and Node2 Restart Again 3
    Check all Pings

Remove Node 1 and VPP
    Remove Node     node_1
    Remove Node     agent_vpp_1
    Sleep    ${SYNC_SLEEP}

Start Node 1 and VPP
    Add Agent Node    node_1
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node1 and VPP Restart
    Check Stuff

Check Ping After Node1 and VPP Restart
    Check all Pings

Remove VPP And Node1
    Remove Node     agent_vpp_1
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start VPP And Node1
    Add Agent Node    node_1
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After VPP And Node1 Restart
    Check Stuff

Check Ping After VPP And Node1 Restart
    Check all Pings

Remove VPP And Node1 Again
    Remove Node     agent_vpp_1
    Remove Node     node_1
    Sleep    ${SYNC_SLEEP}

Start VPP And Node1 Again
    Add Agent Node    node_1
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After VPP And Node1 Restart Again
    Check Stuff

Check Ping After VPP And Node1 Restart Again
    Check all Pings

Remove Node 2 and VPP
    Remove Node     node_2
    Remove Node     agent_vpp_1
    Sleep    ${SYNC_SLEEP}

Start Node 2 and VPP
    Add Agent Node    node_2
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Node2 and VPP Restart
    Check Stuff

Check Ping After Node2 and VPP Restart
    Check all Pings

Remove VPP And Node2
    Remove Node     agent_vpp_1
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start VPP And Node2
    Add Agent Node    node_2
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After VPP And Node2 Restart
    Check Stuff

Check Ping After VPP And Node2 Restart
    Check all Pings

Remove VPP And Node2 Again
    Remove Node     agent_vpp_1
    Remove Node     node_2
    Sleep    ${SYNC_SLEEP}

Start VPP And Node2 Again
    Add Agent Node    node_2
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After VPP And Node2 Restart Again
    Check Stuff

Check Ping After VPP And Node2 Restart Again
    Check all Pings

Remove All Nodes
    Remove Node     node_1
    Remove Node     node_2
    Remove Node     node_3
    Sleep    ${SYNC_SLEEP}

Start All Nodes
    Add Agent Node    node_1
    Add Agent Node    node_2
    Add Agent Node    node_3
    Sleep    ${RESYNC_SLEEP}

Check Stuff After All Nodes Restart
    Check Stuff

Check Ping After All Nodes Restart
    Check all Pings

Remove All Nodes Again
    Remove Node     node_1
    Remove Node     node_2
    Remove Node     node_3
    Sleep    ${SYNC_SLEEP}

Start All Nodes Again
    Add Agent Node    node_1
    Add Agent Node    node_2
    Add Agent Node    node_3
    Sleep    ${RESYNC_SLEEP}

Check Stuff After All Nodes Restart Again
    Check Stuff

Check Ping After All Nodes Restart Again
    Check all Pings

Remove VPP 2x
    Remove Node    agent_vpp_1
    Sleep    ${SYNC_SLEEP}

Start VPP 2x
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Remove VPP 2x
    Check Stuff

Check Ping After Remove VPP 2x
    Check all Pings

Remove VPP 3x
    Remove Node    agent_vpp_1
    Sleep    ${SYNC_SLEEP}

Start VPP 3x
    Add Agent VPP Node    agent_vpp_1    vswitch=${TRUE}
    Sleep    ${RESYNC_SLEEP}

Check Stuff After Remove VPP 3x
    Check Stuff

Check Ping After Remove VPP 3x
    Check all Pings


Done
    [Tags]    debug
    No Operation


Remove Agent Nodes Again
    Remove All Nodes

*** Keywords ***
Check all Pings
    linux: Check Ping    node_1    10.0.0.11
    linux: Check Ping    node_1    10.0.0.12
    linux: Check Ping    node_2    10.0.0.10
    linux: Check Ping    node_2    10.0.0.12
    linux: Check Ping    node_3    10.0.0.10
    linux: Check Ping    node_3    10.0.0.11

Show Interfaces And Other Objects
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_1_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    node_1    ip a
    Execute In Container    node_2    ip a
    Execute In Container    node_3    ip a
    Make Datastore Snapshots    before_check stuff

Check Stuff
    Show Interfaces And Other Objects
    vat_term: Check Afpacket Interface State    agent_vpp_1    IF_AFPIF_VSWITCH_node_1_node1_veth    enabled=1
    vat_term: Check Afpacket Interface State    agent_vpp_1    IF_AFPIF_VSWITCH_node_2_node2_veth    enabled=1
    vat_term: Check Afpacket Interface State    agent_vpp_1    IF_AFPIF_VSWITCH_node_3_node3_veth    enabled=1
    linux: Interface With IP Is Created    node=node_1    mac=${AGENT1_VETH_MAC}      ipv4=10.0.0.10/24
    linux: Interface With IP Is Created    node=node_2    mac=${AGENT2_VETH_MAC}      ipv4=10.0.0.11/24
    linux: Interface With IP Is Created    node=node_3    mac=${AGENT3_VETH_MAC}      ipv4=10.0.0.12/24
    vat_term: BD Is Created    agent_vpp_1    IF_AFPIF_VSWITCH_node_1_node1_veth    IF_AFPIF_VSWITCH_node_2_node2_veth    IF_AFPIF_VSWITCH_node_3_node3_veth


TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

Suite Cleanup
    Stop SFC Controller Container
    Testsuite Teardown
