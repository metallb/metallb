*** Settings ***
Library      OperatingSystem

Resource     ../../variables/${VARIABLES}_variables.robot
Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown

*** Variables ***
${NS1_ID}=                 ns1
${NS2_ID}=                 ns2
${NS3_ID}=                 ns3
${NS4_ID}=                 ns4
${NS5_ID}=                 ns5
${NS6_ID}=                 ns6
${NS7_ID}=                 ns7
${NS8_ID}=                 ns8
${NS9_ID}=                 ns9
${SECRET1}=                111
${SECRET2}=                222
${SECRET3}=                333
${SECRET4}=                444
${TAP1_NAME}=              tap1
${TAP1_MAC}=               12:21:21:11:11:11
${TAP1_IP}=                fd30:0:0:1:1::
${TAP1_SW_IF_INDEX}=       1
${TAP2_NAME}=              tap2
${TAP2_MAC}=               22:21:21:11:11:11
${TAP2_IP}=                fd30:0:0:1:2::
${TAP2_SW_IF_INDEX}=       2
${MEMIF1_NAME}=            memif1
${MEMIF1_IP}=              192.168.1.1
${MEMIF1_MAC}=             33:21:21:11:11:11
${MEMIF1_SW_IF_INDEX}=     3
${LOOP1_NAME}=             loop1
${LOOP1_MAC}=              44:21:21:11:11:11
${LOOP1_IP}=               fd30:0:0:1:3::
${LOOP1_SW_IF_INDEX}=      4
${VXLAN1_NAME}=            vxlan1
${VXLAN1_SRC}=             fd30:0:0:1:e::1
${VXLAN1_DST}=             fd30:0:0:1:e::2
${VXLAN1_VNI}=             15
${VXLAN1_SW_IF_INDEX}=     5
${PREFIX}=          64
${MTU}=             1500
${VARIABLES}=        common
${ENV}=              common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
# wait for resync vpps after restart
${RESYNC_WAIT}=        50s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Check L4 Features Are Disabled
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Contain    ${out}    show app: session layer is not enabled

Enable L4 Features
    vpp_ctl: Set L4 Features On Node    node=agent_vpp_1    enabled=true
    Sleep    1s

Check Default Namespace Was Added
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Contain    ${out}    default

Put Interface TAP1 And Namespace NS1 Associated With TAP1 And Check The Namespace Is Present In Namespaces List
    vpp_ctl: Put TAP Interface With IP    node=agent_vpp_1    name=${TAP1_NAME}    mac=${TAP1_MAC}    ip=${TAP1_IP}    prefix=${PREFIX}    host_if_name=linux_${TAP1_NAME}
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS1_ID}    secret=${SECRET1}    interface=${TAP1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    ${out_lines1}=    Get Line Count    ${out}
    Set Suite Variable    ${out_lines1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET1}    ${TAP1_SW_IF_INDEX}

Put Already Existing Namespace NS1 And Check Namespace Was Not Added To Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS1_ID}    secret=${SECRET1}    interface=${TAP1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    ${out_lines2}=    Get Line Count    ${out}
    Should Be Equal    ${out_lines1}    ${out_lines2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET1}    ${TAP1_SW_IF_INDEX}

Update Namespace NS1 Secret And Check The Namespace's Update Is Reflected In Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS1_ID}    secret=${SECRET2}    interface=${TAP1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}

Put New NS2 Namespace And Check The Namespace Is Present In Namespaces List And Namespace NS1 Is Still Configured
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS2_ID}    secret=${SECRET3}    interface=${TAP1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET3}    ${TAP1_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}

Put Interface TAP2 And Namespace NS3 Associated With TAP2 And Check The Namespace Is Present In Namespaces List
    vpp_ctl: Put TAP Interface With IP    node=agent_vpp_1    name=${TAP2_NAME}    mac=${TAP2_MAC}    ip=${TAP2_IP}    prefix=${PREFIX}    host_if_name=linux_${TAP2_NAME}
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS3_ID}    secret=${SECRET4}    interface=${TAP2_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS3_ID}    3    ${SECRET4}    ${TAP2_SW_IF_INDEX}

Check NS1 And NS2 Namespaces Remained Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET3}    ${TAP1_SW_IF_INDEX}

Update Namespace NS2 Associated Interface To TAP2 And Secret And Check The Namespace's Update Is Reflected In Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS2_ID}    secret=${SECRET1}    interface=${TAP2_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET1}    ${TAP2_SW_IF_INDEX}

Check NS1 And NS3 Namespaces Are Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS3_ID}    3    ${SECRET4}    ${TAP2_SW_IF_INDEX}

Do RESYNC 1
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}
    Add Agent VPP Node    agent_vpp_1
    Sleep    ${RESYNC_WAIT}

Get Interfaces Sw If Index After Resync 1
    ${TAP1_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${TAP1_NAME}
    ${TAP1_SW_IF_INDEX}=    Convert To String    ${TAP1_SW_IF_INDEX}
    Set Suite Variable    ${TAP1_SW_IF_INDEX}
    ${TAP2_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${TAP2_NAME}
    ${TAP2_SW_IF_INDEX}=    Convert To String    ${TAP2_SW_IF_INDEX}
    Set Suite Variable    ${TAP2_SW_IF_INDEX}

Check NS1, NS2 And NS3 Were Automatically Configured After Resync
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET1}    ${TAP2_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS3_ID}    3    ${SECRET4}    ${TAP2_SW_IF_INDEX}

Disable L4 Features And Check They Are Disabled
    vpp_ctl: Set L4 Features On Node    node=agent_vpp_1    enabled=false
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Contain    ${out}    show app: session layer is not enabled

Put Namespace NS4 While L4 Features Are Disabled
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS4_ID}    secret=${SECRET4}    interface=${TAP1_NAME}

Enable L4 Features And Check Namespaces NS1, NS2, NS3 And NS4 Are Present In Namespaces List
    vpp_ctl: Set L4 Features On Node    node=agent_vpp_1    enabled=true
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET1}    ${TAP2_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS3_ID}    3    ${SECRET4}    ${TAP2_SW_IF_INDEX}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS4_ID}    4    ${SECRET4}    ${TAP1_SW_IF_INDEX}

Put Namespace NS5 Associated With MEMIF1 Interface That Is Not Created And Check NS5 Is Not Present In Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS5_ID}    secret=${SECRET1}    interface=${MEMIF1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Not Contain    ${out}    ${NS5_ID}

Put MEMIF1 Interface And Check Namespace NS5 Is Present In Namespaces List
    vpp_ctl: Put Memif Interface With IP    node=agent_vpp_1    name=memif1    mac=${MEMIF1_MAC}    master=true    id=1    ip=fd30:0:0:1:e::2    prefix=${PREFIX}    socket=default.sock
    Sleep    1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS5_ID}    5    ${SECRET1}    ${MEMIF1_SW_IF_INDEX}

Put Namespace NS6 Associated With LOOP1 Interface That Is Not Created And Check NS6 Is Not Present In Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS6_ID}    secret=${SECRET1}    interface=${LOOP1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Not Contain    ${out}    ${NS6_ID}

Put LOOP1 Interface And Check Namespace NS6 Is Present In Namespaces List
    vpp_ctl: Put Loopback Interface With IP    node=agent_vpp_1    name=${LOOP1_NAME}    mac=${LOOP1_MAC}    ip=${LOOP1_IP}    prefix=${PREFIX}    mtu=${MTU}    enabled=true
    Sleep    1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS6_ID}    6    ${SECRET1}    ${LOOP1_SW_IF_INDEX}

Put Namespace NS7 Associated With VXLAN1 Interface That Is Not Created And Check NS7 Is Not Present In Namespaces List
    vpp_ctl: Put Application Namespace    node=agent_vpp_1    id=${NS7_ID}    secret=${SECRET1}    interface=${VXLAN1_NAME}
    ${out}=    vpp_term: Show Application Namespaces    node=agent_vpp_1
    Should Not Contain    ${out}    ${NS7_ID}

Put VXLAN1 Interface And Check Namespace NS7 Is Present In Namespaces List
    vpp_ctl: Put VXLan Interface    node=agent_vpp_1    name=${VXLAN1_NAME}    src=${VXLAN1_SRC}    dst=${VXLAN1_DST}    vni=${VXLAN1_VNI}
    Sleep    1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS7_ID}    7    ${SECRET1}    ${VXLAN1_SW_IF_INDEX}

Do RESYNC 2
    Remove All Nodes
    Sleep    ${SYNC_SLEEP}
    Add Agent VPP Node    agent_vpp_1
    Sleep    ${RESYNC_WAIT}

Get Interfaces Sw If Index After Resync 2
    ${TAP1_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${TAP1_NAME}
    ${TAP1_SW_IF_INDEX}=    Convert To String    ${TAP1_SW_IF_INDEX}
    Set Suite Variable    ${TAP1_SW_IF_INDEX}
    ${TAP2_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${TAP2_NAME}
    ${TAP2_SW_IF_INDEX}=    Convert To String    ${TAP2_SW_IF_INDEX}
    Set Suite Variable    ${TAP2_SW_IF_INDEX}
    ${MEMIF1_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${MEMIF1_NAME}
    ${MEMIF1_SW_IF_INDEX}=    Convert To String    ${MEMIF1_SW_IF_INDEX}
    Set Suite Variable    ${MEMIF1_SW_IF_INDEX}
    ${LOOP1_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${LOOP1_NAME}
    ${LOOP1_SW_IF_INDEX}=    Convert To String    ${LOOP1_SW_IF_INDEX}
    Set Suite Variable    ${LOOP1_SW_IF_INDEX}
    ${VXLAN1_SW_IF_INDEX}=    vpp_ctl: Get Interface Sw If Index    agent_vpp_1    ${VXLAN1_NAME}
    ${VXLAN1_SW_IF_INDEX}=    Convert To String    ${VXLAN1_SW_IF_INDEX}
    Set Suite Variable    ${VXLAN1_SW_IF_INDEX}

Check Namespaces NS1, NS2, NS3, NS4, NS5, NS6 AND NS7 Are Present In Namespaces List
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS1_ID}    1    ${SECRET2}    ${TAP1_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS2_ID}    2    ${SECRET1}    ${TAP2_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS3_ID}    3    ${SECRET4}    ${TAP2_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS4_ID}    4    ${SECRET4}    ${TAP1_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS5_ID}    5    ${SECRET1}    ${MEMIF1_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS6_ID}    6    ${SECRET1}    ${LOOP1_SW_IF_INDEX}
    vpp_term: Check Data In Show Application Namespaces Output    agent_vpp_1    ${NS7_ID}    7    ${SECRET1}    ${VXLAN1_SW_IF_INDEX}

*** Keywords ***
