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

Start 2 Agents
    Add Agent VPP Node                 agent_vpp_1
    Add Agent VPP Node                 agent_vpp_2
    Sleep    ${SYNC_SLEEP}

Create Loopback Interface on Agent1
    Create Loopback Interface bvi_loop0 On agent_vpp_1 With Ip ${IP_1}/${PREFIX} And Mac ${MAC_LOOP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_1}/${PREFIX}

Create Memif Interface on Agent1
    Create Master memif0 On agent_vpp_1 With MAC ${MAC_MEMIF1}, Key 1 And m1.sock Socket
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock

Create Loopback Interface on Agent2
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With Ip ${IP_2}/${PREFIX} And Mac ${MAC_LOOP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_2    bvi_loop0    enabled=1     mac=${MAC_LOOP2}   ipv6=${IP_2}/${PREFIX}

Create Memif Interface on Agent2
    Create Slave memif0 On agent_vpp_2 With MAC ${MAC_MEMIF2}, Key 1 And m1.sock Socket
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  memif0  mac=${MAC_MEMIF2}  role=slave  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock

Create BD on Agent1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, memif0

Create BD on Agent2
    Create Bridge Domain bd1 With Autolearn On agent_vpp_2 With Interfaces bvi_loop0, memif0

Check Created Interfaces Again2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_1}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_2    bvi_loop0    enabled=1     mac=${MAC_LOOP2}   ipv6=${IP_2}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  memif0  mac=${MAC_MEMIF2}  role=slave  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock

Check Traffic VPP1-VPP2
    Ping6 From agent_vpp_1 To ${IP_2}
    Ping6 From agent_vpp_2 To ${IP_1}

Create 3. agent and Interfaces
    Add Agent VPP Node                 agent_vpp_3
    Sleep    ${SYNC_SLEEP}

Create Memif2 Interface on Agent1
    Create Master memif1 On agent_vpp_1 With MAC ${MAC_MEMIF3}, Key 2 And m2.sock Socket
    Sleep    ${SYNC_SLEEP}
    vat_term: Check Memif Interface State     agent_vpp_1  memif1  mac=${MAC_MEMIF3}  role=master  id=2   connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m2.sock

Create Loopback Interface on Agent3
    Create Loopback Interface bvi_loop0 On agent_vpp_3 With Ip ${IP_3}/64 And Mac ${MAC_LOOP3}
    Sleep    ${SYNC_SLEEP}
    vat_term: Check Loopback Interface State    agent_vpp_3    bvi_loop0    enabled=1     mac=${MAC_LOOP3}   ipv6=${IP_3}/${PREFIX}

Create Memif Interface on Agent3
    Create Slave memif0 On agent_vpp_3 With MAC ${MAC_MEMIF4}, Key 2 And m2.sock Socket
    Sleep    ${SYNC_SLEEP}
    vat_term: Check Memif Interface State     agent_vpp_3  memif0  mac=${MAC_MEMIF4}  role=slave  id=2   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m2.sock


Create BD on Agent1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, memif0, memif1

Create BD on Agent3
    Create Bridge Domain bd1 With Autolearn On agent_vpp_3 With Interfaces bvi_loop0, memif0

Check Created Interfaces Again3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_1}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_2    bvi_loop0    enabled=1     mac=${MAC_LOOP2}   ipv6=${IP_2}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_3    bvi_loop0    enabled=1     mac=${MAC_LOOP3}   ipv6=${IP_3}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  memif0  mac=${MAC_MEMIF2}  role=slave  id=1   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif1  mac=${MAC_MEMIF3}  role=master  id=2   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m2.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_3  memif0  mac=${MAC_MEMIF4}  role=slave  id=2   connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m2.sock


Check Traffic VPP2-VPP3
    Ping6 From agent_vpp_2 To ${IP_3}
    Ping6 From agent_vpp_3 To ${IP_2}
    Ping6 From agent_vpp_3 To ${IP_1}

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown