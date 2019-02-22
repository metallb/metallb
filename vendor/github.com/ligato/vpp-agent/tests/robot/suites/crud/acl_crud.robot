*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String
Library      Collections

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Suite Cleanup
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${REPLY_DATA_FOLDER}            ${CURDIR}/replyACL
${VARIABLES}=       common
${ENV}=             common
${api_handler}=     215
${ACL1_NAME}=       acl1_tcp
${ACL2_NAME}=       acl2_tcp
${ACL3_NAME}=       acl3_UDP
${ACL4_NAME}=       acl4_UDP
${ACL5_NAME}=       acl5_ICMP
${ACL6_NAME}=       acl6_ICMP
${E_INTF1}=
${I_INTF1}=
${E_INTF2}=
${I_INTF2}=
#${RULE_NM1_1}=         acl1_rule1
#${RULE_NM2_1}=         acl2_rule1
#${RULE_NM3_1}=         acl3_rule1
#${RULE_NM4_1}=         acl4_rule1
#${RULE_NM5_1}=         acl5_rule1
#${RULE_NM6_1}=         acl6_rule1
${ACTION_DENY}=     1
${ACTION_PERMIT}=   2
${DEST_NTW}=        10.0.0.0/32
${SRC_NTW}=         10.0.0.0/32
${1DEST_PORT_L}=     80
${1DEST_PORT_U}=     1000
${1SRC_PORT_L}=      10
${1SRC_PORT_U}=      2000
${2DEST_PORT_L}=     2000
${2DEST_PORT_U}=     2200
${2SRC_PORT_L}=      20010
${2SRC_PORT_U}=      20020
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${NO_ACL}=



*** Test Cases ***
Configure Environment
    [Tags]    setup
    ${DATA_FOLDER}=       Catenate     SEPARATOR=/       ${CURDIR}         ${TEST_DATA_FOLDER}
    Set Suite Variable          ${DATA_FOLDER}
    Configure Environment 2        acl_basic.conf
    Set Suite Variable    ${API_HANDLER}    ${api_handler}

Show ACL Before Setup
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL1_NAME}    ${REPLY_DATA_FOLDER}/reply_acl_empty.txt     ${REPLY_DATA_FOLDER}/reply_acl_empty_term.txt

Add ACL1_TCP
    Put ACL TCP   agent_vpp_1   ${ACL1_NAME}    ${E_INTF1}    ${I_INTF1}    ${ACTION_DENY}     ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}

Check ACL1 is created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL1_NAME}    ${REPLY_DATA_FOLDER}/reply_acl1_tcp.txt    ${REPLY_DATA_FOLDER}/reply_acl1_tcp_term.txt


Add ACL2_TCP
    Put ACL TCP   agent_vpp_1   ${ACL2_NAME}    ${E_INTF1}    ${I_INTF1}    ${ACTION_DENY}     ${DEST_NTW}     ${SRC_NTW}   ${2DEST_PORT_L}   ${2DEST_PORT_U}    ${2SRC_PORT_L}     ${2SRC_PORT_U}


Check ACL2 is created and ACL1 still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL2_NAME}   ${REPLY_DATA_FOLDER}/reply_acl2_tcp.txt    ${REPLY_DATA_FOLDER}/reply_acl2_tcp_term.txt



Update ACL1
    Put ACL TCP   agent_vpp_1   ${ACL1_NAME}    ${E_INTF1}     ${I_INTF1}     ${ACTION_PERMIT}     ${DEST_NTW}    ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}


Check ACL1 Is Changed and ACL2 not changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL1_NAME}    ${REPLY_DATA_FOLDER}/reply_acl1_update_tcp.txt    ${REPLY_DATA_FOLDER}/reply_acl1_update_tcp_term.txt

Delete ACL2
    Delete ACL     agent_vpp_1    ${ACL2_NAME}

Check ACL2 Is Deleted and ACL1 Is Not Changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL2_NAME}    ${REPLY_DATA_FOLDER}/reply_acl_empty.txt    ${REPLY_DATA_FOLDER}/reply_acl2_delete_tcp_term.txt

Delete ACL1
    Delete ACL     agent_vpp_1    ${ACL1_NAME}

Check ACL1 Is Deleted
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL1_NAME}    ${REPLY_DATA_FOLDER}/reply_acl_empty.txt   ${REPLY_DATA_FOLDER}/reply_acl_empty_term.txt


ADD ACL3_UDP
    Put ACL UDP    agent_vpp_1    ${ACL3_NAME}    ${E_INTF1}   ${I_INTF1}    ${E_INTF2}    ${I_INTF2}      ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}

Check ACL3 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL3_NAME}    ${REPLY_DATA_FOLDER}/reply_acl3_udp.txt    ${REPLY_DATA_FOLDER}/reply_acl3_udp_term.txt

ADD ACL4_UDP
    Put ACL UDP    agent_vpp_1    ${ACL4_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}      ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}


Check ACL4 Is Created And ACL3 Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL4_NAME}    ${REPLY_DATA_FOLDER}/reply_acl4_udp.txt     ${REPLY_DATA_FOLDER}/reply_acl4_udp_term.txt

Delete ACL4
    Delete ACL     agent_vpp_1    ${ACL4_NAME}

Check ACL4 Is Deleted and ACL3 Is Not Changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL4_NAME}   ${REPLY_DATA_FOLDER}/reply_acl_empty.txt     ${REPLY_DATA_FOLDER}/reply_acl3_udp_term.txt

Delete ACL3
    Delete ACL     agent_vpp_1    ${ACL3_NAME}

Check ACL3 Is Deleted
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL3_NAME}    ${REPLY_DATA_FOLDER}/reply_acl_empty.txt    ${REPLY_DATA_FOLDER}/reply_acl_empty_term.txt

ADD ACL5_ICMP
    Put ACL UDP    agent_vpp_1    ${ACL5_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}     ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}

Check ACL5 Is Created
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL5_NAME}   ${REPLY_DATA_FOLDER}/reply_acl5_icmp.txt    ${REPLY_DATA_FOLDER}/reply_acl5_icmp_term.txt

ADD ACL6_ICMP
    Put ACL UDP    agent_vpp_1    ${ACL6_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}     ${ACTION_DENY}  ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}

Check ACL6 Is Created And ACL5 Still Configured
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL6_NAME}    ${REPLY_DATA_FOLDER}/reply_acl6_icmp.txt    ${REPLY_DATA_FOLDER}/reply_acl6_icmp_term.txt

Delete ACL6
    Delete ACL     agent_vpp_1    ${ACL6_NAME}

Check ACL6 Is Deleted and ACL5 Is Not Changed
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL6_NAME}     ${REPLY_DATA_FOLDER}/reply_acl_empty.txt    ${REPLY_DATA_FOLDER}/reply_acl5_icmp_term.txt

Delete ACL5
    Delete ACL     agent_vpp_1    ${ACL5_NAME}

Check ACL5 Is Deleted
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL Reply    agent_vpp_1    ${ACL5_NAME}   ${REPLY_DATA_FOLDER}/reply_acl_empty.txt     ${REPLY_DATA_FOLDER}/reply_acl_empty_term.txt


Add 6 ACL
    Put ACL TCP   agent_vpp_1   ${ACL1_NAME}    ${E_INTF1}    ${I_INTF1}       ${ACTION_DENY}     ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}
    Put ACL TCP   agent_vpp_1   ${ACL2_NAME}    ${E_INTF1}    ${I_INTF1}       ${ACTION_DENY}     ${DEST_NTW}     ${SRC_NTW}   ${2DEST_PORT_L}   ${2DEST_PORT_U}    ${2SRC_PORT_L}     ${2SRC_PORT_U}
    Put ACL UDP   agent_vpp_1    ${ACL3_NAME}    ${E_INTF1}   ${I_INTF1}    ${E_INTF2}    ${I_INTF2}       ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}
    Put ACL UDP   agent_vpp_1    ${ACL4_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}       ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}
    Put ACL UDP   agent_vpp_1    ${ACL5_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}      ${ACTION_DENY}    ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}
    Put ACL UDP   agent_vpp_1    ${ACL6_NAME}    ${E_INTF1}    ${I_INTF1}    ${E_INTF2}    ${I_INTF2}      ${ACTION_DENY}  ${DEST_NTW}     ${SRC_NTW}   ${1DEST_PORT_L}   ${1DEST_PORT_U}    ${1SRC_PORT_L}     ${1SRC_PORT_U}

Check All 6 ACLs Added
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check ACL All Reply    agent_vpp_1     ${REPLY_DATA_FOLDER}/reply_acl_all.txt        ${REPLY_DATA_FOLDER}/reply_acl_all_term.txt

*** Keywords ***

Check ACL All Reply
    [Arguments]         ${node}    ${reply_json}     ${reply_term}
    ${acl_d}=           Get All ACL As Json    ${node}
    ${term_d}=          vat_term: Check All ACL     ${node}
    ${term_d_lines}=    Split To Lines    ${term_d}
    ${data}=            OperatingSystem.Get File    ${reply_json}
    ${data}=            Replace Variables      ${data}
    Should Be Equal     ${data}   ${acl_d}
    ${data}=            OperatingSystem.Get File    ${reply_term}
    ${data}=            Replace Variables      ${data}
    ${t_data_lines}=    Split To Lines    ${data}
    List Should Contain Sub List    ${term_d_lines}    ${t_data_lines}

TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

Suite Cleanup
    Stop SFC Controller Container
    Testsuite Teardown