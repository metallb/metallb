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
${VARIABLES}=          common
${ENV}=                common
${VETH1_MAC}=          1a:00:00:11:11:11
${VETH1_SEC_MAC}=      1a:00:00:11:11:12
${VETH2_MAC}=          2a:00:00:22:22:22
${VETH3_MAC}=          3a:00:00:33:33:33
${VETH4_MAC}=          4a:00:00:44:44:44
${VETHIP1}=            fd33::1:b:0:0:1
${VETHIP2}=            fd31::1:b:0:0:1
${VETHIP3}=            fd33::1:b:0:0:2
${WAIT_TIMEOUT}=       20s
${SYNC_SLEEP}=         2s

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Add Veth1 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=${VETH1_MAC}    peer=vpp1_veth2    ip=${VETHIP1}    prefix=64    mtu=1500
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH1_MAC}

Add Veth2 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH2_MAC}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth2    mac=${VETH2_MAC}    peer=vpp1_veth1

Check That Veth1 And Veth2 Interfaces Are Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH1_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH2_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_MAC}    ipv6=${VETHIP1}/64    mtu=1500    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up

Add Veth3 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH3_MAC}
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth3    mac=${VETH3_MAC}    peer=vpp1_veth4    ip=${VETHIP2}   prefix=64    mtu=1500
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH3_MAC}

Add Veth4 Interface
    linux: Interface Not Exists    node=agent_vpp_1    mac=${VETH4_MAC}
    Put Veth Interface    node=agent_vpp_1    name=vpp1_veth4    mac=${VETH4_MAC}    peer=vpp1_veth3    enabled=false

Check That Veth3 And Veth4 Interfaces Are Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH3_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH4_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth3    mac=${VETH3_MAC}    ipv6=${VETHIP2}/64    mtu=1500    state=lowerlayerdown
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth4    mac=${VETH4_MAC}    state=down

Check That Veth1 And Veth2 Interfaces Are Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_MAC}    ipv6=${VETHIP1}/64    mtu=1500    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up

Update Veth1 Interface
    Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1    mac=${VETH1_SEC_MAC}    peer=vpp1_veth2    ip=${VETHIP3}    prefix=64    mtu=1600
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Deleted    node=agent_vpp_1    mac=${VETH1_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Created    node=agent_vpp_1    mac=${VETH1_SEC_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth1    mac=${VETH1_SEC_MAC}    ipv6=${VETHIP3}/64    mtu=1600    state=up

Check That Veth2 And Veth3 And Veth4 interfaces Are Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth2    mac=${VETH2_MAC}    state=up
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth3    mac=${VETH3_MAC}    ipv6=${VETHIP2}/64    mtu=1500    state=lowerlayerdown
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth4    mac=${VETH4_MAC}    state=down

Delete Veth2 Interface
    Delete Linux Interface    node=agent_vpp_1    name=vpp1_veth2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Deleted    node=agent_vpp_1    mac=${VETH1_SEC_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Deleted    node=agent_vpp_1    mac=${VETH2_MAC}

Check That Veth3 And Veth4 Are Still Configured
    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth3    mac=${VETH3_MAC}    ipv6=${VETHIP2}/64    mtu=1500    state=lowerlayerdown
    linux: Check Veth Interface State     agent_vpp_1    vpp1_veth4    mac=${VETH4_MAC}    state=down

Delete Veth3 Interface
    Delete Linux Interface    node=agent_vpp_1    name=vpp1_veth3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Deleted    node=agent_vpp_1    mac=${VETH3_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    linux: Interface Is Deleted    node=agent_vpp_1    mac=${VETH4_MAC}

Show Interfaces And Other Objects After Setup
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

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

