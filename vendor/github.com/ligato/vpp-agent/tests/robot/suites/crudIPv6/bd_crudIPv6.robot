*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv6    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common

${VETH1_IP}=             fd30:0:0:1:e::1
${PREFIX}=               64
${VETH1_IP_PREFIX}=      fd30::1:e:0:0:1/64
${MEMIF1_IP}=            fd31::1:1:0:0:1
${MEMIF1_IP_PREFIX}=     fd31::1:1:0:0:1/64
${VXLAN_IP}=             fd31::1:1:0:0:2
${VXLAN_IP_PREFIX}=      fd31::1:1:0:0:1/64
${LOOPBACK_IP}=          fd32::1:1:0:0:1
${LOOPBACK_IP_PREFIX}=   fd32::1:1:0:0:1/64
${TAP_IP}=               fd33::1:1:0:0:1
${TAP_IP_PREFIX}=        fd33::1:1:0:0:1/64
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Add Interfaces For BDs
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=62:61:61:61:61:61    master=true    id=1    ip=${MEMIF1_IP}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=12:11:11:11:11:11    peer=vpp1_veth2    ip=${VETH1_IP}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=12:12:12:12:12:12    peer=vpp1_veth1
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${MEMIF1_IP}    dst=${VXLAN_IP}    vni=5
    Put Loopback Interface With IP    node=agent_vpp_1    name=vpp1_loop1    mac=12:21:21:11:11:11    ip=${LOOPBACK_IP}
    Put TAP Interface With IP    node=agent_vpp_1    name=vpp1_tap1    mac=32:21:21:11:11:11    ip=${TAP_IP}    host_if_name=linux_vpp1_tap1
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif2    mac=62:61:61:61:61:62    master=true    id=2    ip=${VXLAN_IP}
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan2    src=192.168.2.1    dst=192.168.2.2    vni=15
    Put Loopback Interface With IP    node=agent_vpp_1    name=bvi_vpp1_loop2    mac=12:21:21:11:11:12    ip=20.20.2.1
    Put Loopback Interface With IP    node=agent_vpp_1    name=bvi_vpp1_loop3    mac=12:21:21:11:11:13    ip=20.20.3.1

Add BD1 Bridge Domain
    @{ints}=    Create List   vpp1_memif1  vpp1_vxlan1    vpp1_afpacket1
    vat_term: BD Not Exists    agent_vpp_1    @{ints}
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}    flood=true    unicast=true    forward=true    learn=true    arp_term=true

Check BD1 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1    vpp1_memif1    vpp1_afpacket1    vpp1_vxlan1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd1  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=vpp1_memif1  interface=vpp1_afpacket1  interface=vpp1_vxlan1  bvi_int=none

Add BD2 Bridge Domain
    @{ints}=    Create List   vpp1_memif2  vpp1_vxlan2    bvi_vpp1_loop3
    vat_term: BD Not Exists    agent_vpp_1    @{ints}
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd2    ints=${ints}    flood=true    unicast=true    forward=true    learn=true    arp_term=true

Check BD2 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1    vpp1_memif2    vpp1_vxlan2    bvi_vpp1_loop3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd2  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=vpp1_memif2  interface=vpp1_vxlan2  interface=bvi_vpp1_loop3  bvi_int=bvi_vpp1_loop3

Check That BD1 Is Not Affected By Adding BD2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd1  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=vpp1_memif1  interface=vpp1_afpacket1  interface=vpp1_vxlan1  bvi_int=none

Update BD1
    @{ints}=    Create List   vpp1_memif1  vpp1_vxlan1    bvi_vpp1_loop2
    vat_term: BD Not Exists    agent_vpp_1    @{ints}
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}    flood=false    unicast=false    forward=false    learn=false    arp_term=false
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Deleted    agent_vpp_1    vpp1_memif1    vpp1_afpacket1    vpp1_vxlan1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Created    agent_vpp_1    vpp1_memif1    vpp1_vxlan1    bvi_vpp1_loop2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd1  flood=0  unicast=0  forward=0  learn=0  arp_term=0  interface=vpp1_memif1  interface=vpp1_vxlan1  interface=bvi_vpp1_loop2  bvi_int=bvi_vpp1_loop2

Check That BD2 Is Not Affected By Updating BD1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd2  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=vpp1_memif2  interface=vpp1_vxlan2  interface=bvi_vpp1_loop3  bvi_int=bvi_vpp1_loop3

Delete VXLan1 Interface
    Delete VPP Interface    node=agent_vpp_1    name=vpp1_vxlan1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Deleted    node=agent_vpp_1    src=192.168.1.1    dst=192.168.1.2    vni=5

Check That VXLan1 Interface Is Deleted From BD1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Deleted    agent_vpp_1    vpp1_memif1    vpp1_vxlan1    bvi_vpp1_loop2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd1  flood=0  unicast=0  forward=0  learn=0  arp_term=0  interface=vpp1_memif1  interface=bvi_vpp1_loop2  bvi_int=bvi_vpp1_loop2

Read VXLan1 Interface
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=192.168.1.1    dst=192.168.1.2    vni=5
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Created    node=agent_vpp_1    src=192.168.1.1    dst=192.168.1.2    vni=5

Check That VXLan1 Interface Is Added To BD1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd1  flood=0  unicast=0  forward=0  learn=0  arp_term=0  interface=vpp1_memif1  interface=vpp1_vxlan1  interface=bvi_vpp1_loop2  bvi_int=bvi_vpp1_loop2

Delete BD1 Bridge Domain
    Delete Bridge Domain    agent_vpp_1    vpp1_bd1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: BD Is Deleted    agent_vpp_1    vpp1_memif1    vpp1_vxlan1    bvi_vpp1_loop2

Check That BD2 Is Not Affected By Deleting BD1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Bridge Domain State    agent_vpp_1  vpp1_bd2  flood=1  unicast=1  forward=1  learn=1  arp_term=1  interface=vpp1_memif2  interface=vpp1_vxlan2  interface=bvi_vpp1_loop3  bvi_int=bvi_vpp1_loop3

Show Interfaces And Other Objects After Test
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

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

