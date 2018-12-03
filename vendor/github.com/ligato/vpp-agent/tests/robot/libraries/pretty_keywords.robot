*** Keywords ***
Ping From ${node} To ${ip}
    vpp_term: Check Ping    ${node}    ${ip}

Ping6 From ${node} To ${ip}
    vpp_term: Check Ping6    ${node}    ${ip}


Ping On ${node} With IP ${ip}, Source ${source}
    vpp_term: Check Ping Within Interface    ${node}    ${ip}    ${source}

Create Loopback Interface ${name} On ${node} With Ip ${ip}/${prefix} And Mac ${mac}
    vpp_ctl: Put Loopback Interface With IP    ${node}    ${name}   ${mac}   ${ip}   ${prefix}

Create Loopback Interface ${name} On ${node} With Mac ${mac}
    vpp_ctl: Put Loopback Interface    ${node}    ${name}   ${mac}

Create Loopback Interface ${name} On ${node} With VRF ${vrf}, Ip ${ip}/${prefix} And Mac ${mac}
    vpp_ctl: Put Loopback Interface With IP    ${node}    ${name}   ${mac}   ${ip}   ${prefix}    vrf=${vrf}

Create ${type} ${name} On ${node} With MAC ${mac}, Key ${key} And ${sock} Socket
    ${type}=    Set Variable if    "${type}"=="Master"    true    false
    vpp_ctl: put memif interface   ${node}   ${name}   ${mac}   ${type}   ${key}   ${sock}

Create ${type} ${name} On ${node} With IP ${ip}, MAC ${mac}, Key ${key} And ${socket} Socket
    ${type}=   Set Variable if    "${type}"=="Master"    true    false
    ${out}=    vpp_ctl: Put Memif Interface With IP    ${node}    ${name}   ${mac}    ${type}   ${key}    ${ip}    socket=${socket}

Create ${type} ${name} On ${node} With Vrf ${vrf}, IP ${ip}, MAC ${mac}, Key ${key} And ${socket} Socket
    ${type}=   Set Variable if    "${type}"=="Master"    true    false
    ${out}=    vpp_ctl: Put Memif Interface With IP    ${node}    ${name}   ${mac}    ${type}   ${key}    ${ip}    socket=${socket}    vrf=${vrf}

Create Tap Interface ${name} On ${node} With Vrf ${vrf}, IP ${ip}, MAC ${mac} And HostIfName ${host_if_name}
    ${out}=    vpp_ctl: Put TAP Interface With IP    ${node}    ${name}   ${mac}    ${ip}    ${host_if_name}    vrf=${vrf}

Create Bridge Domain ${name} with Autolearn On ${node} With Interfaces ${interfaces}
    @{ints}=    Split String   ${interfaces}    separator=,${space}
    vpp_ctl: put bridge domain    ${node}    ${name}   ${ints}

Create Bridge Domain ${name} Without Autolearn On ${node} With Interfaces ${interfaces}
    @{ints}=    Split String   ${interfaces}    separator=,${space}
    vpp_ctl: put bridge domain    ${node}    ${name}   ${ints}    unicast=false    learn=false

Create Route On ${node} With IP ${ip}/${prefix} With Next Hop ${next_hop} And Vrf Id ${id}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../../robot/resources/static_route.json
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/vpp/config/v1/vrf/${id}/fib/${ip}/${prefix}/${next_hop}
    ${out}=         vpp_ctl: Put Json    ${uri}   ${data}

Create Route On ${node} With IP ${ip}/${prefix} With Next Hop VRF ${next_hop_vrf} From Vrf Id ${id} And Type ${type}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../../robot/resources/route_to_other_vrf.json
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/vpp/config/v1/vrf/${id}/fib/${ip}/${prefix}
    ${out}=         vpp_ctl: Put Json    ${uri}   ${data}

Delete IPsec On ${node} With Prefix ${prefix} And Name ${name}
    vpp_ctl: Delete IPsec    ${node}    ${prefix}    ${name}

Create VXLan ${name} From ${src_ip} To ${dst_ip} With Vni ${vni} On ${node}
    vpp_ctl: Put VXLan Interface    ${node}    ${name}    ${src_ip}    ${dst_ip}    ${vni}

Delete Routes On ${node} And Vrf Id ${id}
    vpp_ctl: Delete routes    ${node}    ${id}

Remove Interface ${name} On ${node}
    vpp_ctl: Delete VPP Interface    ${node}    ${name}

Remove Bridge Domain ${name} On ${node}
    vpp_ctl: Delete Bridge Domain    ${node}    ${name}

Add fib entry for ${mac} in ${name} over ${outgoing} on ${node}
    vpp_ctl: Put Static Fib Entry    ${node}    ${name}    ${mac}    ${outgoing}

Command: ${cmd} should ${expected}
    ${out}=         Run Keyword And Ignore Error    ${cmd}
    Should Be Equal    @{out}[0]    ${expected}    ignore_case=True
    [Return]     ${out}

IP Fib Table ${id} On ${node} Should Be Empty
    ${out}=    vpp_term: Show IP Fib Table    ${node}   ${id}
    Should Be Equal    ${out}   vpp#${SPACE}

IP6 Fib Table ${id} On ${node} Should Be Empty
    ${out}=    vpp_term: Show IP6 Fib Table    ${node}   ${id}
    Should Be Equal    ${out}   vpp#${SPACE}

IP Fib Table ${id} On ${node} Should Not Be Empty
    ${out}=    vpp_term: Show IP Fib Table    ${node}   ${id}
    Should Not Be Equal    ${out}   vpp#${SPACE}

IP Fib Table ${id} On ${node} Should Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP Fib Table    ${node}   ${id}
    Should Match Regexp        ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip4-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip4\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]

IP Fib Table ${id} On ${node} Should Not Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP Fib Table    ${node}   ${id}
    Should Not Match Regexp        ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip4-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip4\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]

IP6 Fib Table ${id} On ${node} Should Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP6 Fib Table    ${node}   ${id}
    Should Match Regexp        ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip6-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip6\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]

IP6 Fib Table ${id} On ${node} Should Not Contain Route With IP ${ip}/${prefix}
    ${out}=    vpp_term: Show IP6 Fib Table    ${node}   ${id}
    Should Not Match Regexp        ${out}  ${ip}\\/${prefix}\\s*unicast\\-ip6-chain\\s*\\[\\@0\\]:\\ dpo-load-balance:\\ \\[proto:ip6\\ index:\\d+\\ buckets:\\d+\\ uRPF:\\d+\\ to:\\[0:0\\]\\]


Show IP Fib On ${node}
    ${out}=     vpp_term: Show IP Fib    ${node}

Show IP6 Fib On ${node}
    ${out}=     vpp_term: Show IP6 Fib    ${node}

Show Interfaces On ${node}
    ${out}=   vpp_term: Show Interfaces    ${node}

Show Interfaces Address On ${node}
    ${out}=   vpp_term: Show Interfaces Address    ${node}

Create Linux Route On ${node} With IP ${ip}/${prefix} With Next Hop ${next_hop} And Vrf Id ${id}
    ${data}=        OperatingSystem.Get File    ${CURDIR}/../../robot/resources/linux_static_route.json
    ${data}=        replace variables           ${data}
    ${uri}=         Set Variable                /vnf-agent/${node}/vpp/config/v1/vrf/${id}/fib/${ip}/${prefix}/${next_hop}
    ${out}=         vpp_ctl: Put Json    ${uri}   ${data}
