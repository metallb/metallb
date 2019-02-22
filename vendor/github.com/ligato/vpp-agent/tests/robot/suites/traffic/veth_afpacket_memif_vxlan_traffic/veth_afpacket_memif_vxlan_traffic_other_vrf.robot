*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot
Resource     ../../../libraries/all_libs.robot
Resource    ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv4    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${CONFIG_SLEEP}=       1s
${RESYNC_SLEEP}=       1s
# wait for resync vpps after restart
${RESYNC_WAIT}=        30s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Setup Interfaces
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=62:61:61:61:61:61    master=true    id=1    ip=192.168.1.1    vrf=1
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=12:11:11:11:11:11    peer=vpp1_veth2    ip=10.10.1.1    vrf=1
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=12:12:12:12:12:12    peer=vpp1_veth1
    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2    vrf=1
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=192.168.1.1    dst=192.168.1.2    vni=5    vrf=1
    # # temporarily START to replace bad implemented vrf for vxlan
    # ${int3}=    Get Interface Internal Name    agent_vpp_1    vpp1_vxlan1
    # vpp_term: Issue Command    node=agent_vpp_1    command=create vxlan tunnel src 192.168.1.1 dst 192.168.1.2 vni 5 del
    # vpp_term: Issue Command    node=agent_vpp_1    command=create vxlan tunnel src 192.168.1.1 dst 192.168.1.2 vni 5 encap-vrf-id 1 decap-next l2
    # sleep    2
    # # temporarily END
    @{ints}=    Create List    vpp1_vxlan1    vpp1_afpacket1
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}

    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif1    mac=62:62:62:62:62:62    master=false    id=1    ip=192.168.1.2
    Put Veth Interface With IP    node=agent_vpp_2    name=vpp2_veth1    mac=22:21:21:21:21:21    peer=k tomu ti staci jedno vpp    ip=10.10.1.2
    Put Veth Interface    node=agent_vpp_2    name=vpp2_veth2    mac=22:22:22:22:22:22    peer=vpp2_veth1
    Put Afpacket Interface    node=agent_vpp_2    name=vpp2_afpacket1    mac=a2:a2:a2:a2:a2:a2    host_int=vpp2_veth2
    Put VXLan Interface    node=agent_vpp_2    name=vpp2_vxlan1    src=192.168.1.2    dst=192.168.1.1    vni=5
    @{ints}=    Create List    vpp2_vxlan1    vpp2_afpacket1
    Put Bridge Domain    node=agent_vpp_2    name=vpp2_bd1    ints=${ints}

    Sleep    10

Check Linux Interfaces On VPP1
    ${out}=    Execute In Container    agent_vpp_1    ip a
    Should Contain    ${out}    vpp1_veth2@vpp1_veth1
    Should Contain    ${out}    vpp1_veth1@vpp1_veth2

Check Interfaces On VPP1
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_vxlan1
    Should Contain    ${out}    ${int}

Check Linux Interfaces On VPP2
    ${out}=    Execute In Container    agent_vpp_2    ip a
    Should Contain    ${out}    vpp2_veth2@vpp2_veth1
    Should Contain    ${out}    vpp2_veth1@vpp2_veth2

Check Interfaces On VPP2
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_memif1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_vxlan1
    Should Contain    ${out}    ${int}

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

Check Ping From VPP1 to VPP2
    linux: Check Ping    agent_vpp_1    10.10.1.2
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_memif1
    Ping On agent_vpp_1 With IP 192.168.1.2, Source ${int}

Check Ping From VPP2 to VPP1
    linux: Check Ping    agent_vpp_2    10.10.1.1
    Ping From agent_vpp_2 To 192.168.1.1

Config Done
    No Operation

Final Sleep After Config For Manual Checking
    Sleep   ${CONFIG_SLEEP}

Remove VPP Nodes
    Remove All Nodes

Start VPP1 And VPP2 Again
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

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

Check Ping After Resync From VPP1 to VPP2
    linux: Check Ping    agent_vpp_1    10.10.1.2

Check Ping After Resync From VPP2 to VPP1
    linux: Check Ping    agent_vpp_2    10.10.1.1

Resync Done
    No Operation



*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown