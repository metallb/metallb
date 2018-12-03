*** Settings ***

Library      OperatingSystem
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot
Resource     ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv4    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${SYNC_SLEEP}=       3s

*** Test Cases ***
Configure Environment 1
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1
    #Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3
    Sleep    ${SYNC_SLEEP}

Create 2 Loopbacks And Memifs And BD On VPP1
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip 10.1.1.1/24 and mac 8a:f1:be:90:00:00
    Create Master memif0 on agent_vpp_1 with MAC 02:f1:be:90:02:00, key 2 and m1.sock socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 with interfaces bvi_loop0, memif0
    vpp_ctl: Put Loopback Interface With IP    node=agent_vpp_1    name=bvi_loop1    mac=8a:f1:be:90:01:00    ip=10.1.1.100    prefix=24    vrf=20    enabled=true
    vpp_ctl: Put Memif Interface    node=agent_vpp_1    name=memif1    mac=02:f1:be:90:03:00    master=true    id=3       socket=m2.sock    vrf=20
    Create Bridge Domain bd2 With Autolearn On agent_vpp_1 with interfaces bvi_loop1, memif1


Create 2 Memifs On VPP3
    Create Slave memif0 on agent_vpp_3 with MAC 02:f1:be:90:00:03, key 2 and m1.sock socket
    vpp_ctl: Put Memif Interface    node=agent_vpp_3    name=memif1    mac=02:f1:be:90:03:03    master=false    id=3       socket=m2.sock    vrf=20


Ping Loopback1 X Loopback2
    vpp_term: Check No Ping Within Interface    agent_vpp_1     10.1.1.100    loop0    15
    vpp_term: Check No Ping Within Interface    agent_vpp_1     10.1.1.1    loop1    15

Add L2XConnect for Memif1 and Memif2 On VPP3
    vpp_ctl: Put L2XConnect  agent_vpp_3    memif0    memif1
    vpp_ctl: Put L2XConnect  agent_vpp_3    memif1    memif0

Add Trace for Memif
    vpp_term: Add Trace Memif     agent_vpp_1
    vpp_term: Add Trace Memif     agent_vpp_3

Check Memif1 and Memif2 in XConnect mode on VPP3
    ${out}=      vpp_term: Show Interface Mode    agent_vpp_3
    Should Contain     ${out}      l2 xconnect memif0/2 memif1/3
    Should Contain     ${out}      l2 xconnect memif1/3 memif0/2

Ping Loopback1 -> Loopback2
    vpp_term: Check Ping Within Interface    agent_vpp_1     10.1.1.100    loop0    15

Ping Loopback2 -> Loopback1
    vpp_term: Check Ping Within Interface    agent_vpp_1     10.1.1.1    loop1    15

Show Traces
    vpp_term: Show Trace     agent_vpp_1
    vpp_term: Show Trace     agent_vpp_3

*** Keywords ***

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots
