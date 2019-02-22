*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot

Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${CONFIG_SLEEP}=       20s
${MIN_TX_INTERVAL}=    100000
${VPP1_MAC_ADDR}=      62:61:61:61:61:61
${VPP1_IP4_ADDR}=      192.168.1.1
${DETECT_MULTIPLIER}=  3
${VPP1_BFD_INTF}=      vpp1_memif1
${VPP2_BFD_INTF}=      vpp2_memif1
${MIN_RX_INTERVAL}=    100000
${VPP2_MAC_ADDR}=      62:62:62:62:62:62
${VPP2_IP4_ADDR}=      192.168.1.2
${AUTH_KEY_NAME}=      BFD_KEY_1
${KEYED_SHA1}=         0
${METICULOUS_KEYED_SHA1}=    1
${SECRET}=             21221334567
*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2

Setup Interfaces
    Put Memif Interface With IP    node=agent_vpp_1    name=${VPP1_BFD_INTF}    mac=${VPP1_MAC_ADDR}    master=true    id=1    ip=${VPP1_IP4_ADDR}
    Put Memif Interface With IP    node=agent_vpp_2    name=${VPP2_BFD_INTF}    mac=${VPP2_MAC_ADDR}    master=false   id=1    ip=${VPP2_IP4_ADDR}

Check Interfaces On VPP1
    ${out}=    vpp_term: Show Interfaces    agent_vpp_1
    ${int}=    Get Interface Internal Name    agent_vpp_1    ${VPP1_BFD_INTF}
    Should Contain    ${out}    ${int}

Check Interfaces On VPP2
    ${out}=    vpp_term: Show Interfaces    agent_vpp_2
    ${int}=    Get Interface Internal Name    agent_vpp_2    ${VPP2_BFD_INTF}
    Should Contain    ${out}    ${int}

#Show Interfaces And Other Objects After Config
#    vpp_term: Show Interfaces    agent_vpp_1
#    vpp_term: Show Interfaces    agent_vpp_2
#    Write To Machine    agent_vpp_1_term    show int addr
#    Write To Machine    agent_vpp_2_term    show int addr
#    Write To Machine    agent_vpp_1_term    show h
#    Write To Machine    agent_vpp_2_term    show h
#    Write To Machine    agent_vpp_1_term    show br
#    Write To Machine    agent_vpp_2_term    show br
#    Write To Machine    agent_vpp_1_term    show br 1 detail
#    Write To Machine    agent_vpp_2_term    show br 1 detail
#    Write To Machine    agent_vpp_1_term    show vxlan tunnel
#    Write To Machine    agent_vpp_2_term    show vxlan tunnel
#    Write To Machine    agent_vpp_1_term    show err
#    Write To Machine    agent_vpp_2_term    show err
#    vat_term: Interfaces Dump    agent_vpp_1
#    vat_term: Interfaces Dump    agent_vpp_2
#    Write To Machine    vpp_agent_ctl    vpp-agent-ctl   ${AGENT_VPP_ETCD_CONF_PATH} -ps
#    Execute In Container    agent_vpp_1    ip a
#    Execute In Container    agent_vpp_2    ip a

Setup BFD Authentication Key On VPP1
    Put BFD Authentication Key    agent_vpp_1    ${AUTH_KEY_NAME}    ${METICULOUS_KEYED_SHA1}    1    ${SECRET}

Check BFD Authentication Key On VPP1
    Get BFD Authentication Key As Json    agent_vpp_1    ${AUTH_KEY_NAME}

Setup BFD Authentication Key On VPP2
    Put BFD Authentication Key    agent_vpp_2    ${AUTH_KEY_NAME}  ${METICULOUS_KEYED_SHA1}    1    ${SECRET}

Check BFD Authentication Key On VPP2
    Get BFD Authentication Key As Json    agent_vpp_2    ${AUTH_KEY_NAME}

Setup BFD VPP1 Session Without Authentication
    Put BFD Session    agent_vpp_1    VPP1_BFD_Session1    ${MIN_TX_INTERVAL}    ${VPP2_IP4_ADDR}    ${DETECT_MULTIPLIER}   ${VPP1_BFD_INTF}    ${MIN_RX_INTERVAL}    ${VPP1_IP4_ADDR}    true

Setup BFD VPP2 Session Without Authentication
    Put BFD Session    agent_vpp_2    VPP2_BFD_Session1    ${MIN_TX_INTERVAL}    ${VPP1_IP4_ADDR}    ${DETECT_MULTIPLIER}    ${VPP2_BFD_INTF}    ${MIN_RX_INTERVAL}    ${VPP2_IP4_ADDR}    true

Sleep After Config For Manual Checking
    Sleep   ${CONFIG_SLEEP}

Check BFD VPP1 Session Without Authentication
    Get BFD Session As Json    agent_vpp_1    VPP1_BFD_Session1

Check BFD VPP2 Session Without Authentication
    Get BFD Session As Json    agent_vpp_2    VPP2_BFD_Session1

Update BFD VPP1 Session With Authentication
    Put BFD Session    agent_vpp_1    VPP1_BFD_Session1    ${MIN_TX_INTERVAL}    ${VPP2_IP4_ADDR}    ${DETECT_MULTIPLIER}   ${VPP1_BFD_INTF}    ${MIN_RX_INTERVAL}    ${VPP1_IP4_ADDR}    true    1    35454545

Update BFD VPP2 Session With Authentication
    Put BFD Session    agent_vpp_2    VPP2_BFD_Session1    ${MIN_TX_INTERVAL}    ${VPP1_IP4_ADDR}    ${DETECT_MULTIPLIER}    ${VPP2_BFD_INTF}    ${MIN_RX_INTERVAL}    ${VPP2_IP4_ADDR}    true    1    35454546

Sleep After Config For Manual Checking
    Sleep   ${CONFIG_SLEEP}

Check BFD VPP1 Session With Authentication
    Get BFD Session As Json    agent_vpp_1    VPP1_BFD_Session1

Check BFD VPP2 Session With Authentication
    Get BFD Session As Json    agent_vpp_2    VPP2_BFD_Session1

Setup BFD Echo Function On VPP1
    Put BFD Echo Function    agent_vpp_1    BFD_ECHO_FUNCTION1    ${VPP1_BFD_INTF}

Check BFD Echo Function On VPP1
    Get BFD Echo Function As Json    agent_vpp_1

Remove VPP Nodes
    Remove All Nodes

#Start VPP1 And VPP2 Again
#    Add Agent VPP Node    agent_vpp_1
#    Add Agent VPP Node    agent_vpp_2
#    Sleep    ${RESYNC_WAIT}



*** Keywords ***
