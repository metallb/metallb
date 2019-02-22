*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=       common
${ENV}=             common
${NAME_LOOP1}=      vpp1_loop1
${NAME_LOOP2}=      vpp1_loop2
${MAC_LOOP1}=       12:21:21:11:11:11
${MAC_LOOP1_2}=     22:21:21:11:11:11
${MAC_LOOP2}=       32:21:21:11:11:11
${IP_LOOP1}=        fd30::1:e:0:0:1
${IP_LOOP1_2}=      fd30::1:e:0:0:2
${IP_LOOP2}=        fd31::1:e:0:0:1
${PREFIX}=          64
${MTU}=             4800
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Add Loopback1 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_LOOP1}
    Put Loopback Interface With IP    node=agent_vpp_1    name=${NAME_LOOP1}    mac=${MAC_LOOP1}    ip=${IP_LOOP1}    prefix=${PREFIX}    mtu=${MTU}    enabled=true

Check Loopback1 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_LOOP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP1}    enabled=1     mac=${MAC_LOOP1}    mtu=${MTU}  ipv6=${IP_LOOP1}/${PREFIX}

Add Loopback2 Interface
    vpp_term: Interface Not Exists  node=agent_vpp_1    mac=${MAC_LOOP2}
    Put Loopback Interface With IP    node=agent_vpp_1     name=${NAME_LOOP2}    mac=${MAC_LOOP2}    ip=${IP_LOOP2}    prefix=${PREFIX}    mtu=${MTU}    enabled=true

Check Loopback2 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_LOOP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP2}    enabled=1     mac=${MAC_LOOP2}    mtu=${MTU}    ipv6=${IP_LOOP2}/${PREFIX}

Check Loopback1 Is Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP1}    enabled=1     mac=${MAC_LOOP1}    mtu=${MTU}         ipv6=${IP_LOOP1}/${PREFIX}

Update Loopback1
    Put Loopback Interface With IP    node=agent_vpp_1     name=${NAME_LOOP1}    mac=${MAC_LOOP1_2}    ip=${IP_LOOP1_2}    prefix=${PREFIX}    mtu=${MTU}    enabled=true
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_1    mac=${MAC_LOOP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MAC_LOOP1_2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP1}    enabled=1     mac=${MAC_LOOP1_2}    mtu=${MTU}    ipv6=${IP_LOOP1_2}/${PREFIX}

Check Loopback2 Is Not Changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP2}    enabled=1     mac=${MAC_LOOP2}    mtu=${MTU}         ipv6=${IP_LOOP2}/${PREFIX}

Delete Loopback1_2 Interface
    Delete VPP Interface    node=agent_vpp_1    name=${NAME_LOOP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_1    mac=${MAC_LOOP1_2}

Check Loopback2 Interface Is Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Loopback Interface State    agent_vpp_1    ${NAME_LOOP2}    enabled=1     mac=${MAC_LOOP2}    mtu=${MTU}         ipv6=${IP_LOOP2}/${PREFIX}

Show Interfaces And Other Objects After Setup
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_1_term    show br
    Write To Machine    agent_vpp_1_term    show br 1 detail
    Write To Machine    agent_vpp_1_term    show vxlan tunnel
    Write To Machine    agent_vpp_1_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    Execute In Container    agent_vpp_1    ip a

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

