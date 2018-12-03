[Documentation]     Restconf api specific configurations

*** Settings ***
Library        rest_api.py

*** Keywords ***

rest_api: Get
    [Arguments]      ${node}    ${uri}    ${expected_code}=200
    ${response}=      Get Request          ${node}    ${uri}
#    ${pretty}=        Ordered Json         ${response.text}
    Sleep             ${REST_CALL_SLEEP}
    Run Keyword If    '${expected_code}'!='0'       Should Be Equal As Integers    ${response.status_code}    ${expected_code}
    [Return]         ${response.text}


rest_api: Put
    [Arguments]      ${node}    ${uri}    ${expected_code}=200
    ${response}=      Put Request          ${node}    ${uri}
    ${pretty}=        Ordered Json         ${response.text}
    Sleep             ${REST_CALL_SLEEP}
    Run Keyword If    '${expected_code}'!='0'       Should Be Equal As Integers    ${response.status_code}    ${expected_code}
    [Return]         ${response.text}

rest_api: Get Loggers List
    [Arguments]      ${node}
    ${uri}=           Set Variable     log/list
    ${out}=           rest_api: Get    ${node}    ${uri}
    [Return]         ${out}

rest_api: Change Logger Level
    [Arguments]     ${node}    ${logger}    ${log_level}
    ${uri}=          Set variable      /log/${logger}/${log_level}
    ${out}=          rest_api: Put     ${node}    ${uri}
    [Return]        ${out}
