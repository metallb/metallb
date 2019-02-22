[Documentation]     Keywords for working with VPP Ctl container

*** Settings ***
Library        etcdctl.py
Library        String

*** Variables ***

*** Keywords ***

Put Json
    [Arguments]        ${key}    ${json}
    ${command}=         Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl put ${key} '${json}'
    ${out}=             Execute On Machine    docker    ${command}    log=false
    [Return]           ${out}

Read Key
    [Arguments]        ${key}       ${prefix}=false
    ${command}=        Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl get ${key} --write-out="simple" --prefix="${prefix}"
    ${out}=            Execute On Machine    docker    ${command}    log=false
    ${length}=         Get Length      ${out}
    #${out}=            Remove Empty Lines     ${out}
    #@{ret}=            Run Keyword Unless    ${length} == 0    Split String     ${out}    {    1
    ${ret}=            Run Keyword Unless    ${length} == 0    Split To Lines     ${out}
    #${length}=         Get Length      ${ret}
    #${out}=            Run Keyword Unless  ${length}== 0    Set Variable      \{@{ret}[1]
    #${out0}=            Run Keyword Unless  ${length}== 0    set Variable      \{@{ret}[0]
    ${out0}=            Run Keyword Unless  ${length}== 0    Remove Keys     ${ret}
    [Return]           ${out0}

Delete Key
    [Arguments]        ${key}
    ${command}=         Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl del ${key}
    ${out}=             Execute On Machine    docker    ${command}    log=false
    [Return]           ${out}

#*****
Put Memif Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${master}    ${id}    ${socket}=memif.sock    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${socket}=            Set Variable                  ${${node}_MEMIF_SOCKET_FOLDER}/${socket}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/memif_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Memif Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${master}    ${id}    ${ip}    ${prefix}=24    ${socket}=memif.sock    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${socket}=            Set Variable                  ${${node}_MEMIF_SOCKET_FOLDER}/${socket}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/memif_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Veth Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${peer}    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Veth Interface And Namespace
    [Arguments]    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Veth Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Veth Interface With IP And Namespace
    [Arguments]    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    Log Many    ${node}    ${name}    ${namespace}    ${mac}    ${peer}    ${ip}    ${prefix}    ${mtu}    ${enabled}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/veth_interface_with_ip_and_ns.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    Log Many              ${data}                       ${uri}
    ${data}=              Replace Variables             ${data}
    Log                   ${data}
    Put Json     ${uri}    ${data}

Put Afpacket Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${host_int}    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/afpacket_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put VXLan Interface
    [Arguments]    ${node}    ${name}    ${src}    ${dst}    ${vni}    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/vxlan_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Bridge Domain
    [Arguments]    ${node}    ${name}    ${ints}    ${flood}=true    ${unicast}=true    ${forward}=true    ${learn}=true    ${arp_term}=true
    ${interfaces}=        Create Interfaces Json From List    ${ints}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bridge_domain.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/l2/${AGENT_VER}/bridge-domain/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Loopback Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${mtu}=1500    ${enabled}=true   ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/loopback_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Loopback Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${prefix}=24    ${mtu}=1500    ${vrf}=0    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/loopback_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Physical Interface With IP
    [Arguments]    ${node}    ${name}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/physical_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Get VPP Interface State
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/status/${AGENT_VER}/interface/${interface}
    ${out}=               Read Key    ${key}
    [Return]              ${out}

Get VPP Interface State As Json
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/vpp/status/${AGENT_VER}/interface/${interface}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get VPP Interface Config As Json
    [Arguments]    ${node}    ${interface}
    ${key}=               Set Variable    /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${interface}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get VPP NAT44 Config As Json
    [Arguments]    ${node}    ${name}=None
    ${key}=               Set Variable    /vnf-agent/${node}/config/vpp/nat/${AGENT_VER}/dnat44/${name}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get VPP NAT44 Global Config As Json
    [Arguments]    ${node}
    ${key}=               Set Variable    /vnf-agent/${node}/config/vpp/nat/${AGENT_VER}/nat44-global
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get Linux Interface Config As Json
    [Arguments]    ${node}    ${name}
    ${key}=               Set Variable    /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get Interface Internal Name
    [Arguments]    ${node}    ${interface}
    ${name}=    Set Variable      ${EMPTY}
    ${empty_dict}=   Create Dictionary
    ${state}=    Get VPP Interface State As Json    ${node}    ${interface}
    ${length}=   Get Length     ${state}
    ${name}=    Run Keyword If      ${length} != 0     Set Variable    ${state["internal_name"]}
    [Return]    ${name}

Get Interface Sw If Index
    [Arguments]    ${node}    ${interface}
    ${state}=    Get VPP Interface State As Json    ${node}    ${interface}
    ${sw_if_index}=    Set Variable    ${state["if_index"]}
    [Return]    ${sw_if_index}

Get Bridge Domain ID
    [Arguments]    ${node}    ${bd_name}
    ${bds_dump}=    Execute On Machine    docker    curl -X GET http://localhost:9191/dump/vpp/v2/bd
    ${bds_json}=    Evaluate    json.loads('''${bds_dump}''')    json
    ${index}=   Set Variable    0
    :FOR    ${bd}   IN  @{bds_json}
    \   ${data}=    Set Variable    ${bd['bridge_domain']}
    \   ${meta}=    Set Variable    ${bd['bridge_domain_meta']}
    \   ${index}=   Run Keyword If  "${data["name"]}" == "${bd_name}"     Set Variable  ${meta['bridge_domain_id']}
    [Return]   ${index}

Get Bridge Domain ID IPv6
    [Arguments]    ${node}    ${bd_name}
    ${bds_dump}=    Execute On Machine    docker    curl --noproxy "::" -g -6 -X GET http://[::]:9191/dump/vpp/v2/bd
    ${bds_json}=    Evaluate    json.loads('''${bds_dump}''')    json
    ${index}=   Set Variable    0
    :FOR    ${bd}   IN  @{bds_json}
    \   ${data}=    Set Variable    ${bd['bridge_domain']}
    \   ${meta}=    Set Variable    ${bd['bridge_domain_meta']}
    \   ${index}=   Run Keyword If  "${data["name"]}" == "${bd_name}"     Set Variable  ${meta['bridge_domain_id']}
    [Return]   ${index}

Put TAP Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${host_if_name}    ${prefix}=24    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tap_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put TAP Unnumbered Interface
    [Arguments]    ${node}    ${name}    ${mac}    ${unnumbered}    ${interface_with_ip_name}    ${host_if_name}    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tap_interface_unnumbered.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}


Put Static Fib Entry
    [Arguments]    ${node}    ${bd_name}    ${mac}    ${outgoing_interface}    ${static}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/static_fib.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/l2/${AGENT_VER}/bridge-domain/${name}/fib/${mac}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Create IPsec With SA And Json
    [Arguments]     ${node}    ${interface}    ${file_name}    ${sa_index}    ${spi}    ${crypto_key}    ${integ_key}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../resources/${file_name}
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/config/vpp/ipsec/${AGENT_VER}/sa/${sa_index}
    ${out}=         Put Json    ${uri}   ${data}

Create IPsec With SPD And Json
    [Arguments]     ${node}    ${spd_index}    ${file_name}    ${interface_name}    ${remote_addr}    ${local_addr}    ${sa_index_1}  ${sa_index_2}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../resources/${file_name}
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/config/vpp/ipsec/${AGENT_VER}/spd/${spd_index}
    ${out}=         Put Json    ${uri}   ${data}

Delete Bridge Domain
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/config/vpp/l2/${AGENT_VER}/bridge-domain/${name}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Delete VPP Interface
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Delete Linux Interface
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Delete Route
    [Arguments]    ${node}    ${id}    ${ip}    ${prefix}
    ${uri}=    Set Variable                /vnf-agent/${node}/config/vpp/${AGENT_VER}/route/vrf/${id}/dst/${ip}/${prefix}/gw/
    ${out}=         Delete key  ${uri}
    [Return]       ${out}

Delete Routes
    [Arguments]    ${node}    ${id}
    ${uri}=    Set Variable                /vnf-agent/${node}/config/vpp/${AGENT_VER}/route/vrf/${id}/dst
    ${command}=         Set Variable    ${DOCKER_COMMAND} exec etcd etcdctl del --prefix="true" ${uri}
    ${out}=             Execute On Machine    docker    ${command}    log=false
    [Return]       ${out}

Delete IPsec
    [Arguments]    ${node}    ${prefix}    ${name}
    ${uri}=    Set Variable                /vnf-agent/${node}/config/vpp/ipsec/${AGENT_VER}/${prefix}/${name}
    ${out}=         Delete key  ${uri}
    [Return]       ${out}

Delete Dnat
    [Arguments]    ${node}    ${name}
    ${uri}=    Set Variable                /vnf-agent/${node}/config/vpp/nat/${AGENT_VER}/dnat44/${name}
    ${out}=         Delete key  ${uri}
    [Return]       ${out}

Delete Nat Global
    [Arguments]    ${node}
    ${uri}=    Set Variable                /vnf-agent/${node}/config/vpp/nat/${AGENT_VER}/nat44-global
    ${out}=         Delete key  ${uri}
    [Return]       ${out}

Put BFD Session
    [Arguments]    ${node}    ${session_name}    ${min_tx_interval}    ${dest_adr}    ${detect_multiplier}    ${interface}    ${min_rx_interval}    ${source_adr}   ${enabled}    ${auth_key_id}=0    ${BFD_auth_key_id}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_session.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/session/${session_name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put BFD Authentication Key
    [Arguments]    ${node}    ${key_name}    ${auth_type}    ${id}    ${secret}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_key.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/auth-key/${key_name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put BFD Echo Function
    [Arguments]    ${node}    ${echo_func_name}    ${source_intf}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/bfd_echo_function.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/echo-function
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Get BFD Session As Json
    [Arguments]    ${node}    ${session_name}
    ${key}=               Set Variable            /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/session/${session_name}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get BFD Authentication Key As Json
    [Arguments]    ${node}    ${key_name}
    ${key}=               Set Variable          /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/auth-key/${key_name}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Get BFD Echo Function As Json
    [Arguments]    ${node}
    ${key}=               Set Variable          /vnf-agent/${node}/config/vpp/${AGENT_VER}/bfd/echo-function
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Put ACL TCP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}   ${ingr_intf1}    ${acl_action}    ${dest_ntw}    ${src_ntw}    ${dest_port_low}   ${dest_port_up}    ${src_port_low}    ${src_port_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_TCP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${acl_name}/
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    Put Json     ${uri}    ${data}

Put ACL UDP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}    ${ingr_intf1}     ${egr_intf2}    ${ingr_intf2}     ${acl_action}    ${dest_ntw}   ${src_ntw}    ${dest_port_low}   ${dest_port_up}    ${src_port_low}    ${src_port_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_UDP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${acl_name}/
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    Put Json     ${uri}    ${data}

Put ACL MACIP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}    ${ingr_intf1}    ${acl_action}    ${src_addr}    ${src_addr_prefix}    ${src_mac_addr}   ${src_mac_addr_mask}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_MACIP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${acl_name}/
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    Put Json     ${uri}    ${data}

Put ACL ICMP
    [Arguments]    ${node}    ${acl_name}    ${egr_intf1}   ${egr_intf2}    ${ingr_intf1}   ${ingr_intf2}    ${acl_action}   ${dest_ntw}    ${src_ntw}    ${icmpv6}   ${code_range_low}   ${code_range_up}    ${type_range_low}   ${type_range_up}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/acl_ICMP.json
    ${uri}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${acl_name}/
    ${data}=              Replace Variables             ${data}
    #OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply.json     ${data}
    Put Json     ${uri}    ${data}

Get ACL As Json
    [Arguments]           ${node}  ${acl_name}
    ${key}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${acl_name}/
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''=="" or '''${data}'''=='None'    {}    ${data}
    #${output}=            Evaluate             json.loads('''${data}''')     json
    #log                   ${output}
    OperatingSystem.Create File   ${REPLY_DATA_FOLDER}/reply_${acl_name}.json    ${data}
    #[Return]              ${output}
    [Return]              ${data}

Get All ACL As Json
    [Arguments]           ${node}
    ${key}=               Set Variable          /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl
    #${data}=              etcd: Get ETCD Tree    ${key}
    ${data}=              Read Key    ${key}    true
    ${data}=              Set Variable If      '''${data}'''=="" or '''${data}'''=='None'    {}    ${data}
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

Delete ACL
    [Arguments]    ${node}    ${name}
    ${uri}=      Set Variable    /vnf-agent/${node}/config/vpp/acls/${AGENT_VER}/acl/${name}/
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Put Veth Interface Via Linux Plugin
    [Arguments]    ${node}    ${namespace}    ${name}    ${host_if_name}    ${mac}    ${peer}    ${ip}    ${prefix}=24    ${mtu}=1500    ${enabled}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_veth_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/interfaces/${AGENT_VER}/interface/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Linux Route
    [Arguments]    ${node}    ${namespace}    ${interface}    ${routename}    ${ip}    ${next_hop}    ${prefix}=24    ${metric}=100    ${isdefault}=false
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_static_route.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/route/${routename}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Default Linux Route
    [Arguments]    ${node}    ${namespace}    ${interface}    ${routename}    ${next_hop}    ${metric}=100    ${isdefault}=true
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_default_static_route.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/route/${routename}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Linux Route Without Interface
    [Arguments]    ${node}    ${namespace}    ${routename}    ${ip}    ${next_hop}    ${prefix}=24    ${metric}=100
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/linux_static_route_without_interface.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/route/${routename}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete Linux Route
    [Arguments]    ${node}    ${routename}
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/route/${routename}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Get Linux Route As Json
    [Arguments]    ${node}    ${routename}
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/route/${routename}
    ${data}=              Read Key    ${uri}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')    json
    [Return]              ${output}

Check ACL Reply
    [Arguments]         ${node}    ${acl_name}   ${reply_json}    ${reply_term}    ${api_h}=$(API_HANDLER}
    ${acl_d}=           Get ACL As Json    ${node}    ${acl_name}
    ${term_d}=          vat_term: Check ACL     ${node}    ${acl_name}
    ${term_d_lines}=    Split To Lines    ${term_d}
    ${data}=            OperatingSystem.Get File    ${reply_json}
    ${data}=            Replace Variables      ${data}
    Should Be Equal     ${data}   ${acl_d}
    ${data}=            OperatingSystem.Get File    ${reply_term}
    ${data}=            Replace Variables      ${data}
    ${t_data_lines}=    Split To Lines    ${data}
    List Should Contain Sub List    ${term_d_lines}    ${t_data_lines}


Put ARP
    [Arguments]    ${node}    ${interface}    ${ipv4}    ${MAC}    ${static}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/arp/${interface}/${ipv4}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Get ARP As Json
    [Arguments]           ${node}  ${interface}
    ${key}=               Set Variable          /vnf-agent/${node}/config/vpp/${AGENT_VER}/arp/${interface}
    ${data}=              Read Key    ${key}
    ${data}=              Set Variable If      '''${data}'''==""    {}    ${data}
    ${output}=            Evaluate             json.loads('''${data}''')     json
    [Return]              ${output}

Set L4 Features On Node
    [Arguments]    ${node}    ${enabled}
    [Documentation]    Enable [disable] L4 features by setting ${enabled} to true [false].
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/enable-l4.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/l4/features/feature
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Application Namespace
    [Arguments]    ${node}    ${id}    ${secret}    ${interface}
    [Documentation]    Put application namespace config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/app_namespace.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/l4/namespaces/${id}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}


Delete ARP
    [Arguments]    ${node}    ${interface}    ${ipv4}
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/arp/${interface}/${ipv4}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Put Linux ARP With Namespace
    [Arguments]    ${node}    ${interface}    ${arpname}    ${ipv4}    ${MAC}    ${nsname}    ${nstype}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp_linux.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/arp/${arpname}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put Linux ARP
    [Arguments]    ${node}    ${interface}    ${arpname}    ${ipv4}    ${MAC}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/arp_linux.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/arp/${arpname}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete Linux ARP
    [Arguments]    ${node}    ${arpname}
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/linux/l3/${AGENT_VER}/arp/${arpname}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Put L2XConnect
    [Arguments]    ${node}    ${rx_if}    ${tx_if}
    [Documentation]    Put L2 Xconnect config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/l2xconnect.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/l2/${AGENT_VER}/xconnect/${rx_if}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete L2XConnect
    [Arguments]    ${node}    ${rx_if}
    [Documentation]    Delete L2 Xconnect config json from etcd.
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/l2/${AGENT_VER}/xconnect/${rx_if}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Put TAPv2 Interface With IP
    [Arguments]    ${node}    ${name}    ${mac}    ${ip}    ${host_if_name}    ${prefix}=24    ${mtu}=1500    ${enabled}=true    ${vrf}=0
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/tapv2_interface_with_ip.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/interfaces/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Put STN Rule
    [Arguments]    ${node}    ${interface}    ${ip}
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/stn_rule.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/stn/${AGENT_VER}/rule/${interface}/ip/${ip}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete STN Rule
    [Arguments]    ${node}    ${interface}    ${ip}
    ${uri}=      Set Variable    /vnf-agent/${node}/config/vpp/stn/${AGENT_VER}/rule/${interface}/ip/${ip}
    ${out}=      Delete key    ${uri}
    [Return]    ${out}

Put Local SID
    [Arguments]    ${node}    ${localsidName}    ${sidAddress}    ${fibtable}    ${outinterface}    ${nexthop}
    [Documentation]    Add Local SID config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_local_sid.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/localsid/${localsidName}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete Local SID
    [Arguments]    ${node}    ${localsidName}
    [Documentation]    Delete Local SID config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/localsid/${localsidName}
    ${out}=     Delete key    ${uri}
    [Return]    ${out}

Put SRv6 Policy
    [Arguments]    ${node}    ${name}    ${bsid}    ${fibtable}    ${srhEncapsulation}    ${sprayBehaviour}
    [Documentation]    Add SRv6 Policy config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_policy.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/policy/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete SRv6 Policy
    [Arguments]    ${node}    ${name}
    [Documentation]    Delete SRv6 policy config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/policy/${name}
    ${out}=     Delete key    ${uri}
    [Return]    ${out}

Put SRv6 Policy Segment
    [Arguments]    ${node}    ${name}    ${policyName}    ${policyBSID}    ${weight}    ${segmentlist}
    [Documentation]    Add SRv6 Policy Segment config json to etcd.
    length should be      ${segmentlist}                3
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_policy_segment.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/policy/${policyName}/segment/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete SRv6 Policy Segment
    [Arguments]    ${node}    ${name}    ${policyName}
    [Documentation]    Delete SRv6 policy segment config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/policy/${policyName}/segment/${name}
    ${out}=     Delete key    ${uri}
    [Return]    ${out}

Put SRv6 Steering
    [Arguments]    ${node}    ${name}    ${bsid}    ${fibtable}    ${prefixAddress}
    [Documentation]    Add SRv6 steering config json to etcd.
    ${data}=              OperatingSystem.Get File      ${CURDIR}/../resources/srv6_steering.json
    ${uri}=               Set Variable                  /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/steering/${name}
    ${data}=              Replace Variables             ${data}
    Put Json     ${uri}    ${data}

Delete SRv6 Steering
    [Arguments]    ${node}    ${name}
    [Documentation]    Delete SRv6 steering config json from etcd.
    ${uri}=     Set Variable           /vnf-agent/${node}/config/vpp/${AGENT_VER}/srv6/steering/${name}
    ${out}=     Delete key    ${uri}
    [Return]    ${out}