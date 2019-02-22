*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot
Resource     ../../libraries/pretty_keywords.robot

Force Tags        crud     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=         20s
${SYNC_SLEEP}=           3s
${IP_1}=                 10.0.1.1
${IP_2}=                 10.0.1.1
${IP_3}=                 20.0.1.1
${IP_4}=                 21.0.1.1
${LOCAL_PORT_1}=         80
${EXT_PORT_1}=           8080
${INTERFACE_NAME_1}=     memif1
${INTERFACE_NAME_2}=     memif2
${MEMIF11_MAC}=          1a:00:00:11:11:11
${MEMIF12_MAC}=          3a:00:00:33:33:33
${error_message_1}=      Evaluating expression 'json.loads('''None''')' failed: ValueError: No JSON object could be decoded


*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 8

Show NATs Aren't Created Before Setup
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Run Keyword And Expect Error  ${error_message_1}  Get VPP NAT44 Config As Json    agent_vpp_1    dnat1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Run Keyword And Expect Error  ${error_message_1}  Get VPP NAT44 Config As Json    agent_vpp_1    dnat2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Run Keyword And Expect Error  ${error_message_1}  Get VPP NAT44 Global Config As Json    agent_vpp_1

Add VPP1_memif1 And VPP1_memif2 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MEMIF11_MAC}
    Put Memif Interface With IP    node=agent_vpp_1    name=memif1    mac=${MEMIF11_MAC}    master=true    id=1    ip=${IP_3}    prefix=24    socket=default.sock
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MEMIF12_MAC}
    Put Memif Interface With IP    node=agent_vpp_1    name=memif2    mac=${MEMIF12_MAC}    master=true    id=2    ip=${IP_4}    prefix=24    socket=default.sock

Add NAT1 And Nat Global And Check Are Created
    Create DNat On agent_vpp_1 With Name dnat1 Local IP ${IP_1} Local Port ${LOCAL_PORT_1} External IP ${IP_3} External Interface ${INTERFACE_NAME_1} External Port ${EXT_PORT_1} Vrf Id 0
    Create Interface GlobalNat On agent_vpp_1 With First IP ${IP_3} On Inteface ${INTERFACE_NAME_1} And Second IP ${IP_4} On Interface ${INTERFACE_NAME_2} Vrf Id 0 Config File nat-global.json
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Config As Json    agent_vpp_1    dnat1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Global Config As Json    agent_vpp_1
    vpp_term: Check DNAT Global exists    agent_vpp_1    dnat_global_output_match.txt

Add NAT2 And Check Is Created
    Create DNat On agent_vpp_1 With Name dnat2 Local IP ${IP_2} Local Port ${LOCAL_PORT_1} External IP ${IP_4} External Interface ${INTERFACE_NAME_2} External Port ${EXT_PORT_1} Vrf Id 0
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Config As Json    agent_vpp_1    dnat1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Config As Json    agent_vpp_1    dnat2
    vpp_term: Check DNAT exists    agent_vpp_1    dnat_all_output_match.txt

Delete NAT1 And Check NAT2 After Delete
    Remove DNat On agent_vpp_1 With Name dnat1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Run Keyword And Expect Error  ${error_message_1}  Get VPP NAT44 Config As Json  agent_vpp_1  dnat1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Config As Json    agent_vpp_1    dnat2
    vpp_term: Check DNAT exists    agent_vpp_1    dnat_output_match.txt

Rewrite NAT Global And Check
    Create Interface GlobalNat On agent_vpp_1 With First IP ${IP_3} On Inteface ${INTERFACE_NAME_1} And Second IP ${IP_4} On Interface ${INTERFACE_NAME_2} Vrf Id 0 Config File nat-global-reduced.json
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Get VPP NAT44 Global Config As Json    agent_vpp_1

Delete NAT Global And Check
    Remove Global Nat On agent_vpp_1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Run Keyword And Expect Error  ${error_message_1}  Get VPP NAT44 Global Config As Json    agent_vpp_1

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
