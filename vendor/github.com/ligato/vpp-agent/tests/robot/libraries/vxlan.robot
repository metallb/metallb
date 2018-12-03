[Documentation]     Keywords for checking VXLans

*** Settings ***

*** Variables ***
${tunnel_timeout}=     15s

*** Keywords ***
vxlan: Tunnel Is Created
    [Arguments]    ${node}    ${src}    ${dst}    ${vni}
    ${int_index}=  Wait Until Keyword Succeeds    ${tunnel_timeout}   3s    vat_term: Check VXLan Tunnel Presence    ${node}    ${src}    ${dst}    ${vni}
    [Return]       ${int_index}

vxlan: Tunnel Is Deleted
    [Arguments]    ${node}    ${src}    ${dst}    ${vni}
    Wait Until Keyword Succeeds    ${tunnel_timeout}   3s    vat_term: Check VXLan Tunnel Presence    ${node}    ${src}    ${dst}    ${vni}    ${FALSE}

vxlan: Tunnel Exists
    [Arguments]    ${node}    ${src}    ${dst}    ${vni}
    ${int_index}=  vat_term: Check VXLan Tunnel Presence    ${node}    ${src}    ${dst}    ${vni}
    [Return]       ${int_index}

vxlan: Tunnel Not Exists
    [Arguments]    ${node}    ${src}    ${dst}    ${vni}
    vat_term: Check VXLan Tunnel Presence    ${node}    ${src}    ${dst}    ${vni}    ${FALSE}

