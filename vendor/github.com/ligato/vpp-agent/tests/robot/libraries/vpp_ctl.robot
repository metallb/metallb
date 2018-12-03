[Documentation]     Keywords for working with VPP Ctl container

*** Settings ***
Library        vpp_ctl.py
Library        String

*** Variables ***

*** Keywords ***

vpp_ctl: Put Json
    [Arguments]        ${key}    ${json}    ${container}=vpp_agent_ctl
    ${command}=        Set Variable    echo '${json}' | vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -put ${key} -
    ${out}=            Write To Container Until Prompt    ${container}    ${command}
    [Return]           ${out}

vpp_ctl: Read Key
    [Arguments]        ${key}    ${container}=vpp_agent_ctl
    ${command}=        Set Variable    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -get ${key}
#    ${out}=            Write To Container Until Prompt    ${container}    ${command}
    ${out}=            Execute In Container    ${container}    ${command}
    [Return]           ${out}

vpp_ctl: Read Key With Prefix
    [Arguments]        ${key}    ${container}=vpp_agent_ctl
    ${command}=        Set Variable    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -get ${key}
#    ${out}=            Write To Container Until Prompt    ${container}    ${command}
    ${out}=            Execute In Container    ${container}    ${command}
    [Return]           ${out}

vpp_ctl: Put Memif Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${master}    ${id}    ${socket}=memif.sock    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${socket}=            Set Variable                  ${${node}_MEMIF_SOCKET_FOLDER}/${socket}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/memif_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Memif Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${master}    ${id}    ${ip}    ${prefix}=24    ${socket}=memif.sock    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${socket}=            Set Variable                  ${${node}_MEMIF_SOCKET_FOLDER}/${socket}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/memif_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete key
    [Arguments]     ${key}    ${container}=vpp_agent_ctl
    ${out}=         Write To Container Until Prompt    ${container}   vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -del ${key}
    [Return]        ${out}

vpp_ctl: Put Veth Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${peer}    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Veth Interface And Namespace
    [Arguments]    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Veth Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Veth Interface With IP And Namespace
    [Arguments]    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    Log Many    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${ip}    ${prefix}    ${mtu}    ${enabled}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface_with_ip_and_ns.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/interface/${name}
    Log Many              ${data}                       ${uri}
    ${data}=              Replace Variables             ${data}
    Log                   ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Afpacket Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${host_int}    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/afpacket_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put VXLan Interface
    [Arguments]    ${node}    ${name}    ${src}    ${dst}    ${vni}    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/vxlan_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Bridge Domain
    [Arguments]    ${node}    ${name}    ${ints}    ${flood}=true    ${unicast}=true    ${forward}=true    ${learn}=true    ${arp_term}=true
    ${interfaces}=        Create Interfaces Json From List    ${ints}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bridge_domain.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/bd/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Loopback Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${mtu}=1500    ${enabled}=true   ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/loopback_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Loopback Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${prefix}=24    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/loopback_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Physical Interface With IP
    [Arguments]    ${node}    ${name}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/physical_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Get VPP Interface State
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/status/v1/interface/${interface}
    ${out}=               vpp_ctl: Read Key    ${key}
    [Return]              ${out}

vpp_ctl: Get VPP Interface State As Json
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/status/v1/interface/${interface}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get VPP Interface Config As Json
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/config/v1/interface/${interface}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get Linux Interface Config As Json
    [Arguments]    ${node}    ${name}
    ${key}=               Set Variable    /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get Bridge Domain State As Json
    [Arguments]    ${node}    ${bd}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/status/v1/bd/${bd}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get Interface Internal Name
    [Arguments]    ${node}    ${interface}
    ${name}=    Set Variable      ${EMPTY}
    ${empty_dict}=   Create Dictionary
    ${state}=    vpp_ctl: Get VPP Interface State As Json    ${node}    ${interface}
    ${length}=   Get Length     ${state}
    ${name}=    Run Keyword If      ${length} != 0     Set Variable    ${state["internal_name"]}
    [Return]    ${name}

vpp_ctl: Get Interface Sw If Index
    [Arguments]    ${node}    ${interface}
    ${state}=    vpp_ctl: Get VPP Interface State As Json    ${node}    ${interface}
    ${sw_if_index}=    Set Variable    ${state["if_index"]}
    [Return]    ${sw_if_index}

vpp_ctl: Get Bridge Domain ID
    [Arguments]    ${node}    ${bd}
    ${state}=    vpp_ctl: Get Bridge Domain State As Json    ${node}    ${bd}
    ${bd_id}=    Set Variable    ${state["index"]}
    [Return]    ${bd_id}

vpp_ctl: Put TAP Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${host_if_name}    ${prefix}=24    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tap_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put TAP Unnumbered Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${unnumbered}    ${interface_with_ip_name}    ${host_if_name}    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tap_interface_unnumbered.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}


vpp_ctl: Put Static Fib Entry
    [Arguments]    ${node}    ${bd_name}    ${mac}    ${outgoing_interface}    ${static}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/static_fib.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/bd/${bd_name}/fib/${mac}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Create IPsec With SA And Json
    [Arguments]     ${node}    ${interface}    ${file_name}    ${name}    ${spi}    ${crypto_key}    ${integ_key}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../resources/${file_name}
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/vpp/config/v1/ipsec/sa/${interface}
    ${out}=         vpp_ctl: Put Json    ${uri}   ${data}

vpp_ctl: Create IPsec With SPD And Json
    [Arguments]     ${node}    ${spd_name}    ${file_name}    ${interface_name}    ${remote_addr}    ${local_addr}    ${sa_name_1}  ${sa_name_2}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../resources/${file_name}
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/vpp/config/v1/ipsec/spd/${spd_name}
    ${out}=         vpp_ctl: Put Json    ${uri}   ${data}

vpp_ctl: Delete Bridge Domain
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/vpp/config/v1/bd/${name}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Delete VPP Interface
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Delete Linux Interface
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Delete Route
    [Arguments]    ${node}    ${id}    ${ip}    ${prefix}
    ${uri}=    Set Variable                /vnf-agent/${node}/vpp/config/v1/vrf/${id}/fib/${ip}/${prefix}
    ${out}=         vpp_ctl: Delete key  ${uri}
    [Return]       ${out}

vpp_ctl: Delete Routes
    [Arguments]    ${node}    ${id}
    ${uri}=    Set Variable                /vnf-agent/${node}/vpp/config/v1/vrf/${id}/fib
    ${out}=         vpp_ctl: Delete key  ${uri}
    [Return]       ${out}

vpp_ctl: Delete IPsec
    [Arguments]    ${node}    ${prefix}    ${name}
    ${uri}=    Set Variable                /vnf-agent/${node}/vpp/config/v1/ipsec/${prefix}/${name}
    ${out}=         vpp_ctl: Delete key  ${uri}
    [Return]       ${out}

vpp_ctl: Put BFD Session
    [Arguments]    ${node}    ${session_name}    ${min_tx_interval}    ${dest_adr}    ${detect_multiplier}    ${interface}    ${min_rx_interval}    ${source_adr}   ${enabled}    ${auth_key_id}=0    ${BFD_auth_key_id}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_session.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/bfd/session/${session_name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put BFD Authentication Key
    [Arguments]    ${node}    ${key_name}    ${auth_type}    ${id}    ${secret}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_key.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/bfd/auth-key/${key_name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put BFD Echo Function
    [Arguments]    ${node}    ${echo_func_name}    ${source_intf}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_echo_function.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/bfd/echo-function
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Get BFD Session As Json
    [Arguments]    ${node}    ${session_name}
    ${key}=               Set Variable            /vnf-agent/${node}/vpp/config/v1/bfd/session/${session_name}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get BFD Authentication Key As Json
    [Arguments]    ${node}    ${key_name}
    ${key}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/bfd/auth-key/${key_name}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Get BFD Echo Function As Json
    [Arguments]    ${node}
    ${key}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/bfd/echo-function
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Put ACL TCP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}   ${ingr_intf1}    ${acl_action}    ${dest_ntw}    ${src_ntw}    ${dest_port_low}   ${dest_port_up}    ${src_port_low}    ${src_port_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_TCP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl/${acl_name}
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put ACL UDP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}    ${ingr_intf1}     ${egr_intf2}    ${ingr_intf2}     ${acl_action}    ${dest_ntw}   ${src_ntw}    ${dest_port_low}   ${dest_port_up}    ${src_port_low}    ${src_port_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_UDP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl/${acl_name}
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put ACL MACIP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}    ${ingr_intf1}    ${acl_action}    ${src_addr}    ${src_addr_prefix}    ${src_mac_addr}   ${src_mac_addr_mask}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_MACIP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl/${acl_name}
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put ACL ICMP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}   ${egr_intf2}    ${ingr_intf1}   ${ingr_intf2}    ${acl_action}   ${dest_ntw}    ${src_ntw}    ${icmpv6}   ${code_range_low}   ${code_range_up}    ${type_range_low}   ${type_range_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_ICMP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl/${acl_name}
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Get ACL As Json
    [Arguments]           ${node}  ${acl_name}
    ${key}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl/${acl_name}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    #${output}=            Evaluate             json.loads('''${data}''')     json
    #log                   ${output}
    OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply_${acl_name}.json    ${data}
    #[Return]              ${output}
    [Return]              ${data}

vpp_ctl: Get All ACL As Json
    [Arguments]           ${node}
    ${key}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/acl
    ${data}=              etcd: Get ETCD Tree    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    #${output}=            Evaluate             json.loads('''${data}''')     json
    #log                   ${output}
    OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply_acl_all.json    ${data}
    #[Return]              ${output}
    [Return]              ${data}

etcd: Get ETCD Tree
    [Arguments]           ${key}
    ${command}=         Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl get --prefix="true" ${key}
    ${out}=             Execute On Machine    docker    ${command}    log=false
    [Return]            ${out}

vpp_ctl: Delete ACL
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/vpp/config/v1/acl/${name}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put Veth Interface Via Linux Plugin
    [Arguments]    ${node}    ${namespace}    ${name}    ${host_if_name}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Linux Route
    [Arguments]    ${node}    ${namespace}    ${interface}    ${routename}    ${ip}    ${next_hop}    ${prefix}=24    ${metric}=100    ${isdefault}=false
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_static_route.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/route/${routename}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Default Linux Route
    [Arguments]    ${node}    ${namespace}    ${interface}    ${routename}    ${next_hop}    ${metric}=100    ${isdefault}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_default_static_route.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/route/${routename}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Linux Route Without Interface
    [Arguments]    ${node}    ${namespace}    ${routename}    ${ip}    ${next_hop}    ${prefix}=24    ${metric}=100
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_static_route_without_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/route/${routename}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete Linux Route
    [Arguments]    ${node}    ${routename}
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/route/${routename}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Get Linux Route As Json
    [Arguments]    ${node}    ${routename}
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/route/${routename}
    ${data}=              vpp_ctl: Read Key    ${uri}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

vpp_ctl: Check ACL Reply
    [Arguments]         ${node}    ${acl_name}   ${reply_json}    ${reply_term}    ${api_h}=$(API_HANDLER}
    ${acl_d}=           vpp_ctl: Get ACL As Json    ${node}    ${acl_name}
    ${term_d}=          vat_term: Check ACL     ${node}    ${acl_name}
    ${term_d_lines}=    Split To Lines    ${term_d}
    ${data}=            OperatingSystem.Get File    ${reply_json}
    ${data}=            Replace Variables      ${data}
    Should Be Equal     ${data}   ${acl_d}
    ${data}=            OperatingSystem.Get File    ${reply_term}
    ${data}=            Replace Variables      ${data}
    ${t_data_lines}=    Split To Lines    ${data}
    List Should Contain Sub List    ${term_d_lines}    ${t_data_lines}


vpp_ctl: Put ARP
    [Arguments]    ${node}    ${interface}    ${ipv4}    ${MAC}    ${static}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/arp/${interface}/${ipv4}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Get ARP As Json
    [Arguments]           ${node}  ${interface}
    ${key}=               Set Variable          /vnf-agent/${node}/vpp/config/v1/arp/${interface}
    ${data}=              vpp_ctl: Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')     json
    [Return]              ${output}

vpp_ctl: Set L4 Features On Node
    [Arguments]    ${node}    ${enabled}
    [Documentation]    Enable [disable] L4 features by setting ${enabled} to true [false].
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/enable-l4.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/l4/features/feature
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Application Namespace
    [Arguments]    ${node}    ${id}    ${secret}    ${interface}
    [Documentation]    Put application namespace config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/app_namespace.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/l4/namespaces/${id}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}


vpp_ctl: Delete ARP
    [Arguments]    ${node}    ${interface}    ${ipv4}
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/arp/${interface}/${ipv4}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put Linux ARP With Namespace
    [Arguments]    ${node}    ${interface}    ${arpname}    ${ipv4}    ${MAC}    ${nsname}    ${nstype}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp_linux.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/arp/${arpname}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put Linux ARP
    [Arguments]    ${node}    ${interface}    ${arpname}    ${ipv4}    ${MAC}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp_linux.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/arp/${arpname}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete Linux ARP
    [Arguments]    ${node}    ${arpname}
    ${uri}=               Set Variable                  /vnf-agent/${node}/linux/config/v1/arp/${arpname}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put L2XConnect
    [Arguments]    ${node}    ${rx_if}    ${tx_if}
    [Documentation]    Put L2 Xconnect config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/l2xconnect.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/xconnect/${rx_if}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete L2XConnect
    [Arguments]    ${node}    ${rx_if}
    [Documentation]    Delete L2 Xconnect config json from etcd.
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/xconnect/${rx_if}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put TAPv2 Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${host_if_name}    ${prefix}=24    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tapv2_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/interface/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Put STN Rule
    [Arguments]    ${node}    ${interface}    ${ip}    ${rule_name}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/stn_rule.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/stn/rules/${rule_name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete STN Rule
    [Arguments]    ${node}    ${rule_name}
    ${uri}=      Set Variable    /vnf-agent/${node}/vpp/config/v1/stn/rules/${rule_name}
    ${out}=      vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put Local SID
    [Arguments]    ${node}    ${localsidName}    ${sidAddress}    ${fibtable}    ${outinterface}    ${nexthop}
    [Documentation]    Add Local SID config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_local_sid.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/srv6/localsid/${localsidName}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete Local SID
    [Arguments]    ${node}    ${localsidName}
    [Documentation]    Delete Local SID config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/vpp/config/v1/srv6/localsid/${localsidName}
    ${out}=     vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put SRv6 Policy
    [Arguments]    ${node}    ${name}    ${bsid}    ${fibtable}    ${srhEncapsulation}    ${sprayBehaviour}
    [Documentation]    Add SRv6 Policy config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_policy.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/srv6/policy/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete SRv6 Policy
    [Arguments]    ${node}    ${name}
    [Documentation]    Delete SRv6 policy config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/vpp/config/v1/srv6/policy/${name}
    ${out}=     vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put SRv6 Policy Segment
    [Arguments]    ${node}    ${name}    ${policyName}    ${policyBSID}    ${weight}    ${segmentlist}
    [Documentation]    Add SRv6 Policy Segment config json to etcd.
    length should be      ${segmentlist}                3
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_policy_segment.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/srv6/policy/${policyName}/segment/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete SRv6 Policy Segment
    [Arguments]    ${node}    ${name}    ${policyName}
    [Documentation]    Delete SRv6 policy segment config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/vpp/config/v1/srv6/policy/${policyName}/segment/${name}
    ${out}=     vpp_ctl: Delete key    ${uri}
    [Return]    ${out}

vpp_ctl: Put SRv6 Steering
    [Arguments]    ${node}    ${name}    ${bsid}    ${fibtable}    ${prefixAddress}
    [Documentation]    Add SRv6 steering config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_steering.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/vpp/config/v1/srv6/steering/${name}
    ${data}=              Replace Variables             ${data}
    vpp_ctl: Put Json     ${uri}    ${data}

vpp_ctl: Delete SRv6 Steering
    [Arguments]    ${node}    ${name}
    [Documentation]    Delete SRv6 steering config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/vpp/config/v1/srv6/steering/${name}
    ${out}=     vpp_ctl: Delete key    ${uri}
    [Return]    ${out}