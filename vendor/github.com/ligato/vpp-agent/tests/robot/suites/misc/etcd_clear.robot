*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Force Tags        misc    ExpectedFailure
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

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1


Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Setup Interfaces
    Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns1    name=ns1_veth1    host_if_name=ns1_veth1_linux    mac=d2:74:8c:12:67:d2    peer=ns2_veth2    ip=192.168.22.1    prefix=30
    Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns2    name=ns2_veth2    host_if_name=ns2_veth2_linux    mac=92:c7:42:67:ab:cd    peer=ns1_veth1    ip=192.168.22.2    prefix=30

    Put Afpacket Interface    node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2
    @{ints}=    Create List    vpp1_vxlan1    vpp1_afpacket1
    Put Bridge Domain    node=agent_vpp_1    name=vpp1_bd1    ints=${ints}

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
    Write To Machine    vpp_agent_ctl    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -ps 
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_2    ip a

Check Ping From VPP1 to VPP2
    linux: Check Ping    agent_vpp_1    10.10.1.2

Check Ping From VPP2 to VPP1
    linux: Check Ping    agent_vpp_2    10.10.1.1

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
Check Ping From VPP1 to VPP2 After Resync
    linux: Check Ping    agent_vpp_1    10.10.1.2

Check Ping From VPP2 to VPP1 After Resync
    linux: Check Ping    agent_vpp_2    10.10.1.1

Remove VPP Nodes 2
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again 2
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Check Ping From VPP1 to VPP2 After Resync 2
    linux: Check Ping    agent_vpp_1    10.10.1.2

Check Ping From VPP2 to VPP1 After Resync 2
    linux: Check Ping    agent_vpp_2    10.10.1.1

Remove VPP Nodes 3
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}

Start VPP1 And VPP2 Again 3
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Sleep    ${RESYNC_WAIT}

Check Ping From VPP1 to VPP2 After Resync 3
    linux: Check Ping    agent_vpp_1    10.10.1.2

Check Ping From VPP2 to VPP1 After Resync 3
    linux: Check Ping    agent_vpp_2    10.10.1.1


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