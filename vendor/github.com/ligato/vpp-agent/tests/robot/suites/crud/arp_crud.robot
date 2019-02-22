*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv4    ExpectedFailure
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

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1


Add Veth1 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=${VETH1_MAC}    peer=vpp1_veth2    ip=10.10.1.1    prefix=24    mtu=1500
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}

Add Veth2 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH2_MAC}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=${VETH2_MAC}    peer=vpp1_veth1

Check That Veth1 And Veth2 Interfaces Are Created
    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH1_MAC}
    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH2_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_MAC}    ipv4=10.10.1.1/24    mtu=1500    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up
    vpp_term: Show Interface Mode  agent_vpp_1
    vpp_term: Show Interface Mode  agent_vpp_1


Add Memif Interface
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=62:61:61:61:61:61    master=true    id=1    ip=192.168.1.1    prefix=24    socket=default.sock

Check Memif Interface Created
    vpp_term: Interface Is Created    node=agent_vpp_1    mac=62:61:61:61:61:61
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=62:61:61:61:61:61  role=master  id=1  ipv4=192.168.1.1/24  connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock


Add VXLan Interface
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=192.168.1.1    dst=192.168.1.2    vni=5

Check VXLan Interface Created
    vxlan: Tunnel Is Created    node=agent_vpp_1    src=192.168.1.1    dst=192.168.1.2    vni=5
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan1    enabled=1    src=192.168.1.1    dst=192.168.1.2    vni=5

Add Loopback Interface
    Put Loopback Interface With IP    node=agent_vpp_1    name=vpp1_loop1    mac=12:21:21:11:11:11    ip=20.20.1.1   prefix=24   mtu=1400

Check Loopback Interface Created
    vpp_term: Interface Is Created    node=agent_vpp_1    mac=12:21:21:11:11:11
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    vpp1_loop1    enabled=1     mac=12:21:21:11:11:11    mtu=1400  ipv4=20.20.1.1/24

Add Tap Interface
    Put TAP Interface With IP    node=agent_vpp_1    name=vpp1_tap1    mac=32:21:21:11:11:11    ip=30.30.1.1   prefix=24      host_if_name=linux_vpp1_tap1

Check TAP Interface Created
    vpp_term: Interface Is Created    node=agent_vpp_1    mac=32:21:21:11:11:11
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check TAP interface State    agent_vpp_1    vpp1_tap1    mac=32:21:21:11:11:11    ipv4=30.30.1.1/24    state=up

Check Stuff
    Show Interfaces And Other Objects

Add ARPs
    Put ARP    agent_vpp_1    vpp1_memif1    155.155.155.155    32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_memif1    155.155.155.156    32:51:51:51:51:52    false
    Put ARP    agent_vpp_1    vpp1_veth1    155.155.155.155    32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_veth1    155.155.155.150    32:51:51:51:51:05    false
    Put ARP    agent_vpp_1    vpp1_veth2    155.155.155.155    32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_veth2    155.155.155.150    32:51:51:51:51:05    false
    Put ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.155    32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.154    32:51:51:51:51:53    false
    Put ARP    agent_vpp_1    vpp1_loop1    155.155.155.155   32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_loop1    155.155.155.152   32:51:51:51:51:55    false
    Put ARP    agent_vpp_1    vpp1_tap1    155.155.155.155   32:51:51:51:51:51    false
    Put ARP    agent_vpp_1    vpp1_tap1    155.155.155.150   32:51:51:51:51:05    false
    Sleep    ${SYNC_SLEEP}

Check Memif ARP
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP   agent_vpp_1     vpp1_memif1    155.155.155.155    32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP   agent_vpp_1     vpp1_memif1    155.155.155.156    32:51:51:51:51:52    True

#Check Veth1 ARP
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth1    155.155.155.155    32:51:51:51:51:51    True
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth1    155.155.155.150    32:51:51:51:51:5    True

#Check Veth2 ARP
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth2    155.155.155.155    32:51:51:51:51:51    True
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth2    155.155.155.150    32:51:51:51:51:5    True

Check VXLan ARP
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.155    32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.154    32:51:51:51:51:53    True

Check Loopback ARP
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_loop1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_loop1    155.155.155.152   32:51:51:51:51:55    True

Check TAP ARP
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_tap1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_tap1    155.155.155.150   32:51:51:51:51:05    True

ADD Afpacket Interface
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2

Check AFpacket Interface Created
    vpp_term: Interface Is Created    node=agent_vpp_1    mac=a2:a1:a1:a1:a1:a1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Afpacket Interface State    agent_vpp_1    vpp1_afpacket1    enabled=1    mac=a2:a1:a1:a1:a1:a1

Check Veth1 Veth2 Are Created After Afpacket is created
    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH1_MAC}
    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH2_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_MAC}    ipv4=10.10.1.1/24    mtu=1500    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up

Add ARP for Afpacket
    Put ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.155   32:51:51:51:51:51    False
    Put ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.150   32:51:51:51:51:05    False

Check Afpacket ARP
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.150   32:51:51:51:51:05    True

Delete ARPs
    Delete ARP    agent_vpp_1    vpp1_memif1    155.155.155.156
    Delete ARP    agent_vpp_1    vpp1_veth1    155.155.155.150
    Delete ARP    agent_vpp_1    vpp1_veth2    155.155.155.150
    Delete ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.154
    Delete ARP    agent_vpp_1    vpp1_loop1    155.155.155.152
    Delete ARP    agent_vpp_1    vpp1_tap1    155.155.155.150
    Delete ARP    agent_vpp_1    host-vpp1_veth2    155.155.155.150
    vpp_term:Show ARP   agent_vpp_1
    Execute In Container    agent_vpp_1    ip neigh
    Sleep    ${SYNC_SLEEP}

Check Memif ARP After Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP   agent_vpp_1     vpp1_memif1    155.155.155.155    32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP   agent_vpp_1     vpp1_memif1    155.155.155.156    32:51:51:51:51:52    False

#Check Veth1 ARP After Delete
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth1    155.155.155.155    32:51:51:51:51:51    True
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth1    155.155.155.15    32:51:51:51:51:5    False

#Check Veth2 ARP After Delete
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth2    155.155.155.155    32:51:51:51:51:51    True
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth2    155.155.155.15    32:51:51:51:51:5    False

Check VXLan ARP After Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.155    32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.154    32:51:51:51:51:53    False

Check Loopback ARP After Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_loop1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_loop1    155.155.155.152   32:51:51:51:51:55    False

Check TAP ARP After Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_tap1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_tap1    155.155.155.150   32:51:51:51:51:05    False

Check Afpacket ARP After Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.155   32:51:51:51:51:51    True
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.150   32:51:51:51:51:05    False

Modify ARPs
    Put ARP    agent_vpp_1    vpp1_memif1    155.155.155.155    32:51:51:51:51:58    false
    vpp_term:Show ARP   agent_vpp_1
#    Put ARP    agent_vpp_1    vpp1_veth1    155.155.155.155    32:51:51:51:51:58    false
#    vpp_term:Show ARP   agent_vpp_1
#    Put ARP    agent_vpp_1    vpp1_veth2    155.155.155.155    32:51:51:51:51:58    false
#    vpp_term:Show ARP   agent_vpp_1
    Put ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.155    32:51:51:51:51:58    false
    vpp_term:Show ARP   agent_vpp_1
    Sleep    ${SYNC_SLEEP}
    Put ARP    agent_vpp_1    vpp1_loop1    155.155.155.155   32:51:51:51:51:58    false
    vpp_term:Show ARP   agent_vpp_1
    Put ARP    agent_vpp_1    vpp1_tap1    155.155.155.155   32:51:51:51:51:58    false
    vpp_term:Show ARP   agent_vpp_1
    Put ARP    agent_vpp_1    host-vpp1_veth2    155.155.155.155   32:51:51:51:51:58    False


Check Memif ARP After Modify
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP   agent_vpp_1     vpp1_memif1    155.155.155.155    32:51:51:51:51:58    True

#Check Veth1 ARP After Modify
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth1    155.155.155.155    32:51:51:51:51:5    True

#Check Veth2 ARP After Modify
#    vpp_term: Check ARP    agent_vpp_1    vpp1_veth2    155.155.155.155    32:51:51:51:51:5    True

Check VXLan ARP After Modify
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_vxlan1    155.155.155.155    32:51:51:51:51:58    True

Check Loopback ARP After Modify
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_loop1    155.155.155.155   32:51:51:51:51:58    True

Check TAP ARP After Modify
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_tap1    155.155.155.155   32:51:51:51:51:58    True

Check Afpacket ARP After Modify
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check ARP    agent_vpp_1    vpp1_afpacket1    155.155.155.155   32:51:51:51:51:58    True


*** Keywords ***
Show Interfaces And Other Objects
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_1_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    node_1    ip a
    Make Datastore Snapshots    before_check stuff


TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
