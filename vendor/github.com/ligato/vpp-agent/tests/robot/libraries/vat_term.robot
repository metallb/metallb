[Documentation]     Keywords for working with VAT terminal

*** Settings ***
Library      vat_term.py

*** Variables ***
${terminal_timeout}=      30s
${bd_timeout}=            15s

*** Keywords ***

vat_term: Check VAT Terminal
    [Arguments]        ${node}
    [Documentation]    Check VAT terminal on node ${node}
    ${out}=            Write To Machine    ${node}_vat    ${DOCKER_COMMAND} exec -it ${node} /bin/bash
    ${command}=        Set Variable        ${VAT_START_COMMAND}
    ${out}=            Write To Machine    ${node}_vat    ${command}
    Should Contain     ${out}              ${${node}_VPP_VAT_PROMPT}
    [Return]           ${out}

vat_term: Open VAT Terminal
    [Arguments]    ${node}
    [Documentation]    Wait for VAT terminal on node ${node} or timeout
    wait until keyword succeeds  ${terminal_timeout}    5s   vat_term: Check VAT Terminal    ${node}

vat_term: Exit VAT Terminal
    [Arguments]        ${node}
    ${ctrl_c}          Evaluate    chr(int(3))
    ${command}=        Set Variable       ${ctrl_c}
    ${out}=            Write To Machine   ${node}_vat    ${command}
    [Return]           ${out}

vat_term: Issue Command
    [Arguments]        ${node}     ${command}    ${delay}=${SSH_READ_DELAY}s
    ${out}=            Write To Machine Until String    ${node}_vat    ${command}    ${${node}_VPP_VAT_PROMPT}    delay=${delay}
#    Should Contain     ${out}             ${${node}_VPP_VAT_PROMPT}
    [Return]           ${out}

vat_term: Interfaces Dump
    [Arguments]        ${node}
    [Documentation]    Executing command sw_interface_dump
    ${out}=            vat_term: Issue Command  ${node}  sw_interface_dump
    [Return]           ${out}

vat_term: Bridge Domain Dump
    [Arguments]        ${node}    ${bd_id}=${EMPTY}
    [Documentation]    Executing command bridge_domain_dump
    ${add_params}=     Set Variable If    '''${bd_id}'''==""    ${EMPTY}    bd_id ${bd_id}
    ${out}=            vat_term: Issue Command  ${node}  bridge_domain_dump ${add_params}
    ${out}=            Evaluate    """${out}"""["""${out}""".find('['):"""${out}""".rfind(']')+1]
    [Return]           ${out}

vat_term: IP FIB Dump
    [Arguments]        ${node}
    [Documentation]    Executing command ip_fib_dump
    ${out}=            vat_term: Issue Command  ${node}  ip_fib_dump
    [Return]           ${out}

vat_term: VXLan Tunnel Dump
    [Arguments]        ${node}    ${args}=${EMPTY}
    [Documentation]    Executing command vxlan_tunnel_dump
    ${out}=            vat_term: Issue Command  ${node}  vxlan_tunnel_dump ${args}
    ${out}=            Evaluate    """${out}"""["""${out}""".find('['):"""${out}""".rfind(']')+1]
    [Return]           ${out}

vat_term: Check VXLan Tunnel Presence
    [Arguments]        ${node}     ${src}    ${dst}    ${vni}    ${status}=${TRUE}
    [Documentation]    Checking if specified vxlan tunnel exists
    ${out}=            vat_term: VXLan Tunnel Dump    ${node}
    ${result}  ${if_index}=    Check VXLan Tunnel Presence    ${out}    ${src}    ${dst}    ${vni}
    Should Be Equal    ${result}    ${status}
    [Return]           ${if_index}

vat_term: Get Interface Name
    [Arguments]        ${node}     ${index}
    [Documentation]    Return interface with specified index name
    ${out}=            vat_term: Interfaces Dump    ${node}
    ${name}=           Get Interface Name    ${out}    ${index}
    [Return]           ${name}

vat_term: Get Interface Index
    [Arguments]        ${node}     ${name}
    [Documentation]    Return interface index with specified name
    ${out}=            vat_term: Interfaces Dump    ${node}
    ${index}=          Get Interface Index    ${out}    ${name}
    [Return]           ${index}

vat_term: Check VXLan Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${internal_index}=   vat_term: Get Interface Index    ${node}    ${internal_name}
    ${vxlan_data}=       vat_term: VXLan Tunnel Dump    ${node}    sw_if_index ${internal_index}
    ${vxlan_data}=       Evaluate    json.loads('''${vxlan_data}''')    json
    ${interfaces}=       vat_term: Interfaces Dump    ${node}
    ${int_state}=        Get Interface State    ${interfaces}    ${internal_index}
    ${src}=              Set Variable    ${vxlan_data[0]["src_address"]}
    ${dst}=              Set Variable    ${vxlan_data[0]["dst_address"]}
    ${vni}=              Set Variable    ${vxlan_data[0]["vni"]}
    ${enabled}=          Set Variable    ${int_state["admin_up_down"]}
    ${actual_state}=     Create List    src=${src}    dst=${dst}    vni=${vni}    enabled=${enabled}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: Check Afpacket Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${internal_index}=   vat_term: Get Interface Index    ${node}    ${internal_name}
    ${interfaces}=       vat_term: Interfaces Dump    ${node}
    ${int_state}=        Get Interface State    ${interfaces}    ${internal_index}
    ${ipv4_list}=        vpp_term: Get Interface IPs    ${node}    ${internal_name}
    ${config}=           vpp_ctl: Get VPP Interface Config As Json    ${node}    ${name}
    ${host_int}=         Set Variable    ${config["afpacket"]["host_if_name"]}
    #Should Contain       ${internal_name}    ${host_int}
    ${enabled}=          Set Variable    ${int_state["admin_up_down"]}
    ${mtu}=              Set Variable    ${int_state["mtu"]}
    ${dec_mac}=          Set Variable    ${int_state["l2_address"]}
    ${mac}=              Convert Dec MAC To Hex    ${dec_mac}
    ${actual_state}=     Create List    enabled=${enabled}    mtu=${mtu}    mac=${mac}
    :FOR    ${ip}    IN    @{ipv4_list}
    \    Append To List    ${actual_state}    ipv4=${ip}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: Check Physical Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${internal_index}=   vat_term: Get Interface Index    ${node}    ${internal_name}
    ${interfaces}=       vat_term: Interfaces Dump    ${node}
    ${int_state}=        Get Interface State    ${interfaces}    ${internal_index}
    ${ipv4_list}=        vpp_term: Get Interface IPs    ${node}    ${internal_name}
    ${enabled}=          Set Variable    ${int_state["admin_up_down"]}
    ${mtu}=              Set Variable    ${int_state["mtu"]}
    ${dec_mac}=          Set Variable    ${int_state["l2_address"]}
    ${mac}=              Convert Dec MAC To Hex    ${dec_mac}
    ${actual_state}=     Create List    enabled=${enabled}    mtu=${mtu}    mac=${mac}
    :FOR    ${ip}    IN    @{ipv4_list}
    \    Append To List    ${actual_state}    ipv4=${ip}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: Check Loopback Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${internal_index}=   vat_term: Get Interface Index    ${node}    ${internal_name}
    ${interfaces}=       vat_term: Interfaces Dump    ${node}
    ${int_state}=        Get Interface State    ${interfaces}    ${internal_index}
    ${ipv4_list}=        vpp_term: Get Interface IPs    ${node}    ${internal_name}
    ${ipv6_list}=        vpp_term: Get Interface IP6 IPs    ${node}    ${internal_name}
    ${enabled}=          Set Variable    ${int_state["admin_up_down"]}
    ${mtu}=              Set Variable    ${int_state["mtu"]}
    ${dec_mac}=          Set Variable    ${int_state["l2_address"]}
    ${mac}=              Convert Dec MAC To Hex    ${dec_mac}
    ${actual_state}=     Create List    enabled=${enabled}    mtu=${mtu}    mac=${mac}
    :FOR    ${ip}    IN    @{ipv4_list}
    \    Append To List    ${actual_state}    ipv4=${ip}
    :FOR    ${ip}    IN    @{ipv6_list}
    \    Append To List    ${actual_state}    ipv6=${ip}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: Check Memif Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${internal_name}=    vpp_ctl: Get Interface Internal Name    ${node}    ${name}
    ${memif_info}=       vpp_term: Show Memif    ${node}    ${internal_name}
    ${memif_state}=      Parse Memif Info    ${memif_info}
    ${ipv4_list}=        vpp_term: Get Interface IPs    ${node}    ${internal_name}
    ${ipv6_list}=        vpp_term: Get Interface IP6 IPs    ${node}    ${internal_name}
    ${mac}=              vpp_term: Get Interface MAC    ${node}    ${internal_name}
    ${actual_state}=     Create List    mac=${mac}
    :FOR    ${ip}    IN    @{ipv4_list}
    \    Append To List    ${actual_state}    ipv4=${ip}
    :FOR    ${ip}    IN    @{ipv6_list}
    \    Append To List    ${actual_state}    ipv6=${ip}
    Append To List       ${actual_state}    @{memif_state}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: Check Bridge Domain State
    [Arguments]          ${node}    ${bd}    @{desired_state}
    ${bd_id}=            vpp_ctl: Get Bridge Domain ID    ${node}    ${bd}
    ${bd_dump}=          vat_term: Bridge Domain Dump    ${node}    ${bd_id}
    ${bd_json}=          Evaluate    json.loads('''${bd_dump}''')    json
    ${flood}=            Set Variable    ${bd_json[0]["flood"]}
    ${forward}=          Set Variable    ${bd_json[0]["forward"]}
    ${learn}=            Set Variable    ${bd_json[0]["learn"]}
    ${bd_details}=       vpp_term: Show Bridge-Domain Detail    ${node}    ${bd_id}
    ${bd_state}=         Parse BD Details    ${bd_details}
    ${etcd_dump}=        Get ETCD Dump
    ${etcd_json}=        Convert_ETCD_Dump_To_JSON    ${etcd_dump}
    ${interfaces}=       Parse BD Interfaces    ${node}    ${bd}    ${etcd_json}    ${bd_dump}
    ${actual_state}=     Create List    flood=${flood}    forward=${forward}    learn=${learn}
    Append To List       ${actual_state}    @{bd_state}    @{interfaces}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

vat_term: BD Is Created
    [Arguments]    ${node}    @{interfaces}
    Wait Until Keyword Succeeds    ${bd_timeout}   3s    vat_term: Check BD Presence    ${node}    ${interfaces}

vat_term: BD Is Deleted
    [Arguments]    ${node}    @{interfaces}
    Wait Until Keyword Succeeds    ${bd_timeout}   3s    vat_term: Check BD Presence    ${node}    ${interfaces}    ${FALSE}

vat_term: BD Exists
    [Arguments]    ${node}    @{interfaces}
    vat_term: Check BD Presence    ${node}    ${interfaces}

vat_term: BD Not Exists
    [Arguments]    ${node}    @{interfaces}
    vat_term: Check BD Presence    ${node}    ${interfaces}    ${FALSE}

vat_term: Check BD Presence
    [Arguments]        ${node}     ${interfaces}    ${status}=${TRUE}
    ${indexes}=    Create List
    :FOR    ${int}    IN    @{interfaces}
    \    ${sw_if_index}=    vpp_ctl: Get Interface Sw If Index    ${node}    ${int}
    \    Append To List    ${indexes}    ${sw_if_index}
    ${bd_dump}=        vat_term: Bridge Domain Dump    ${node}
    ${result}=         Check BD Presence    ${bd_dump}    ${indexes}
    Should Be Equal    ${result}    ${status}


vat_term: ACL Dump
    [Arguments]        ${node}   ${acl_name}
    [Documentation]    Executing command acl_dump
    ${out}=            vat_term: Issue Command  ${node}  acl_dump
    ${out_data_vat}=   Strip String     ${out}
    ${out_data}=       Remove String     ${out_data_vat}    vat#${SPACE}   vat#
    OperatingSystem.Create File    ${REPLY_DATA_FOLDER}/reply_${acl_name}_term.json    ${out_data}
    [Return]           ${out_data}

vat_term: ACL All Dump
    [Arguments]        ${node}
    [Documentation]    Executing command acl_dump
    ${out}=            vat_term: Issue Command  ${node}  acl_dump
    ${out_data_vat}=   Strip String     ${out}
    ${out_data}=       Remove String     ${out_data_vat}      vat#${SPACE}   vat#
    OperatingSystem.Create File    ${REPLY_DATA_FOLDER}/reply_acl_all_term.json     ${out_data}
    [Return]           ${out_data}


vat_term: Check All ACL
    [Arguments]        ${node}
    ${acl_list}=       vat_term: ACL All Dump    ${node}
    [Return]           ${acl_list}

vat_term: Check ACL
    [Arguments]        ${node}    ${acl_name}
    ${acl_list}=       vat_term: ACL Dump    ${node}    ${acl_name}
    [Return]           ${acl_list}