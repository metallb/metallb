*** Settings ***
Library     OperatingSystem
Library     String

Resource     ../../variables/${VARIABLES}_variables.robot
Resource    ../../libraries/all_libs.robot
Resource    ../../libraries/pretty_keywords.robot

Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown

*** Variables ***
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s

*** Test Cases ***
# CRUD tests for IPsec
Add Agent Vpp Node
    Add Agent VPP Node                 agent_vpp_1

Add SA1 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA sa 1
    vpp_ctl: Create IPsec With SA And Json  agent_vpp_1   sa10   ipsec-sa.json  sa10  1001  4a506a794f574265564551694d653768  4339314b55523947594d6d3547666b45764e6a58
#    Create IPsec On agent_vpp_1 With SA sa10 And Json ipsec-sa.json
    IP Sec On agent_vpp_1 Should Contain SA sa 1

Add SA2 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA sa 2
    vpp_ctl: Create IPsec With SA And Json  agent_vpp_1   sa20   ipsec-sa.json  sa20  1000  4a506a794f574265564551694d653768  4339314b55523947594d6d3547666b45764e6a58
#    Create IPsec On agent_vpp_1 With SA sa20 And Json ipsec-sa20.json
    IP Sec On agent_vpp_1 Should Contain SA sa 2

Add SPD1 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA spd 1
    vpp_ctl: Create IPsec With SPD And Json  agent_vpp_1    spd1    ipsec-spd.json    afp1    10.0.0.1    10.0.0.2    sa10  sa20
    IP Sec On agent_vpp_1 Should Contain SA spd 1

Check IPsec config_1 On VPP
    IP Sec Should Contain  agent_vpp_1  sa 1  sa 2  spd 1  IPSEC_ESP  outbound policies

Add SA3 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA sa 3
    vpp_ctl: Create IPsec With SA And Json  agent_vpp_1   sa30   ipsec-sa.json  sa30  1003  4a506a794f574265564551694d653770  4339314b55523947594d6d3547666b45764e6a60
    IP Sec On agent_vpp_1 Should Contain SA sa 3

Add SA4 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA sa 4
    vpp_ctl: Create IPsec With SA And Json  agent_vpp_1   sa40   ipsec-sa.json  sa40  1002  4a506a794f574265564551694d653770  4339314b55523947594d6d3547666b45764e6a60
    IP Sec On agent_vpp_1 Should Contain SA sa 4

Add SPD2 Into VPP
    IP Sec On agent_vpp_1 Should Not Contain SA spd 2
    vpp_ctl: Create IPsec With SPD And Json  agent_vpp_1    spd2    ipsec-spd.json    afp2    10.0.0.3    10.0.0.4    sa30  sa40
    IP Sec On agent_vpp_1 Should Contain SA spd 2

Check IPsec config_1 On VPP After Add SPD2
    IP Sec Should Contain  agent_vpp_1  sa 1  sa 2  spd 1  IPSEC_ESP  outbound policies

Check IPsec config_2 On VPP
    IP Sec Should Contain  agent_vpp_1  sa 3  sa 4  spd 2  IPSEC_ESP  outbound policies

Delete SAs And SPD1 For Default IPsec
    Delete IPsec On agent_vpp_1 With Prefix sa And Name sa10
    Delete IPsec On agent_vpp_1 With Prefix sa And Name sa20
    Delete IPsec On agent_vpp_1 With Prefix spd And Name spd1
    IP Sec On agent_vpp_1 Should Not Contain SA sa 1
    IP Sec On agent_vpp_1 Should Not Contain SA sa 2
    IP Sec On agent_vpp_1 Should Not Contain SA spd 1

Check IPsec config_2 On VPP After Delete SPD1
    IP Sec Should Contain  agent_vpp_1  sa 3  sa 4  spd 2  IPSEC_ESP  outbound policies

Delete SAs And SPD2 For Default IPsec
    Delete IPsec On agent_vpp_1 With Prefix sa And Name sa30
    Delete IPsec On agent_vpp_1 With Prefix sa And Name sa40
    Delete IPsec On agent_vpp_1 With Prefix spd And Name spd2
    IP Sec On agent_vpp_1 Should Not Contain SA sa 3
    IP Sec On agent_vpp_1 Should Not Contain SA sa 4
    IP Sec On agent_vpp_1 Should Not Contain SA spd 2


*** Keywords ***
IP Sec On ${node} Should Not Contain SA ${sa}
    ${out}=    vpp_term: Show IPsec    ${node}
    Should Not Contain  ${out}  ${sa}

IP Sec On ${node} Should Contain SA ${sa}
    ${out}=    vpp_term: Show IPsec    ${node}
    Should Contain  ${out}  ${sa}

IP Sec Should Contain
    [Arguments]     ${node}  ${sa_name_1}  ${sa_name_2}  ${spd_name_1}  ${ipsec_esp}  ${outbound_policies}
    ${out}=         vpp_term: Show IPsec    ${node}
    Run Keyword Unless  "${sa_name_1}" == "${EMPTY}"   Should Contain  ${out}  ${sa_name_1}
    Run Keyword Unless  "${sa_name_2}" == "${EMPTY}"   Should Contain  ${out}  ${sa_name_2}
    Run Keyword Unless  "${spd_name_1}" == "${EMPTY}"   Should Contain  ${out}  ${spd_name_1}
    Run Keyword Unless  "${ipsec_esp}" == "${EMPTY}"   Should Contain  ${out}  ${ipsec_esp}
    Run Keyword Unless  "${outbound_policies}" == "${EMPTY}"   Should Contain  ${out}  ${outbound_policies}
