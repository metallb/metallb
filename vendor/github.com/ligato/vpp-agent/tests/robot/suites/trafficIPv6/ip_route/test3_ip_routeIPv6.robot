*** Settings ***

Library     OperatingSystem
Library     String
#Library     RequestsLibrary

Resource     ../../../variables/${VARIABLES}_variables.robot
Resource    ../../../libraries/all_libs.robot
Resource    ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${IP_1}=               fd30::1:b:0:0:1
${IP_2}=               fd30::1:b:0:0:2
${IP_3}=               fd31::1:b:0:0:1
${IP_4}=               fd31::1:b:0:0:2
${NET1}=               fd30:0:0:1::
${NET2}=               fd31:0:0:1::
${MAC_LOOP1}=          8a:f1:be:90:00:00
${MAC_LOOP2}=          8a:f1:be:90:02:00
${MAC_MEMIF1}=         02:f1:be:90:00:00
${MAC_MEMIF2}=         02:f1:be:90:02:00

${MAC2_LOOP1}=          8a:f1:be:90:00:02
${MAC3_LOOP1}=          8a:f1:be:90:00:03
${MAC2_MEMIF1}=         02:f1:be:90:00:02
${MAC3_MEMIF1}=         02:f1:be:90:00:03

${PREFIX}=             64
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${SYNC_WAIT}=          25s
*** Test Cases ***
# Default VRF table ...
Start Three Agents

    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3
    Sleep    ${SYNC_WAIT}

Setup Agent1 for agent2
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip ${IP_1}/${PREFIX} and mac ${MAC_LOOP1}
    Create Master memif0 on agent_vpp_1 with MAC ${MAC_MEMIF1}, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_1 with interfaces bvi_loop0, memif0


Check Interfaces on Agent1 for Agent2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_1}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m0.sock

Create BD on Agent1
    Create bridge domain bd1 With Autolearn on agent_vpp_1 with interfaces bvi_loop0, memif0


Check1 bd1 on Agent1 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1   bvi_loop0     memif0
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  bd1  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=memif0  interface=bvi_loop0

Check2 Interfaces on Agent1 for Agent2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop0    enabled=1     mac=${MAC_LOOP1}   ipv6=${IP_1}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif0  mac=${MAC_MEMIF1}  role=master  id=1   connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m0.sock

Setup Agent1 for Agent3
    Create loopback interface bvi_loop1 on agent_vpp_1 with ip ${IP_3}/${PREFIX} and mac ${MAC_LOOP2}
    Create Master memif1 on agent_vpp_1 with MAC ${MAC_MEMIF2}, key 2 and m1.sock socket
    Create bridge domain bd2 With Autolearn on agent_vpp_1 with interfaces bvi_loop1, memif1

Check Interfaces on Agent1 for Agent3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    bvi_loop1    enabled=1     mac=${MAC_LOOP2}   ipv6=${IP_3}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  memif1  mac=${MAC_MEMIF2}  role=master  id=2   connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/m1.sock

Check bd2 on Agent1 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1   bvi_loop1     memif1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  bd2  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=memif1  interface=bvi_loop1

Setup Agent2
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With VRF 2, Ip ${IP_2}/${PREFIX} And Mac ${MAC2_LOOP1}
    Create Slave memif0 on agent_vpp_2 with MAC ${MAC2_MEMIF1}, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_2 with interfaces bvi_loop0, memif0


Check Interfaces on Agent2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_2    bvi_loop0    enabled=1     mac=${MAC2_LOOP1}   ipv6=${IP_2}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  memif0  mac=${MAC2_MEMIF1}  role=slave  id=1   connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/m0.sock

Check bd1 on Agent2 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_2   bvi_loop0     memif0
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_2  bd1  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=memif0  interface=bvi_loop0

Setup Agent3
    Create Loopback Interface bvi_loop0 On agent_vpp_3 With VRF 3, Ip ${IP_4}/${PREFIX} And Mac ${MAC3_LOOP1}
    Create Slave memif0 on agent_vpp_3 with MAC ${MAC3_MEMIF1}, key 2 and m1.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_3 with interfaces bvi_loop0, memif0


Check Interfaces on Agent3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_3    bvi_loop0    enabled=1     mac=${MAC3_LOOP1}   ipv6=${IP_4}/${PREFIX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_3  memif0  mac=${MAC3_MEMIF1}  role=slave  id=2   connected=1  enabled=1  socket=${AGENT_VPP_3_MEMIF_SOCKET_FOLDER}/m1.sock

Check bd1 on Agent3 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_3   bvi_loop0     memif0
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_3  bd1  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=memif0  interface=bvi_loop0

Create Routes
    Create Route On agent_vpp_2 With IP ${NET2}/64 With Next Hop ${IP_1} And Vrf Id 2
    Create Route On agent_vpp_3 With IP ${NET1}/64 With Next Hop ${IP_3} And Vrf Id 3
    Sleep    ${SYNC_WAIT}
    Sleep    ${SYNC_WAIT}

Check Interfaces And FIB
    Show Interfaces On agent_vpp_1
    Show Interfaces Address On agent_vpp_2
    Show IP6 Fib On agent_vpp_2
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    Show Interfaces Address On agent_vpp_3
    Show IP6 Fib On agent_vpp_3
    IP6 Fib Table 3 On agent_vpp_3 Should Contain Route With IP ${NET1}/64

Check Pinging
    # try ping
    Ping6 From agent_vpp_1 To ${IP_2}
    Ping6 From agent_vpp_1 To ${IP_4}
    #Ping From agent_vpp_2 To ${IP_4}

    ${int}=    Get Interface Internal Name    agent_vpp_2    bvi_loop0
    Ping On agent_vpp_2 With IP ${IP_4}, Source ${int}
    #Ping From agent_vpp_3 To ${IP_2}
    ${int}=    Get Interface Internal Name    agent_vpp_3    bvi_loop0
    Ping On agent_vpp_3 With IP ${IP_2}, Source ${int}


*** Keywords ***
List of interfaces On ${node} Should Contain Interface ${int}
    ${out}=   vpp_term: Show Interfaces    ${node}
    Should Match Regexp        ${out}  ${int}

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown