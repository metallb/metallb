*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Force Tags        traffic     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     60s
${SYNC_SLEEP}=       3s
${RESYNC_SLEEP}=       15s

${AGENT1_VETH_MAC}=    12:11:11:11:11:11
${AGENT2_VETH_MAC}=    12:11:11:11:11:12
${AGENT3_VETH_MAC}=    12:11:11:11:11:13

${VARIABLES}=       common
${ENV}=             common


*** Test Cases ***
Configure Environment
    [Tags]    setup
    ${DATA_FOLDER}=       Catenate     SEPARATOR=/       ${CURDIR}         ${TEST_DATA_FOLDER}
    Set Suite Variable          ${DATA_FOLDER}
    Configure Environment 7

Configure Interfaces
    Put Veth Interface With IP And Namespace       node=agent_vpp_1    name=node1_veth    namespace=node_1    mac=12:11:11:11:11:11    peer=vpp1_veth1    ip=10.0.0.10
    Put Veth Interface And Namespace    node=agent_vpp_1    name=vpp1_veth1    namespace=agent_vpp_1     mac=12:12:12:12:12:11    peer=node1_veth
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth1

    Put Veth Interface With IP And Namespace       node=agent_vpp_1    name=node2_veth    namespace=node_2    mac=12:11:11:11:11:12    peer=vpp1_veth2    ip=10.0.0.11
    Put Veth Interface And Namespace    node=agent_vpp_1    name=vpp1_veth2    namespace=agent_vpp_1     mac=12:12:12:12:12:12    peer=node2_veth
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket2    mac=a2:a1:a1:a1:a1:a2    host_int=vpp1_veth2

    Put Veth Interface With IP And Namespace       node=agent_vpp_1    name=node3_veth    namespace=node_3    mac=12:11:11:11:11:13    peer=vpp1_veth3    ip=10.0.0.12
    Put Veth Interface And Namespace    node=agent_vpp_1    name=vpp1_veth3    namespace=agent_vpp_1     mac=12:12:12:12:12:13    peer=node3_veth
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket3    mac=a2:a1:a1:a1:a1:a3    host_int=vpp1_veth3

    @{ints}=    Create List    vpp1_afpacket1    vpp1_afpacket2    vpp1_afpacket3
    Put Bridge Domain    node=agent_vpp_1    name=east-west-bd    ints=${ints}

    Sleep    ${RESYNC_SLEEP}
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
    Write To Machine    vpp_agent_ctl    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -ps
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    node_1    ip a
    Execute In Container    node_2    ip a
    Execute In Container    node_3    ip a
    Make Datastore Snapshots    before_check stuff

Check Stuff
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Show Interfaces And Other Objects
    #${WAIT_TIMEOUT} in first keyword is 60s because after restart agent_vpp_1 need waiting to interface internal name
    #Bug: CV-595
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    vpp1_afpacket1    enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    vpp1_afpacket2    enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    vpp1_afpacket3    enabled=1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface With IP Is Created    node=node_1    mac=${AGENT1_VETH_MAC}      ipv4=10.0.0.10/24
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface With IP Is Created    node=node_2    mac=${AGENT2_VETH_MAC}      ipv4=10.0.0.11/24
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface With IP Is Created    node=node_3    mac=${AGENT3_VETH_MAC}      ipv4=10.0.0.12/24
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1    vpp1_afpacket1    vpp1_afpacket2     vpp1_afpacket3


TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

Suite Cleanup
    Testsuite Teardown