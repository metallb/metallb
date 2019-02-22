*** Settings ***

Library     OperatingSystem
Library     String
#Library     RequestsLibrary

Resource     ../../variables/${VARIABLES}_variables.robot
Resource    ../../libraries/all_libs.robot
Resource    ../../libraries/pretty_keywords.robot

Force Tags        trafficIPv4
Suite Setup       Run Keywords    Discard old results     Test Setup
Suite Teardown    Test Teardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=       20s
${SYNC_SLEEP}=         2s
${FINAL_SLEEP}=        1s
${IP_1}=               fd30::1:b:0:0:1
${IP_2}=               fd30::1:b:0:0:2
${IP_3}=               fd31::1:b:0:0:1
${IP_4}=               fd31::1:b:0:0:2
${IP_5}=               fd32::1:b:0:0:1
${IP_6}=               fd32::1:b:0:0:2
${NET1}=               fd30:0:0:1::
${NET2}=               fd31:0:0:1::
${NET3}=               fd32:0:0:1::

*** Test Cases ***
# Non default VRF table 2 used in Agent VPP Node agent_vpp_2
Start Two Agents And Then Configure With Default And Non Default VRF
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2

    Create Master memif0 on agent_vpp_1 with IP ${IP_1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create Slave memif0 on agent_vpp_2 with IP ${IP_2}, MAC 02:f1:be:90:00:02, key 1 and m0.sock socket

    Create Master memif1 on agent_vpp_1 with VRF 2, IP ${IP_3}, MAC 02:f1:be:90:02:00, key 1 and m1.sock socket
    Create Slave memif1 on agent_vpp_2 with VRF 2, IP ${IP_4}, MAC 02:f1:be:90:02:02, key 1 and m1.sock socket

    Create Master memif2 on agent_vpp_1 with VRF 1, IP ${IP_5}, MAC 02:f1:be:90:04:00, key 1 and m2.sock socket
    Create Slave memif2 on agent_vpp_2 with VRF 1, IP ${IP_6}, MAC 02:f1:be:90:04:02, key 1 and m2.sock socket

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_1 Should Contain Interface memif1/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_2 Should Contain Interface memif1/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_1 Should Contain Interface memif2/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_2 Should Contain Interface memif2/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_1 Should Contain Interface memif3/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    List of interfaces On agent_vpp_2 Should Contain Interface memif3/1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP_1}/128
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP_3}/128
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${IP_2}/128
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${IP_4}/128
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 1 On agent_vpp_1 Should Contain Route With IP ${IP_5}/128
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${IP_6}/128

Create Route For Inter Vrf Routing
    Create Route On agent_vpp_1 With IP ${NET2}/64 With Next Hop ${IP_2} And Vrf Id 0
    Create Route On agent_vpp_1 With IP ${NET1}/64 With Next Hop ${IP_4} And Vrf Id 2
    Create Route On agent_vpp_2 With IP ${NET2}/64 With Next Hop VRF 2 From Vrf Id 0 And Type 1
    Create Route On agent_vpp_2 With IP ${NET1}/64 With Next Hop VRF 0 From Vrf Id 2 And Type 1

Config Done
    No Operation

Check Inter VRF Routing
    Show IP Fib On agent_vpp_1
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Vrf ipv6 via ${IP_2} memif1/1
    Show IP Fib On agent_vpp_2
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:

Create Next Route For Inter Vrf Routing
    Create Route On agent_vpp_2 With IP ${NET3}/64 With Next Hop VRF 1 From Vrf Id 0 And Type 1
    Create Route On agent_vpp_2 With IP ${NET3}/64 With Next Hop VRF 1 From Vrf Id 2 And Type 1
    Create Route On agent_vpp_2 With IP ${NET1}/64 With Next Hop VRF 0 From Vrf Id 1 And Type 1
    Create Route On agent_vpp_2 With IP ${NET2}/64 With Next Hop VRF 2 From Vrf Id 1 And Type 1

Check Inter VRF Routing Again
    Show IP Fib On agent_vpp_1
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Vrf ipv6 via ${IP_2} memif1/1
    Show IP Fib On agent_vpp_2
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:

    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET3}/64
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${NET3}/64
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:

    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    IP6 Fib Table 1 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 1 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:

Delete Route VRF 1
    Delete Route    agent_vpp_2    1    ${NET1}    64
    Delete Route    agent_vpp_2    1    ${NET2}    64
    Delete Route    agent_vpp_2    0    ${NET3}    64
    Delete Route    agent_vpp_2    2    ${NET3}    64

Check State After Delete
    Show IP Fib On agent_vpp_1
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_1 Should Contain Vrf ipv6 via ${IP_2} memif1/1
    Show IP Fib On agent_vpp_2
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    IP6 Fib Table 2 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    IP6 Fib Table 0 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:

    ${status}=        Run Keyword And Return Status    IP6 Fib Table 2 On agent_vpp_2 Should Contain Route With IP ${NET3}/64
    Should Not Be True    ${status}

    ${status}=        Run Keyword And Return Status    IP6 Fib Table 0 On agent_vpp_2 Should Contain Route With IP ${NET3}/64
    Should Not Be True    ${status}

    ${status}=        Run Keyword And Return Status    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    Should Not Be True    ${status}

    ${status}=        Run Keyword And Return Status    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${NET2}/64
    Should Not Be True    ${status}

    ${status}=        Run Keyword And Return Status    IP6 Fib Table 1 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:
    Should Not Be True    ${status}

Update Inter Vrf Route
    Create Route On agent_vpp_2 With IP ${NET1}/64 With Next Hop VRF 0 From Vrf Id 1 And Type 1
    Create Route On agent_vpp_2 With IP ${NET1}/64 With Next Hop VRF 2 From Vrf Id 1 And Type 1

Check Route After Update
    Show IP Fib On agent_vpp_2
    IP6 Fib Table 1 On agent_vpp_2 Should Contain Route With IP ${NET1}/64
    IP6 Fib Table 1 On agent_vpp_2 Should Contain Vrf unicast lookup in ipv6-VRF:2

#can use in debug
#Check Route With Ping
#    Ping On agent_vpp_1 With IP ${IP_4}, Source memif1/1
#    Ping On agent_vpp_1 With IP ${IP_4}, Source memif2/1
#    Ping On agent_vpp_1 With IP ${IP_3}, Source memif1/1
#

Final Sleep For Manual Checking
    Sleep   ${FINAL_SLEEP}

*** Keywords ***
List of interfaces On ${node} Should Contain Interface ${int}
    ${out}=   vpp_term: Show Interfaces    ${node}
    Should Match Regexp        ${out}  ${int}

IP Fib Table ${table_id} On ${node} Should Contain Vrf ${inter_vrf_string}
    ${out}=    vpp_term: Show IP Fib Table    ${node}    ${table_id}
    Should Contain  ${out}  ${inter_vrf_string}

IP6 Fib Table ${table_id} On ${node} Should Contain Vrf ${inter_vrf_string}
    ${out}=    vpp_term: Show IP6 Fib Table    ${node}    ${table_id}
    Should Contain  ${out}  ${inter_vrf_string}
