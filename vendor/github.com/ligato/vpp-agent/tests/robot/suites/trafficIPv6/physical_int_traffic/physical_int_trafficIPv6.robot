*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Force Tags        traffic     IPv6    ExpectedFailure
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${CONFIG_SLEEP}=       1s
${RESYNC_SLEEP}=       1s
# wait for resync vpps after restart
${RESYNC_WAIT}=        30s
${IP_1}=         fd30::1:a:0:0:1
${IP_2}=         fd30::1:a:0:0:2
${IP_3}=         fd31::1:a:0:0:1
${IP_4}=         fd31::1:a:0:0:2
${IP_5}=         fd32::1:a:0:0:1
${IP_6}=         fd33::1:a:0:0:1

*** Test Cases ***
Configure Environment
    [Tags]    setup
    ${phys_ints}=    Create List    1
    Add Agent VPP Node With Physical Int    agent_vpp_1    ${phys_ints}
    ${phys_ints}=    Create List    2
    Add Agent VPP Node With Physical Int    agent_vpp_2    ${phys_ints}

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Setup Interfaces
    Put Physical Interface With IP    node=agent_vpp_1    name=GigabitEthernet0/9/0    ip=${IP_1}
#    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=62:61:61:61:61:61    master=true    id=1    ip=${IP_1}
##    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=12:11:11:11:11:11    peer=vpp1_veth2    ip=${IP_3}
##    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=12:12:12:12:12:12    peer=vpp1_veth1
##    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2
##    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP_1}    dst=${IP_2}    vni=5
##    @{ints}=    Create List    vpp1_vxlan1    vpp1_afpacket1
##    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}
##    Put Loopback Interface With IP    node=agent_vpp_1    name=vpp1_loop1    mac=12:21:21:11:11:11    ip=20.20.1.1
##    Put TAP Interface With IP    node=agent_vpp_1    name=vpp1_tap1    mac=32:21:21:11:11:11    ip=30.30.1.1    host_if_name=linux_vpp1_tap1

    Put Physical Interface With IP    node=agent_vpp_2    name=GigabitEthernet0/a/0    ip=${IP_2}
#    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif1    mac=62:62:62:62:62:62    master=false    id=1    ip=${IP_2}
    Put Veth Interface With IP    node=agent_vpp_2    name=vpp2_veth1    mac=22:21:21:21:21:21    peer=vpp2_veth2    ip=${IP_4}
    Put Veth Interface    node=agent_vpp_2    name=vpp2_veth2    mac=22:22:22:22:22:22    peer=vpp2_veth1
    Put Afpacket Interface    node=agent_vpp_2    name=vpp2_afpacket1    mac=a2:a2:a2:a2:a2:a2    host_int=vpp2_veth2
    Put VXLan Interface    node=agent_vpp_2    name=vpp2_vxlan1    src=${IP_2}    dst=${IP_1}    vni=5
    @{ints}=    Create List    vpp2_vxlan1    vpp2_afpacket1
    Put Bridge Domain    node=agent_vpp_2    name=vpp2_bd1    ints=${ints}
    Put Loopback Interface With IP    node=agent_vpp_2    name=vpp2_loop1    mac=22:21:21:11:11:11    ip=${IP_5}
    Put TAP Interface With IP    node=agent_vpp_2    name=vpp2_tap1    mac=32:22:22:11:11:11    ip=${IP_6}    host_if_name=linux_vpp2_tap1
 
Check Linux Interfaces On VPP1
    ${out}=    Execute In Container    agent_vpp_1    ip a
    Should Contain    ${out}    vpp1_veth2@vpp1_veth1
    Should Contain    ${out}    vpp1_veth1@vpp1_veth2
    Should Contain    ${out}    linux_vpp1_tap1

Check Interfaces On VPP1
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
#    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_memif1
#    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    vpp1_tap1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_1    GigabitEthernet0/9/0
    Should Contain    ${out}    ${int}

Check Linux Interfaces On VPP2
    ${out}=    Execute In Container    agent_vpp_2    ip a
    Should Contain    ${out}    vpp2_veth2@vpp2_veth1
    Should Contain    ${out}    vpp2_veth1@vpp2_veth2
    Should Contain    ${out}    linux_vpp2_tap1            

Check Interfaces On VPP2
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
#   ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_memif1
#    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_afpacket1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_vxlan1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_loop1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    vpp2_tap1
    Should Contain    ${out}    ${int}
    ${int}=    Get Interface Internal Name    agent_vpp_2    GigabitEthernet0/a/0
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
    Write To Machine    vpp_agent_ctl    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -ps 
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_2    ip a

Check Ping From VPP1 to VPP2
    linux: Check Ping    agent_vpp_1    ${IP_4}

Check Ping From VPP2 to VPP1
    linux: Check Ping    agent_vpp_2    ${IP_3}

fdsasdf
#    Delete VPP Interface    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
#    Delete VPP Interface    node=agent_vpp_2    name=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    sleep    5
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_2_term    show int addr


Config Done
    No Operation

Final Sleep After Config For Manual Checking
    Sleep   ${CONFIG_SLEEP}

Remove VPP Nodes
    Remove All Nodes

Start VPP1 And VPP2 Again
    ${phys_ints}=    Create List    1
    Add Agent VPP Node With Physical Int    agent_vpp_1    ${phys_ints}
    ${phys_ints}=    Create List    2
    Add Agent VPP Node With Physical Int    agent_vpp_2    ${phys_ints}
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
    linux: Check Ping    agent_vpp_1    ${IP_4}

Check Ping After Resync From VPP2 to VPP1
    linux: Check Ping    agent_vpp_2    ${IP_3}

asdf
    Delete VPP Interface    node=agent_vpp_1    name=${DOCKER_PHYSICAL_INT_1_VPP_NAME}
    Delete VPP Interface    node=agent_vpp_2    name=${DOCKER_PHYSICAL_INT_2_VPP_NAME}
    sleep    5
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_2_term    show int addr


Resync Done
    No Operation

Final Sleep After Resync For Manual Checking
    Sleep   ${RESYNC_SLEEP}


*** Keywords ***
