*** Settings ***
Library        linux.py

*** Variables ***
${interface_timeout}=     15s
${PINGSERVER_UDP}=        nc -uklp
${PINGSERVER_TCP}=        nc -klp
${UDPPING}=               nc -uzv
${TCPPING}=               nc -zv


*** Keywords ***
linux: Get Linux Interfaces
    [Arguments]        ${node}
    ${out}=    Execute In Container    ${node}    ip a
    ${ints}=    Parse Linux Interfaces    ${out}
    [Return]    ${ints}

linux: Check Veth Interface State
    [Arguments]          ${node}    ${name}    @{desired_state}
    ${veth_config}=      Get Linux Interface Config As Json    ${node}    ${name}
    ${peer}=             Set Variable    ${veth_config["veth"]["peer_if_name"]}
    ${ints}=             linux: Get Linux Interfaces    ${node}
    ${actual_state}=     Pick Linux Interface    ${ints}    ${name}\@${peer}
    List Should Contain Sub List    ${actual_state}    ${desired_state}
    [Return]             ${actual_state}

linux: Check Interface Presence
    [Arguments]        ${node}     ${mac}    ${status}=${TRUE}
    [Documentation]    Checking if specified interface with mac exists in linux
    ${ints}=           linux: Get Linux Interfaces    ${node}
    ${result}=         Check Linux Interface Presence    ${ints}    ${mac}
    Should Be Equal    ${result}    ${status}

linux: Check Interface With IP Presence
    [Arguments]        ${node}     ${mac}    ${ip}      ${status}=${TRUE}
    [Documentation]    Checking if specified interface with mac and ip exists in linux
    ${ints}=           linux: Get Linux Interfaces    ${node}
    ${result}=         Check Linux Interface IP Presence    ${ints}    ${mac}   ${ip}
    Should Be Equal    ${result}    ${status}

linux: Interface Is Created
    [Arguments]    ${node}    ${mac}                    
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    linux: Check Interface Presence    ${node}    ${mac}

linux: Interface With IP Is Created
    [Arguments]    ${node}    ${mac}    ${ipv4}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    linux: Check Interface With IP Presence    ${node}    ${mac}    ${ipv4}

linux: Interface Is Deleted
    [Arguments]    ${node}    ${mac}                    
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    linux: Check Interface Presence    ${node}    ${mac}    ${FALSE}

linux: Interface With IP Is Deleted
    [Arguments]    ${node}    ${mac}   ${ipv4}
    Wait Until Keyword Succeeds    ${interface_timeout}   3s    linux: Check Interface With IP Presence    ${node}    ${mac}    ${ipv4}   ${FALSE}

linux: Interface Exists
    [Arguments]    ${node}    ${mac}
    linux: Check Interface Presence    ${node}    ${mac}

linux: Interface Not Exists
    [Arguments]    ${node}    ${mac}
    linux: Check Interface Presence    ${node}    ${mac}    ${FALSE}

linux: Check Ping
    [Arguments]        ${node}    ${ip}
    ${out}=            Execute In Container    ${node}    ping -c 5 ${ip}
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

linux: Check Ping6
    [Arguments]        ${node}    ${ip}
    ${out}=            Execute In Container    ${node}    ping6 -c 5 ${ip}
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

linux: Run TCP Ping Server On Node
    [Arguments]    ${node}   ${port}
    [Documentation]    Run TCP PingServer as listener on node ${node}
    ${out}=            Execute In Container Background    ${node}    ${PINGSERVER_TCP} ${port}

linux: Run UDP Ping Server On Node
    [Arguments]    ${node}   ${port}
    [Documentation]    Run UDP PingServer as listener on node ${node}
    ${out}=            Execute In Container Background    ${node}    ${PINGSERVER_UDP} ${port}

linux: TCPPing
    [Arguments]        ${node}    ${ip}     ${port}
    #${out}=            Execute In Container    ${node}    ${TCPPING} ${ip} ${port}
    #${out}=            Write To Container Until Prompt   ${node}     ${TCPPING} ${ip} ${port}
    ${out}=            Write Command to Container   ${node}     ${TCPPING} ${ip} ${port}
    Should Contain     ${out}    Connection to ${ip} ${port} port [tcp/*] succeeded!
    Should Not Contain    ${out}    Connection refused

linux: TCPPingNot
    [Arguments]        ${node}    ${ip}     ${port}
    #${out}=            Execute In Container    ${node}    ${TCPPING} ${ip} ${port}
    #${out}=            Write To Container Until Prompt   ${node}     ${TCPPING} ${ip} ${port}
    ${out}=            Write Command to Container   ${node}     ${TCPPING} ${ip} ${port}
    Should Not Contain     ${out}    Connection to ${ip} ${port} port [tcp/*] succeeded!
    Should Contain    ${out}    Connection refused

linux: UDPPing
    [Arguments]        ${node}    ${ip}     ${port}
    #${out}=            Execute In Container    ${node}    ${UDPPING} ${ip} ${port}
    #${out}=            Write To Container Until Prompt    ${node}    ${UDPPING} ${ip} ${port}
    ${out}=            Write Command to Container    ${node}    ${UDPPING} ${ip} ${port}
    Should Contain     ${out}    Connection to ${ip} ${port} port [udp/*] succeeded!
    Should Not Contain    ${out}    Connection refused

linux: UDPPingNot
    [Arguments]        ${node}    ${ip}     ${port}
    #${out}=            Execute In Container    ${node}    ${UDPPING} ${ip} ${port}
    #${out}=            Write To Container Until Prompt    ${node}    ${UDPPING} ${ip} ${port}
    ${out}=            Write Command to Container    ${node}    ${UDPPING} ${ip} ${port}
    Should Not Contain     ${out}    Connection to ${ip} ${port} port [udp/*] succeeded!
    Should Contain    ${out}    Connection refused

linux: Check Processes on Node
    [Arguments]        ${node}
    ${out}=            Execute In Container    ${node}    ps aux

linux: Set Host TAP Interface
    [Arguments]    ${node}    ${host_if_name}    ${ip}    ${prefix}
    ${out}=    Execute In Container    ${node}    ip link set dev ${host_if_name} up
    ${out}=    Execute In Container    ${node}    ip addr add ${ip}/${prefix} dev ${host_if_name}

linux: Add Route
    [Arguments]    ${node}    ${destination_ip}    ${prefix}    ${next_hop_ip}
    Execute In Container    ${node}    ip route add ${destination_ip}/${prefix} via ${next_hop_ip}

linux: Delete Route
    [Arguments]    ${node}    ${destination_ip}    ${prefix}    ${next_hop_ip}
    Execute In Container    ${node}    ip route del ${destination_ip}/${prefix} via ${next_hop_ip}

