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

${IP1}=                fd30::1:b:0:0:1
${IP2}=                fd30::1:b:0:0:2
${IP3}=                fd31::1:b:0:0:1
${IP4}=                fd31::1:b:0:0:2
${IP10}=               fd32::1:b:0:0:1
${IP20}=               fd32::1:b:0:0:2
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Add First VXLan Interface
    vxlan: Tunnel Not Exists    node=agent_vpp_1    src=${IP1}    dst=${IP2}    vni=15
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP1}    dst=${IP2}    vni=15

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Created    node=agent_vpp_1    src=${IP1}    dst=${IP2}    vni=15
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan1    enabled=1    src=${IP1}    dst=${IP2}    vni=15

Add Second VXLan Interface
    vxlan: Tunnel Not Exists    node=agent_vpp_1    src=${IP3}    dst=${IP4}    vni=25
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan2    src=${IP3}    dst=${IP4}    vni=25

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Created    node=agent_vpp_1    src=${IP3}    dst=${IP4}    vni=25
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan2    enabled=1    src=${IP3}    dst=${IP4}    vni=25

Check That First VXLan Interface Is Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan1    enabled=1    src=${IP1}    dst=${IP2}    vni=15

Update First VXLan Interface
    Put VXLan Interface    node=agent_vpp_1    name=vpp1_vxlan1    src=${IP10}    dst=${IP20}    vni=150

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Deleted    node=agent_vpp_1    src=${IP1}    dst=${IP2}    vni=15
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Created    node=agent_vpp_1    src=${IP10}    dst=${IP20}    vni=150
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan1    enabled=1    src=${IP10}    dst=${IP20}    vni=150

Check That Second VXLan Interface Is Not Changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan2    enabled=1    src=${IP3}    dst=${IP4}    vni=25

Delete First VXLan Interface
    Delete VPP Interface    node=agent_vpp_1    name=vpp1_vxlan1

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vxlan: Tunnel Is Deleted    node=agent_vpp_1    src=${IP10}    dst=${IP20}    vni=150

Check That Second VXLan Interface Is Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check VXLan Interface State    agent_vpp_1    vpp1_vxlan2    enabled=1    src=${IP3}    dst=${IP4}    vni=25

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

