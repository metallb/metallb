*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Force Tags        traffic     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown
*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${CONFIG_SLEEP}=       1s
${RESYNC_SLEEP}=       1s
${SYNC_SLEEP}=         10s
# wait for resync vpps after restart
${RESYNC_WAIT}=        20s
${NET1_IP1}=            fd30::1:a:0:0:1
${NET1_IP2}=            fd30::1:a:0:0:2
${NET2_IP1}=            fd31::1:a:0:0:1
${NET2_IP2}=            fd31::1:a:0:0:2
${NET3_IP1}=            fd32::1:a:0:0:1
${NET3_IP2}=            fd32::1:a:0:0:2
${NET4_IP1}=            fd33::1:a:0:0:1
${NET4_IP2}=            fd33::1:a:0:0:2

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Setup Interfaces
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=62:61:61:61:61:61    master=true    id=1    ip=${NET1_IP1}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=12:11:11:11:11:11    peer=vpp1_veth2    ip=${NET2_IP1}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=12:12:12:12:12:12    peer=vpp1_veth1
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${NET1_IP1}    dst=${NET1_IP2}    vni=5
    @{ints}=    Create List    vpp1_vxlan1    vpp1_afpacket1
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}
    Put Loopback Interface With IP    node=agent_vpp_1    name=vpp1_loop1    mac=12:21:21:11:11:11    ip=${NET3_IP1}
    Put TAP Interface With IP    node=agent_vpp_1    name=vpp1_tap1    mac=32:21:21:11:11:11    ip=${NET4_IP1}    host_if_name=linux_vpp1_tap1

    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif1    mac=62:62:62:62:62:62    master=false    id=1    ip=${NET1_IP2}
    Put Veth Interface With IP    node=agent_vpp_2    name=vpp2_veth1    mac=22:21:21:21:21:21    peer=vpp2_veth2    ip=${NET2_IP2}
    Put Veth Interface    node=agent_vpp_2    name=vpp2_veth2    mac=22:22:22:22:22:22    peer=vpp2_veth1
    Put Afpacket Interface    node=agent_vpp_2    name=vpp2_afpacket1    mac=a2:a2:a2:a2:a2:a2    host_int=vpp2_veth2
    Put VXLan Interface    node=agent_vpp_2    name=vpp2_vxlan1    src=${NET1_IP2}    dst=${NET1_IP1}    vni=5
    @{ints}=    Create List    vpp2_vxlan1    vpp2_afpacket1
    Put Bridge Domain    node=agent_vpp_2    name=vpp2_bd1    ints=${ints}
    Put Loopback Interface With IP    node=agent_vpp_2    name=vpp2_loop1    mac=22:21:21:11:11:11    ip=${NET3_IP2}
    Put TAP Interface With IP    node=agent_vpp_2    name=vpp2_tap1    mac=32:22:22:11:11:11    ip=${NET4_IP2}    host_if_name=linux_vpp2_tap1
    Sleep    ${SYNC_SLEEP}

Check Linux Interfaces On VPP1
    ${out}=    Execute In Container    agent_vpp_1    ip a
    Should Contain    ${out}    vpp1_veth2@vpp1_veth1
    Should Contain    ${out}    vpp1_veth1@vpp1_veth2
    Should Contain    ${out}    linux_vpp1_tap1

Check Interfaces On VPP1
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_tap1
    Should Contain    ${out}    ${int}

Check Linux Interfaces On VPP2
    ${out}=    Execute In Container    agent_vpp_2    ip a
    Should Contain    ${out}    vpp2_veth2@vpp2_veth1
    Should Contain    ${out}    vpp2_veth1@vpp2_veth2
    Should Contain    ${out}    linux_vpp2_tap1            

Check Interfaces On VPP2
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_tap1
    Should Contain    ${out}    ${int}

Check Bridge Domain On VPP1 Is Created
    vat_term: BD Is Created    agent_vpp_1    vpp1_vxlan1    vpp1_afpacket1

Check Bridge Domain On VPP2 Is Created
    vat_term: BD Is Created    agent_vpp_2    vpp2_vxlan1    vpp2_afpacket1

Show Interfaces And Other Objects After Config
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

Check Ping6 From VPP1 to VPP2
    linux: Check Ping6    agent_vpp_1    ${NET2_IP2}

Check Ping6 From VPP2 to VPP1
    linux: Check Ping6    agent_vpp_2    ${NET2_IP1}

Config Done
    No Operation


Remove VPP Nodes
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Check Linux Interfaces On VPP1 After Resync
    ${out}=    Execute In Container    agent_vpp_1    ip a
    Should Contain    ${out}    vpp1_veth2@vpp1_veth1
    Should Contain    ${out}    vpp1_veth1@vpp1_veth2
    Should Contain    ${out}    linux_vpp1_tap1

Check Interfaces On VPP1 After Resync
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_tap1
    Should Contain    ${out}    ${int}

Check Linux Interfaces On VPP2 After Resync
    ${out}=    Execute In Container    agent_vpp_2    ip a
    Should Contain    ${out}    vpp2_veth2@vpp2_veth1
    Should Contain    ${out}    vpp2_veth1@vpp2_veth2
    Should Contain    ${out}    linux_vpp2_tap1

Check Interfaces On VPP2 After Resync
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_tap1
    Should Contain    ${out}    ${int}

Check Bridge Domain On VPP1 Is Created After Resync
    vat_term: BD Is Created    agent_vpp_1    vpp1_vxlan1    vpp1_afpacket1

Check Bridge Domain On VPP2 Is Created After Resync
    vat_term: BD Is Created    agent_vpp_2    vpp2_vxlan1    vpp2_afpacket1

Show Interfaces And Other Objects After Resync
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
    Sleep    ${SYNC_SLEEP}
Check Ping6 From VPP1 to VPP2 After Resync
    linux: Check Ping6    agent_vpp_1    ${NET2_IP2}

Check Ping6 From VPP2 to VPP1 After Resync
    linux: Check Ping6    agent_vpp_2    ${NET2_IP1}

Remove VPP Nodes 2
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again 2
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Check Ping6 From VPP1 to VPP2 After Resync 2
    linux: Check Ping6    agent_vpp_1    ${NET2_IP2}

Check Ping6 From VPP2 to VPP1 After Resync 2
    linux: Check Ping6    agent_vpp_2    ${NET2_IP1}

Remove VPP Nodes 3
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again 3
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Check Ping6 From VPP1 to VPP2 After Resync 3
    linux: Check Ping6    agent_vpp_1    ${NET2_IP2}

Check Ping6 From VPP2 to VPP1 After Resync 3
    linux: Check Ping6    agent_vpp_2    ${NET2_IP1}


Resync Done
    No Operation

Final Sleep After Resync For Manual Checking
    Sleep   ${RESYNC_SLEEP}


*** Keywords ***
*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown