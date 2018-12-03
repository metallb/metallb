[Documentation]     Keywords for working with LibMemif Running APP

*** Settings ***
Library      Collections
Library      vpp_term.py

*** Variables ***
${interface_timeout}=     15s
${terminal_timeout}=      30s

*** Keywords ***

lmterm: Open LM Terminal
    [Arguments]    ${node}
    [Documentation]    Attaching to already running Libmemif App on node ${node}
    #lmterm: Issue Command   ${node}_lmterm    ${DOCKER_COMMAND} attach ${node}
    Write To Machine    ${node}_lmterm    ${DOCKER_COMMAND} exec -it ${node} bash -c './.libs/icmpr-epoll'

lmterm: Issue Command
    [Arguments]        ${node}     ${command}
    ${out}=            Write To Machine    ${node}_lmterm    ${command}
#    Should Contain     ${out}             ${${node}_VPP_TERM_PROMPT}
    [Return]           ${out}

lmterm: Exit VPP Terminal
    [Arguments]        ${node}
    ${ctrl_p}          Evaluate    chr(int(16))
    ${ctrl_q}          Evaluate    chr(int(17))
    ${command}=        Set Variable       ${ctrl_p} ${ctrl_q}
    ${out}=            Write To Machine   ${node}_lmterm    ${command}
    [Return]           ${out}

