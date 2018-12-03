*** Settings ***

Library      OperatingSystem
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot
Resource     ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv6    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${SYNC_SLEEP}=       12s
${IP_1}=               fd30::1:b:0:0:1
${IP_2}=               fd30::1:b:0:0:10
${IP_3}=               fd31::1:b:0:0:10

*** Test Cases ***
Configure Environment 1
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_3

Create Infs And BD1 On VPP1
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip ${IP_1}/64 and mac 8a:f1:be:90:00:00
    Create Master memif0 on agent_vpp_1 with MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 with interfaces bvi_loop0, memif0

Add Intf And Update BD1 On VPP1
    Create Master memif1 on agent_vpp_1 with MAC 02:f1:be:90:02:00, key 2 and m1.sock socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 with interfaces bvi_loop0, memif0, memif1

Create Intfs And BD1 On VPP3
    Create loopback interface bvi_loop0 on agent_vpp_3 with ip ${IP_2}/64 and mac 8a:f1:be:90:00:03
    Create Slave memif0 on agent_vpp_3 with MAC 02:f1:be:90:00:03, key 2 and m1.sock socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_3 with interfaces bvi_loop0, memif0
    Sleep    ${SYNC_SLEEP}

Ping VPP3 From VPP1 And VPP2
    Ping6 from agent_vpp_1 to ${IP_2}

Moving Memif1 From BD1 To BD2 on VPP1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 with interfaces bvi_loop0, memif0
    Create Bridge Domain bd2 With Autolearn On agent_vpp_1 with interfaces bvi_loop1, memif1, memif2

Modify Loopback IP on VPP3
    Create loopback interface bvi_loop0 on agent_vpp_3 with ip ${IP_3}/64 and mac 8a:f1:be:90:00:03
    vpp_term: Show Interfaces    agent_vpp_3

Modify Loopback IP on VPP3 Back
    Create loopback interface bvi_loop0 on agent_vpp_3 with ip ${IP_2}/64 and mac 8a:f1:be:90:00:03
    vpp_term: Show Interfaces    agent_vpp_3

Moving Memif1 From BD2 To BD1 on VPP1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 with interfaces bvi_loop0, memif0, memif1
    Create Bridge Domain bd2 With Autolearn On agent_vpp_1 with interfaces bvi_loop1, memif2
    Sleep    ${SYNC_SLEEP}

Ping VPP3 From VPP1 And VPP2 Again
    Ping6 from agent_vpp_1 to ${IP_2}

*** Keywords ***

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown