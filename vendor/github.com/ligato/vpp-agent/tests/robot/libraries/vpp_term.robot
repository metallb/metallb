[Documentation]     Keywords for working with VPP terminal

*** Settings ***
Library      Collections
Library      vpp_term.py

*** Variables ***
${interface_timeout}=     15s
${terminal_timeout}=      30s

*** Keywords ***

vpp_term: Check VPP Terminal
    [Arguments]        ${node}
    [Documentation]    Check terminal on node ${node}
    # using telnet does not work with latest VPP
    #${command}=        Set Variable       telnet 0 ${${node}_VPP_HOST_PORT}
    ${command}=        Set Variable       docker exec -it ${node} vppctl -s localhost:${${node}_VPP_PORT}
    ${out}=            Write To Machine   ${node}_term    ${command}
    Should Contain     ${out}             ${${node}_VPP_TERM_PROMPT}
    [Return]           ${out}

vpp_term: Open VPP Terminal
    [Arguments]    ${node}
    [Documentation]    Wait for VPP terminal on node ${node} or timeout
    wait until keyword succeeds  ${terminal_timeout}    5s   vpp_term: Check VPP Terminal    ${node}

vpp_term: Issue Command
    [Arguments]        ${node}     ${command}    ${delay}=${SSH_READ_DELAY}s
    ${out}=            Write To Machine Until String    ${node}_term    ${command}    ${${node}_VPP_TERM_PROMPT}    delay=${delay}
#    Should Contain     ${out}             ${${node}_VPP_TERM_PROMPT}
    [Return]           ${out}

vpp_term: Exit VPP Terminal
    [Arguments]        ${node}
    ${ctrl_d}          Evaluate    chr(int(4))
    ${command}=        Set Variable       ${ctrl_d}
    ${out}=            Write To Machine   ${node}_term    ${command}
    [Return]           ${out}

vpp_term: Show Interfaces
    [Arguments]        ${node}    ${interface}=${EMPTY}
    [Documentation]    Show interfaces through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh int ${interface}
    [Return]           ${out}

vpp_term: Show Interfaces Address
    [Arguments]        ${node}    ${interface}=${EMPTY}
    [Documentation]    Show interfaces address through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh int addr ${interface}
    [Return]           ${out}

vpp_term: Show Hardware
    [Arguments]        ${node}    ${interface}=${EMPTY}
    [Documentation]    Show interfaces hardware through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh h ${interface}
    [Return]           ${out}

vpp_term: Show IP Fib
    [Arguments]        ${node}    ${ip}=${EMPTY}
    [Documentation]    Show IP fib output
    ${out}=            vpp_term: Issue Command  ${node}    show ip fib ${ip}
    [Return]           ${out}

vpp_term: Show IP6 Fib
    [Arguments]        ${node}    ${ip}=${EMPTY}
    [Documentation]    Show IP fib output
    ${out}=            vpp_term: Issue Command  ${node}    show ip6 fib ${ip}
    [Return]           ${out}

vpp_term: Show IP Fib Table
    [Arguments]        ${node}    ${id}
    [Documentation]    Show IP fib output for VRF table defined in input
    ${out}=            vpp_term: Issue Command  ${node}    show ip fib table ${id}
    [Return]           ${out}

vpp_term: Show IP6 Fib Table
    [Arguments]        ${node}    ${id}
    [Documentation]    Show IP fib output for VRF table defined in input
    ${out}=            vpp_term: Issue Command  ${node}    show ip6 fib table ${id}
    [Return]           ${out}

vpp_term: Show L2fib
    [Arguments]        ${node}
    [Documentation]    Show verbose l2fib output
    ${out}=            vpp_term: Issue Command  ${node}    show l2fib verbose
    [Return]           ${out}

vpp_term: Show Bridge-Domain Detail
    [Arguments]        ${node}    ${id}=1
    [Documentation]    Show detail of bridge-domain
    ${out}=            vpp_term: Issue Command  ${node}    show bridge-domain ${id} detail
    [Return]           ${out}

vpp_term: Show IPsec
    [Arguments]        ${node}
    [Documentation]    Show IPsec output
    ${out}=            vpp_term: Issue Command  ${node}    show ipsec
    [Return]           ${out}

vpp_term: Check Ping
    [Arguments]        ${node}    ${ip}     ${count}=5
    ${out}=            vpp_term: Issue Command    ${node}    ping ${ip} repeat ${count}   delay=10s
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

vpp_term: Check Ping6
    [Arguments]        ${node}    ${ip}     ${count}=5
    ${out}=            vpp_term: Issue Command    ${node}    ping ${ip} repeat ${count}   delay=10s
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

vpp_term: Check Ping Within Interface
    [Arguments]        ${node}    ${ip}    ${source}     ${count}=5
    ${out}=            vpp_term: Issue Command    ${node}    ping ${ip} source ${source} repeat ${count}   delay=10s
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

vpp_term: Check No Ping Within Interface
    [Arguments]        ${node}    ${ip}    ${source}     ${count}=5
    ${out}=            vpp_term: Issue Command    ${node}    ping ${ip} source ${source} repeat ${count}   delay=10s
    Should Not Contain     ${out}    from ${ip}
    Should Contain    ${out}    100% packet loss

vpp_term: Check Interface Presence
    [Arguments]        ${node}     ${mac}    ${status}=${TRUE}
    [Documentation]    Checking if specified interface with mac exists in VPP
    ${ints}=           vpp_term: Show Hardware    ${node}
    ${result}=         Run Keyword And Return Status    Should Contain    ${ints}    ${mac}
    Should Be Equal    ${result}    ${status}

vpp_term: Interface Is Created
    [Arguments]    ${node}    ${mac}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    vpp_term: Check Interface Presence    ${node}    ${mac}

vpp_term: Interface Is Deleted
    [Arguments]    ${node}    ${mac}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    vpp_term: Check Interface Presence    ${node}    ${mac}    ${FALSE}

vpp_term: Interface Exists
    [Arguments]    ${node}    ${mac}
    vpp_term: Check Interface Presence    ${node}    ${mac}

vpp_term: Interface Not Exists
    [Arguments]    ${node}    ${mac}
    vpp_term: Check Interface Presence    ${node}    ${mac}    ${FALSE}

vpp_term: Check Interface UpDown Status
    [Arguments]          ${node}     ${interface}    ${status}=1
    [Documentation]      Checking up/down state of specified internal interface
    ${internal_index}=   vat_term: Get Interface Index    agent_vpp_1    ${interface}
    ${interfaces}=       vat_term: Interfaces Dump    agent_vpp_1
    ${int_state}=        Get Interface State    ${interfaces}    ${internal_index}
    ${enabled}=          Set Variable    ${int_state["admin_up_down"]}
    Should Be Equal As Integers    ${enabled}    ${status}

vpp_term: Get Interface IPs
    [Arguments]          ${node}     ${interface}
    ${int_addr}=         vpp_term: Show Interfaces Address    ${node}    ${interface}
    @{ipv4_list}=        Find IPV4 In Text    ${int_addr}
    [Return]             ${ipv4_list}

vpp_term: Get Interface IP6 IPs
    [Arguments]          ${node}     ${interface}
    [Documentation]    Get all IPv6 addresses for the specified interface.
    ${int_addr}=         vpp_term: Show Interfaces Address    ${node}    ${interface}
    @{ipv6_list}=        Find IPV6 In Text    ${int_addr}
    # Remove link-local address as it is hardware-dependent
    :FOR    ${address}    IN    @{ipv6_list}
    \    Run Keyword If    ${address.startswith('fd80:')}    Remove Values From List    ${ipv6_list}    ${address}
    [Return]             ${ipv6_list}

vpp_term: Get Interface MAC
    [Arguments]          ${node}     ${interface}
    ${sh_h}=             vpp_term: Show Hardware    ${node}    ${interface}
    ${mac}=              Find MAC In Text    ${sh_h}
    [Return]             ${mac}

vpp_term: Interface Is Enabled
    [Arguments]          ${node}     ${interface}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    vpp_term: Check Interface UpDown Status    ${node}     ${interface}

vpp_term: Interface Is Disabled
    [Arguments]          ${node}     ${interface}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    vpp_term: Check Interface UpDown Status    ${node}     ${interface}    0

vpp_term: Interface Is Up
    [Arguments]          ${node}     ${interface}
    vpp_term: Check Interface UpDown Status    ${node}     ${interface}

vpp_term: Interface Is Down
    [Arguments]          ${node}     ${interface}
    vpp_term: Check Interface UpDown Status    ${node}     ${interface}    0

vpp_term: Show Memif
    [Arguments]        ${node}    ${interface}=${EMPTY}
    [Documentation]    Show memif interfaces through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh memif ${interface}
    [Return]           ${out}

vpp_term: Check TAP Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    Sleep                 10s    Time to let etcd to get state of newly setup tap interface.
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${interface}=        vpp_term: Show Interfaces    ${node}    ${internal_name}
    ${state}=            Set Variable    up
    ${status}=           Evaluate     "${state}" in """${interface}"""
    ${tap_int_state}=    Set Variable If    ${status}==True    ${state}    down
    ${ipv4}=             vpp_term: Get Interface IPs    ${node}     ${internal_name}
    ${ipv4_string}=      Get From List    ${ipv4}    0
    ${mac}=              vpp_term: Get Interface MAC    ${node}    ${internal_name}
    ${actual_state}=     Create List    mac=${mac}    ipv4=${ipv4_string}    state=${tap_int_state}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vpp_term: Check TAP IP6 Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    [Documentation]    Get operational state of the specified interface and compare with expected state.
    Sleep                 10s    Time to let etcd to get state of newly setup tap interface.
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${interface}=        vpp_term: Show Interfaces    ${node}    ${internal_name}
    ${state}=            Set Variable    up
    ${status}=           Evaluate     "${state}" in """${interface}"""
    ${tap_int_state}=    Set Variable If    ${status}==True    ${state}    down
    ${ipv6}=             vpp_term: Get Interface IP6 IPs    ${node}     ${internal_name}
    ${ipv6_string}=      Get From List    ${ipv6}    0
    ${mac}=              vpp_term: Get Interface MAC    ${node}    ${internal_name}
    ${actual_state}=     Create List    mac=${mac}    ipv6=${ipv6_string}    state=${tap_int_state}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vpp_term: Show ACL
    [Arguments]        ${node}
    [Documentation]    Show ACLs through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh acl-plugin acl
    [Return]           ${out}

vpp_term: Add Route
    [Arguments]    ${node}    ${destination_ip}    ${prefix}    ${next_hop_ip}
    [Documentation]    Add ip route through vpp terminal.
    vpp_term: Issue Command    ${node}    ip route add ${destination_ip}/${prefix} via ${next_hop_ip}

vpp_term: Show ARP
    [Arguments]        ${node}
    [Documentation]    Show ARPs through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh ip arp
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply_arp.json    ${out}
    [Return]           ${out}

vpp_term: Check ARP
    [Arguments]        ${node}      ${interface}    ${ipv4}     ${MAC}    ${presence}
    [Documentation]    Check ARPs presence on interface
    ${out}=            vpp_term: Show ARP    ${node}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${interface}
    #Should Not Be Equal      ${internal_name}    ${None}
    ${status}=         Run Keyword If     '${internal_name}'!='${None}'  Parse ARP    ${out}   ${internal_name}   ${ipv4}     ${MAC}   ELSE    Set Variable   False
    Should Be Equal As Strings   ${status}   ${presence}

vpp_term: Show Application Namespaces
    [Arguments]        ${node}
    [Documentation]    Show application namespaces through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh app ns
    [Return]           ${out}

vpp_term: Return Data From Show Application Namespaces Output
    [Arguments]    ${node}    ${id}
    [Documentation]    Returns a list containing namespace id, index, namespace secret and sw_if_index of an
    ...   interface associated with the namespace.
    ${out}=    vpp_term: Show Application Namespaces    ${node}
    ${out_line}=    Get Lines Containing String    ${out}    ${id}
    ${out_data}=    Split String    ${out_line}
    [Return]    ${out_data}

vpp_term: Check Data In Show Application Namespaces Output
    [Arguments]    ${node}    ${id}    @{desired_state}
    [Documentation]    Desired data is a list variable containing namespace index, namespace secret and sw_if_index of an
    ...   interface associated with the namespace.
    ${actual_state}=    vpp_term: Return Data From Show Application Namespaces Output    ${node}    ${id}
    List Should Contain Sub List    ${actual_state}    ${desired_state}

vpp_term: Show Interface Mode
    [Arguments]        ${node}
    [Documentation]    vpp_term: Show Interfaces Mode
    ${out}=            vpp_term: Issue Command  ${node}    show mode
    [Return]           ${out}

vpp_term: Check TAPv2 Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    Sleep                 10s    Time to let etcd to get state of newly setup tapv2 interface.
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${interface}=        vpp_term: Show Interfaces    ${node}    ${internal_name}
    ${state}=            Set Variable    up
    ${status}=           Evaluate     "${state}" in """${interface}"""
    ${tap_int_state}=    Set Variable If    ${status}==True    ${state}    down
    ${ipv4}=             vpp_term: Get Interface IPs    ${node}     ${internal_name}
    ${ipv4_string}=      Get From List    ${ipv4}    0
    ${mac}=              vpp_term: Get Interface MAC    ${node}    ${internal_name}
    ${actual_state}=     Create List    mac=${mac}    ipv4=${ipv4_string}    state=${tap_int_state}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vpp_term: Check TAPv2 IP6 Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    Sleep                 10s    Time to let etcd to get state of newly setup tapv2 interface.
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${interface}=        vpp_term: Show Interfaces    ${node}    ${internal_name}
    ${state}=            Set Variable    up
    ${status}=           Evaluate     "${state}" in """${interface}"""
    ${tap_int_state}=    Set Variable If    ${status}==True    ${state}    down
    ${ipv6}=             vpp_term: Get Interface IP6 IPs    ${node}     ${internal_name}
    ${ipv6_string}=      Get From List    ${ipv6}    0
    ${mac}=              vpp_term: Get Interface MAC    ${node}    ${internal_name}
    ${actual_state}=     Create List    mac=${mac}    ipv6=${ipv6_string}    state=${tap_int_state}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vpp_term: Show Trace
    [Arguments]        ${node}
    [Documentation]    vpp_term: Show Trace
    ${out}=            vpp_term: Issue Command  ${node}    show trace
    [Return]           ${out}


vpp_term: Add Trace Memif
    [Arguments]        ${node}
    [Documentation]    vpp_term: Add Trace for memif interfaces
    ${out}=            vpp_term: Issue Command  ${node}    trace add memif-input 10
    [Return]           ${out}


vpp_term: Show STN Rules
    [Arguments]        ${node}
    [Documentation]    Show STN Rules
    ${out}=            vpp_term: Issue Command  ${node}   show stn rules
    [Return]           ${out}

vpp_term: Check STN Rule State
    [Arguments]        ${node}  ${interface}  ${ip}
    [Documentation]    Check STN Rules
    ${out}=            vpp_term: Show STN Rules    ${node}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${interface}
    ${ip_address}  ${iface}  ${next_node}  Parse STN Rule    ${out}
    Should Be Equal As Strings   ${ip}  ${ip_address}
    Should Be Equal As Strings   ${internal_name}  ${iface}

vpp_term: Check STN Rule Deleted
    [Arguments]        ${node}  ${interface}  ${ip}
    [Documentation]    Check STN Rules
    ${out}=            vpp_term: Show STN Rules    ${node}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${interface}
    Should Not Contain     ${out}    ${ip}
    Should Not Contain     ${out}    ${internal_name}

vpp_term: Add Trace Afpacket
    [Arguments]        ${node}
    [Documentation]    vpp_term: Add Trace for afpacket interfaces
    ${out}=            vpp_term: Issue Command  ${node}    trace add af-packet-input 10
    [Return]           ${out}

vpp_term: Set VPP Tracing And Debugging
    [Arguments]        ${node}
    [Documentation]    vpp_term: Add More Tracing and debugging
    ${out}=            vpp_term: Issue Command  ${node}    clear hardware
    ${out}=            vpp_term: Issue Command  ${node}    clear interface
    ${out}=            vpp_term: Issue Command  ${node}    clear error
    ${out}=            vpp_term: Issue Command  ${node}    clear run
    ${out}=            vpp_term: Issue Command  ${node}    api trace on
    ${out}=            vpp_term: Issue Command  ${node}    api trace post-mortem-on
    [Return]           ${out}

vpp_term: Dump Trace
    [Arguments]        ${node}
    [Documentation]    vpp_term: Dump VPP Trace
    ${out}=            vpp_term: Issue Command  ${node}    api trace save apitrace.trc
    [Return]           ${out}


vpp_term: Check Local SID Presence
    [Arguments]        ${node}     ${sidAddress}    ${interface}    ${nexthop}
    [Documentation]    Checking if specified local sid exists or will show up
    #${terminal_timeout}
    Wait Until Keyword Succeeds    5x    2s    vpp_term: Local SID exists    node=${node}     sidAddress=${sidAddress}    interface=${interface}    nexthop=${nexthop}

vpp_term: Local SID exists
    [Arguments]        ${node}     ${sidAddress}    ${interface}    ${nexthop}
    [Documentation]    Checking if specified local sid exists
    ${localsidsStr}=   vpp_term: Show Local SIDs    ${node}
    Create File        /tmp/srv6_sh_sr_localsid_output.txt    ${localsidsStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${localsidsStr}=   OperatingSystem.Get File    /tmp/srv6_sh_sr_localsid_output.txt
    ${localsidsStr}=   Basic_Operations.Replace_Rn_N   ${localsidsStr}    #FIX for BUG with New Line
    ${localsidsStr}=   Convert To Lowercase    ${localsidsStr}
    ${matchdata}=      OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_localsid_output_match.txt
    ${matchdata}=      Replace Variables           ${matchdata}
    ${matchdata}=      Convert To Lowercase    ${matchdata}
    Should Contain    ${localsidsStr}    ${matchdata}

vpp_term: Show Local SIDs
    [Arguments]        ${node}
    [Documentation]    Show locasids through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh sr localsids
    [Return]           ${out}

vpp_term: Check Local SID Deleted
    [Arguments]        ${node}     ${sidAddress}
    [Documentation]    Checking if specified local sid will be(or already is) deleted
    Wait Until Keyword Succeeds    5x    2s    vpp_term: Local SID doesnt exist    node=${node}     sidAddress=${sidAddress}

vpp_term: Local SID doesnt exist
    [Arguments]           ${node}     ${sidAddress}
    [Documentation]       Checking if specified local sid doesnt exist
    ${localsidsStr}=      vpp_term: Show Local SIDs    agent_vpp_1
    Create File           /tmp/srv6_sh_sr_localsid_output.txt    ${localsidsStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${localsidsStr}=      OperatingSystem.Get File    /tmp/srv6_sh_sr_localsid_output.txt
    ${localsidsStr}=      Convert To Lowercase    ${localsidsStr}
    ${matchdata}=         OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_localsid_output_no_match.txt
    ${matchdata}=         Replace Variables           ${matchdata}
    ${matchdata}=         Convert To Lowercase    ${matchdata}
    Should Not Contain    ${localsidsStr}    ${matchdata}

vpp_term: Check SRv6 Policy Presence
    [Arguments]        ${node}    ${bsid}    ${fibtable}    ${behaviour}    ${type}    ${index}    ${segmentlists}
    [Documentation]    Checking if specified SRv6 policy exists or will show up
    #${terminal_timeout}
    Wait Until Keyword Succeeds    5x    2s    vpp_term: SRv6 Policy exists    node=${node}    bsid=${bsid}    fibtable=${fibtable}    behaviour=${behaviour}    type=${type}    index=${index}    segmentlists=${segmentlists}

vpp_term: SRv6 Policy exists
    [Arguments]        ${node}    ${bsid}    ${fibtable}    ${behaviour}    ${type}    ${index}    ${segmentlists}
    [Documentation]    Checking if specified SRv6 policy exists
    ${policyStr}=      vpp_term: Show SRv6 policies    ${node}
    Create File        /tmp/srv6_sh_sr_policies_output.txt    ${policyStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${policyStr}=      OperatingSystem.Get File    /tmp/srv6_sh_sr_policies_output.txt
    ${policyStr}=      Basic_Operations.Replace_Rn_N   ${policyStr}    #FIX for BUG with New Line
    ${policyStr}=      Convert To Lowercase    ${policyStr}
    ${policymatchdata}=     OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_policies_output_match.txt
    ${policymatchdata}=     Replace Variables           ${policymatchdata}
    ${policymatchdata}=     Convert To Lowercase    ${policymatchdata}
    ${segmentlistsmatchdata}=    Set Variable    ${EMPTY}
    :FOR    ${segmentlist}    IN    @{segmentlists}
    \    ${segmentlistmatchdata}=    OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_policy_segments_output_match.txt
    \    ${segmentlistmatchdata}=    Replace Variables           ${segmentlistmatchdata}
    \    ${segmentlistmatchdata}=    Convert To Lowercase    ${segmentlistmatchdata}
    \    ${segmentlistsmatchdata}=    Catenate    SEPARATOR=  ${segmentlistsmatchdata}   ${segmentlistmatchdata}
    ${matchdata}=    Catenate    SEPARATOR=  ${policymatchdata}   ${segmentlistsmatchdata}
    Should Contain    ${policyStr}    ${matchdata}

vpp_term: Show SRv6 policies
    [Arguments]        ${node}
    [Documentation]    Show SRv6 policies through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh sr policies
    [Return]           ${out}

vpp_term: Check SRv6 Policy Nonexistence
    [Arguments]        ${node}    ${bsid}
    [Documentation]    Checking if specified SRv6 policy doesn't exist (or will be deleted soon)
    Wait Until Keyword Succeeds    5x    2s    vpp_term: SRv6 Policy doesnt exist    node=${node}     bsid=${bsid}

vpp_term: SRv6 Policy doesnt exist
    [Arguments]           ${node}     ${bsid}
    [Documentation]       Checking if specified SRv6 policy doesnt exist
    ${policyStr}=         vpp_term: Show SRv6 policies    ${node}
    Create File           /tmp/srv6_sh_sr_policies_output.txt    ${policyStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${policyStr}=         OperatingSystem.Get File    /tmp/srv6_sh_sr_policies_output.txt
    ${policyStr}=         Convert To Lowercase    ${policyStr}
    ${matchdata}=         OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_policies_output_no_match.txt
    ${matchdata}=         Replace Variables           ${matchdata}
    ${matchdata}=         Convert To Lowercase    ${matchdata}
    Should Not Contain    ${policyStr}    ${matchdata}

vpp_term: Check SRv6 Steering Presence
    [Arguments]        ${node}    ${bsid}    ${prefixAddress}
    [Documentation]    Checking if specified steering exists or will show up
    #${terminal_timeout}
    Wait Until Keyword Succeeds    5x    2s    vpp_term: SRv6 Steering exists    node=${node}    bsid=${bsid}     prefixAddress=${prefixAddress}

vpp_term: SRv6 Steering exists
    [Arguments]        ${node}    ${bsid}    ${prefixAddress}
    [Documentation]    Checking if specified steering exists
    ${steeringStr}=    vpp_term: Show SRv6 steering policies    ${node}
    Create File        /tmp/srv6_sh_sr_steerings_output.txt    ${steeringStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${steeringStr}=    OperatingSystem.Get File    /tmp/srv6_sh_sr_steerings_output.txt
    ${steeringStr}=    Convert To Lowercase    ${steeringStr}
    ${matchdata}=      OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_steering_output_match.txt
    ${matchdata}=      Replace Variables           ${matchdata}
    ${matchdata}=      Convert To Lowercase    ${matchdata}
    Should Contain    ${steeringStr}    ${matchdata}

vpp_term: Show SRv6 steering policies
    [Arguments]        ${node}
    [Documentation]    Show SRv6 steering policies through vpp terminal
    ${out}=            vpp_term: Issue Command  ${node}   sh sr steering policies
    [Return]           ${out}

vpp_term: Check SRv6 Steering NonExistence
    [Arguments]        ${node}    ${bsid}    ${prefixAddress}
    [Documentation]    Checking if specified steering is deleted (or soon will be deleted)
    #${terminal_timeout}
    Wait Until Keyword Succeeds    5x    2s    vpp_term: SRv6 Steering doesnt exist    node=${node}    bsid=${bsid}     prefixAddress=${prefixAddress}

vpp_term: SRv6 Steering doesnt exist
    [Arguments]           ${node}    ${bsid}    ${prefixAddress}
    [Documentation]       Checking if specified steering doesnt exist
    ${steeringStr}=       vpp_term: Show SRv6 steering policies    ${node}
    Create File           /tmp/srv6_sh_sr_steerings_output.txt    ${steeringStr}   #FIXME remove dirty trick with saving string to file just to be able to match substring in string
    ${steeringStr}=       OperatingSystem.Get File    /tmp/srv6_sh_sr_steerings_output.txt
    ${steeringStr}=       Convert To Lowercase    ${steeringStr}
    ${matchdata}=         OperatingSystem.Get File    ${CURDIR}/../suites/crud/test_data/srv6_sh_sr_steering_output_match.txt
    ${matchdata}=         Replace Variables           ${matchdata}
    ${matchdata}=         Convert To Lowercase    ${matchdata}
    Should Not Contain    ${steeringStr}    ${matchdata}