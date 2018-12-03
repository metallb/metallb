*** Settings ***

Library      OperatingSystem
Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot
Resource     ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common

${MAC_LOOP1}=          8a:f1:be:90:00:00
${MAC_LOOP2}=          8a:f1:be:90:00:02
${MAC_LOOP3}=          8a:f1:be:90:00:03
${MAC_MEMIF1}=         02:f1:be:90:00:00
${MAC_MEMIF2}=         02:f1:be:90:00:02
${MAC_MEMIF3}=         02:f1:be:90:00:10
${MAC_MEMIF4}=         02:f1:be:90:00:03
${IP_1}=               fd30::1:b:0:0:1
${IP_2}=               fd30::1:b:0:0:2
${IP_3}=               fd30::1:b:0:0:3
${IP_4}=               fd31::1:b:0:0:1
${IP_5}=               fd31::1:b:0:0:2

${PREFIX}=             64
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
*** Test Cases ***
First configure Bridge Domain with Memif interfaces and VXLan then add two agents and try traffic

    Create Master memif0 On agent_vpp_1 With IP ${IP_1}, MAC ${MAC_MEMIF1}, Key 1 And m0.sock Socket
    Create Slave memif0 On agent_vpp_2 With IP ${IP_2}, MAC ${MAC_MEMIF2}, Key 1 And m0.sock Socket
    Create Loopback Interface bvi_loop0 On agent_vpp_1 With Ip ${IP_4}/64 And Mac ${MAC_LOOP1}
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With Ip ${IP_5}/64 And Mac ${MAC_LOOP2}
    Create VXLan vxlan1 From ${IP_1} To ${IP_2} With Vni 13 On agent_vpp_1
    Create VXLan vxlan1 From ${IP_2} To ${IP_1} With Vni 13 On agent_vpp_2
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, vxlan1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_2 With Interfaces bvi_loop0, vxlan1

Start Agents
    Add Agent VPP Node                 agent_vpp_1
    Add Agent VPP Node                 agent_vpp_2

Check Created Interfaces
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}     vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_4}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}     vat_term: Check Loopback Interface State    agent_vpp_2    bvi_loop0    enabled=1     mac=${MAC_LOOP2}   ipv6=${IP_5}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}     vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m0.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}     vat_term: Check Memif Interface State     agent_vpp_2  memif0  mac=${MAC_MEMIF2}  role=slave  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m0.sock

Pinging
    Ping6 From agent_vpp_1 To ${IP_2}
    Ping6 From agent_vpp_2 To ${IP_1}
    Ping6 From agent_vpp_1 To ${IP_5}
    Ping6 From agent_vpp_2 To ${IP_4}

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown