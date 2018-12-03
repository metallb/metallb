*** Keywords ***
Open VPP terminal
    [Arguments]    ${node}
    ${out}=    Write To Machine    ${node}    telnet localhost 5002
    Should Contain     ${out}     vpp#

Close VPP terminal
    [Arguments]    ${node}
    ${ctrl_c}          Evaluate    chr(int(3))
    ${command}=        Set Variable       ${ctrl_c}
    ${out}=            Write To Machine   ${node}    ${command}
    [Return]           ${out}

# TODO: this will not work in case there is physical int
Start VPP
    [Arguments]    ${node}
    Write To Container Until Prompt     ${node}     vpp unix { cli-listen localhost:5002 } plugins { plugin dpdk_plugin.so { disable } }
    ${out}=     Write To Container Until Prompt    ${node}     ps aux | grep vpp
    Should Contain        ${out}      unix { cli-listen localhost:5002 } plugins { plugin dpdk_plugin.so { disable } }

Stop VPP
    [Arguments]    ${node}
    Write To Machine    ${node}     pgrep -f vpp | xargs kill -15

# TODO: refactoring needed, this will block docker connection with telnet
Wait until VPP successful load
    [Arguments]    ${node}    ${retries}=5x    ${delay}=1s
    wait until keyword succeeds    ${retries}   ${delay}   Try connect to VPP terminal    ${node}


Try connect to VPP terminal
    [Arguments]    ${node}
    ${out}=    Write To Machine    docker    telnet localhost ${${node}_VPP_HOST_PORT}
    Should Contain    ${out}    vpp#

Execute In VPP
    [Arguments]              ${container}       ${command}
    Switch Connection        docker
    ${out}   ${stderr}=      Execute Command    ${DOCKER_COMMAND} exec -it ${container} vppctl ${command}    return_stderr=True
    ${currdate}=             Get Current Date
    ${status}=               Run Keyword And Return Status    Should be Empty    ${stderr}
    Run Keyword If           ${status}==False         Log     One or more error occured during execution of a command ${command} in container ${container}    level=WARN
    Append To File           ${RESULTS_FOLDER}/output_${container}_term.log    *** Time:${currdate} Command: ${command}${\n}${out}${\n}
    Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}_term.log    *** Time:${currdate} Command: ${command}${\n}${out}${\n}
    Run Keyword If           ${status}==False      Append To File           ${RESULTS_FOLDER}/output_${container}_term.log      *** Error: ${stderr}${\n}
    Run Keyword If           ${status}==False      Append To File           ${RESULTS_FOLDER_SUITE}/output_${container}_term.log      *** Error: ${stderr}${\n}
    [Return]                 ${out}

