*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${VETH1_MAC}=          1a:00:00:11:11:11
${VETH2_MAC}=          2a:00:00:22:22:22
${AFP1_MAC}=           a2:01:01:01:01:01
${NAMESPACE}=
${NSTYPE}=            3
${VETH1_IP}=             fd30::1:e:0:0:1
${PREFIX}=               64
${VETH1_IP_PREFIX}=      fd30::1:e:0:0:1/64
${ARP_IP1}=              fd33::1:a:0:0:1
${ARP_IP2}=              fd33::1:a:0:0:2
${ARP_IP3}=              fd33::1:b:0:0:1
${ARP_IP4}=              fd33::1:b:0:0:2
*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    Show Info

Add Veth1 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=${VETH1_MAC}    peer=vpp1_veth2    ip=${VETH1_IP}    prefix=${PREFIX}    mtu=1500
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}

Add Veth2 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH2_MAC}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=${VETH2_MAC}    peer=vpp1_veth1
    Show Info

Check That Veth1 And Veth2 Interfaces Are Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH1_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH2_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_MAC}    ipv6=${VETH1_IP_PREFIX}    mtu=1500    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up


ADD Afpacket Interface
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2

Check AFpacket Interface Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=a2:a1:a1:a1:a1:a1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    vpp1_afpacket1    enabled=1    mac=a2:a1:a1:a1:a1:a1


Add ARPs
    Put Linux ARP    agent_vpp_1    vpp1_veth1  veth1_arp  ${ARP_IP1}    32:51:51:51:51:51
    Put Linux ARP    agent_vpp_1    vpp1_veth2  veth2_arp  ${ARP_IP2}    32:51:51:51:51:52
    #Put Linux ARP    agent_vpp_1    eth0        eth_arp  ${ARP_IP2}    32:51:51:51:51:52

Check ARPSs
    ${out}=       Execute In Container    agent_vpp_1    ip neigh
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Contain     ${out}    ${ARP_IP2} dev vpp1_veth2 lladdr 32:51:51:51:51:52 PERMANENT
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Contain     ${out}    ${ARP_IP1} dev vpp1_veth1 lladdr 32:51:51:51:51:51 PERMANENT
    #Should Contain     ${out}    ${ARP_IP2} dev eth0 lladdr 32:51:51:51:51:52 PERMANENT

Change ARPs
    Put Linux ARP    agent_vpp_1    vpp1_veth1  veth1_arp  ${ARP_IP3}    32:61:51:51:51:51
    Put Linux ARP    agent_vpp_1    vpp1_veth2  veth2_arp  ${ARP_IP4}    32:61:51:51:51:52
    #Put Linux ARP    agent_vpp_1    eth0        eth_arp  ${ARP_IP4}    32:61:51:51:51:52

Check ARPSs Again
    ${out}=       Execute In Container    agent_vpp_1    ip neigh
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Contain     ${out}    ${ARP_IP4} dev vpp1_veth2 lladdr 32:61:51:51:51:52 PERMANENT
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Contain     ${out}    ${ARP_IP3} dev vpp1_veth1 lladdr 32:61:51:51:51:51 PERMANENT
    #Should Contain     ${out}    ${ARP_IP4} dev eth0 lladdr 32:61:51:51:51:52 PERMANENT

Delete ARPs
    Delete Linux ARP    agent_vpp_1    veth1_arp
    Delete Linux ARP    agent_vpp_1    veth2_arp
    #Delete Linux ARP    agent_vpp_1    eth_arp

Check ARPSs After Delete
    ${out}=       Execute In Container    agent_vpp_1    ip neigh
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Not Contain     ${out}    ${ARP_IP4} dev vpp1_veth2 lladdr 32:61:51:51:51:52 PERMANENT
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Should Not Contain     ${out}    ${ARP_IP3} dev vpp1_veth1 lladdr 32:61:51:51:51:51 PERMANENT
    #Should Not Contain     ${out}    ${ARP_IP4} dev eth0 lladdr 32:61:51:51:51:52 PERMANENT


*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

Show Info
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_1    ip neigh
    vpp_term:Show ARP   agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_1
