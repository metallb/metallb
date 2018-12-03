*** Settings ***

Library     OperatingSystem
Library     String
#Library     RequestsLibrary

Resource     ../../variables/${VARIABLES}_variables.robot
Resource    ../../libraries/all_libs.robot
Resource    ../../libraries/pretty_keywords.robot

Force Tags        crud     IPv6
Suite Setup       Run Keywords    Discard old results

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${IP1}=                fd31::1:1:0:0:1
${IP2}=                fd31::1:1:0:0:2
${IPNET1}=             fd30:0:0:1::
${IPNET2}=             fd31:0:0:1::
${WAIT_TIMEOUT}=       20s
${SYNC_SLEEP}=         2s

*** Test Cases ***
# CRUD tests for routing
Add Route, Then Delete Route And Again Add Route For Default VRF
    [Setup]      Test Setup
    [Teardown]   Test Teardown

    Given Add Agent VPP Node                 agent_vpp_1
    Then IP6 Fib On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then Create Route On agent_vpp_1 With IP ${IPNET1}/64 With Next Hop ${IP1} And Vrf Id 0
    Then Show Interfaces On agent_vpp_1
    Then IP6 Fib On agent_vpp_1 Should Contain Route With IP ${IPNET1}/64
    Then Delete Routes On agent_vpp_1 And Vrf Id 0
    Then IP6 Fib On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then Create Route On agent_vpp_1 With IP ${IPNET1}/64 With Next Hop ${IP1} And Vrf Id 0

Add Route, Then Delete Route And Again Add Route For Non Default VRF
    [Setup]      Test Setup
    [Teardown]   Test Teardown

    Given Add Agent VPP Node                 agent_vpp_1
    Then IP6 Fib On agent_vpp_1 Should Not Contain Route With IP ${IPNET2}/64
    Then Create Route On agent_vpp_1 With IP ${IPNET1}/64 With Next Hop ${IP1} And Vrf Id 2
    Then Show Interfaces On agent_vpp_1
    Then IP6 Fib On agent_vpp_1 Should Contain Route With IP ${IPNET1}/64
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IPNET1}/64
    Then Delete Routes On agent_vpp_1 And Vrf Id 2
    Then IP6 Fib On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then IP6 Fib Table 2 On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then Create Route On agent_vpp_1 With IP ${IPNET1}/64 With Next Hop ${IP1} And Vrf Id 2
    Then IP6 Fib On agent_vpp_1 Should Contain Route With IP ${IPNET1}/64
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IPNET1}/64
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IPNET1}/64

# CRUD tests for VRF - automatically added with creating of interface - delete is not implemented
Add VRF Table In Background While Creating Interface Memif
    [Setup]      Test Setup
    [Teardown]   Test Teardown

    Given Add Agent VPP Node                 agent_vpp_1
    # create memif interface in default vrf
    Then Create Master memif0 on agent_vpp_1 with IP ${IP1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Then Show Interfaces On agent_vpp_1
    Then IP6 Fib Table 2 On agent_vpp_1 Should Be Empty
    Then IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    # this will transfer interface to newly-in-background-created non default vrf table
    Then Create Master memif0 on agent_vpp_1 with VRF 2, IP ${IP1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to other newly-in-background-created non default vrf table
    Then Create Master memif0 on agent_vpp_1 with VRF 1, IP ${IP1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Then IP6 Fib Table 1 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will remove non default vrf table in background - N/A
    # Then IP6 Fib Table 2 On agent_vpp_1 Should Be Empty - N/A
    Then IP6 Fib Table 2 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to existing non default vrf table
    Then Create Master memif0 on agent_vpp_1 with VRF 2, IP ${IP1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 1 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to default vrf table
    Then Create Master memif0 on agent_vpp_1 with IP ${IP1}, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    # 10 nov 2017 this will fail for memif - reason is that Create Master memif0 does not transfer interface to the VRF table 0
    Then IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 1 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # 10 nov 2017 this will fail for memif - reason is that Create Master memif0 does not transfer interface to the VRF table 0
    Then IP6 Fib Table 2 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128

Add VRF Table In Background While Creating Interface Tap
    [Setup]      Test Setup
    [Teardown]   Test Teardown

    Given Add Agent VPP Node                 agent_vpp_1
    # create Tap interface in default vrf
    Then Create Tap Interface tap0 On agent_vpp_1 With Vrf 0, IP ${IP1}, MAC 02:f1:be:90:00:00 And HostIfName linux_tap0
    Then Show Interfaces On agent_vpp_1
    Then IP6 Fib Table 2 On agent_vpp_1 Should Be Empty
    Then IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    # this will transfer interface to newly-in-background-created non default vrf table
    Then Create Tap Interface tap0 On agent_vpp_1 With Vrf 2, IP ${IP1}, MAC 02:f1:be:90:00:00 And HostIfName linux_tap0
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to other newly-in-background-created non default vrf table
    Then Create Tap Interface tap0 On agent_vpp_1 With Vrf 1, IP ${IP1}, MAC 02:f1:be:90:00:00 And HostIfName linux_tap0
    Then IP6 Fib Table 1 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will remove non default vrf table in background - N/A
    # Then IP6 Fib Table 2 On agent_vpp_1 Should Be Empty - N/A
    Then IP6 Fib Table 2 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to existing non default vrf table
    Then Create Tap Interface tap0 On agent_vpp_1 With Vrf 2, IP ${IP1}, MAC 02:f1:be:90:00:00 And HostIfName linux_tap0
    Then IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 0 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 1 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    # this will transfer interface to default vrf table
    Then Create Tap Interface tap0 On agent_vpp_1 With Vrf 0, IP ${IP1}, MAC 02:f1:be:90:00:00 And HostIfName linux_tap0
    Then IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 1 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128
    Then IP6 Fib Table 2 On agent_vpp_1 Should Not Contain Route With IP ${IP1}/128

Add VRF Table In Background While Creating Interface VXLAN
    [Setup]      Test Setup
    [Teardown]   Test Teardown

    Add Agent VPP Node                 agent_vpp_1
    Sleep    10
    # create VXLan interface in default vrf
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=5    vrf=0
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Show IP6 Fib On agent_vpp_1
    Show Interfaces Address On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP2}/128
    # this will transfer interface to newly-in-background-created non default vrf table
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=5    vrf=2
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Show IP6 Fib On agent_vpp_1
    Show Interfaces Address On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP2}/128
    # this will transfer interface to other newly-in-background-created non default vrf table
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=5    vrf=1
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Show IP6 Fib On agent_vpp_1
    Show Interfaces Address On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 1 On agent_vpp_1 Should Contain Route With IP ${IP2}/128
    # this will transfer interface to existing non default vrf table
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=5    vrf=2
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Show IP6 Fib On agent_vpp_1
    Show Interfaces Address On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 2 On agent_vpp_1 Should Contain Route With IP ${IP2}/128
    # this will transfer interface to default vrf table
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=5    vrf=0
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Show IP6 Fib On agent_vpp_1
    Show Interfaces Address On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    IP6 Fib Table 0 On agent_vpp_1 Should Contain Route With IP ${IP2}/128

*** Keywords ***
IP6 Fib On ${node} Should Not Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP6 Fib    ${node}
    Should Not Match Regexp    ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip6-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip6\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]

IP6 Fib On ${node} Should Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP6 Fib    ${node}
    Should Match Regexp        ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip6-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip6\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]
