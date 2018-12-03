[Documentation] Keywords for ssh sessions

*** Settings ***
#Library       String
#Library       RequestsLibrary
Library       SSHLibrary            timeout=15 seconds       loglevel=TRACE

*** Keywords ***
Execute On Machine     [Arguments]              ${machine}               ${command}               ${log}=true
                       [Documentation]          *Execute On Machine ${machine} ${command}*
                       ...                      Executing ${command} on connection with name ${machine}
                       ...                      Output log is added to machine output log
                       Switch Connection        ${machine}
                       ${currdate}=             Get Current Date
                       Append To File    ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Append To File    ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       ${out}   ${stderr}=      Execute Command          ${command}    return_stderr=True
                       ${status}=               Run Keyword And Return Status    Should Be Empty    ${stderr}
                       Run Keyword If           ${status}==False         Log     One or more error occured during execution of a command ${command} on ${machine}    level=WARN
                       Run Keyword If           '${log}'=='true'         Append To File    ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Response: ${out}${\n}
                       Run Keyword If           '${log}'=='true'         Append To File    ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Response: ${out}${\n}
                       Run Keyword If           ${status}==False         Append To File    ${RESULTS_FOLDER}/output_${machine}.log    *** Error: ${stderr}${\n}
                       Run Keyword If           ${status}==False         Append To File    ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Error: ${stderr}${\n}
                       [Return]                 ${out}

Write To Machine       [Arguments]              ${machine}               ${command}               ${delay}=${SSH_READ_DELAY}s
                       [Documentation]          *Write Machine ${machine} ${command}*
                       ...                      Writing ${command} to connection with name ${machine}
                       ...                      Output log is added to machine output log
                       Switch Connection        ${machine}
                       ${currdate}=             Get Current Date
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Write                    ${command}      loglevel=TRACE
                       ${out}=                  Read            loglevel=TRACE         delay=${delay}
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Response: ${out}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Response: ${out}${\n}
                       [Return]                 ${out}

Write To Machine Until Prompt
                       [Arguments]              ${machine}    ${command}    ${prompt}=root@    ${delay}=${SSH_READ_DELAY}
                       [Documentation]          *Write Machine ${machine} ${command}*
                       ...                      Writing ${command} to connection with name ${machine} and reading until prompt
                       ...                      Output log is added to machine output log
                       Log                      Use 'Write To Container Until Prompt' instead of this kw    level=WARN
                       Switch Connection        ${machine}
                       ${currdate}=             Get Current Date
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Write                    ${command}       loglevel=TRACE
                       ${out}=                  Read Until               ${prompt}${${machine}_HOSTNAME}     loglevel=TRACE
                       ${out2}=                 Read             loglevel=TRACE       delay=${delay}
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
                       [Return]                 ${out}${out2}

Write To Machine Until String
                       [Arguments]              ${machine}    ${command}    ${string}    ${delay}=${SSH_READ_DELAY}
                       [Documentation]          *Write Machine ${machine} ${command}*
                       ...                      Writing ${command} to connection with name ${machine} and reading until specified string
                       ...                      Output log is added to machine output log
                       Switch Connection        ${machine}
                       ${currdate}=             Get Current Date
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Command: ${command}${\n}
                       Write                    ${command}       loglevel=TRACE
                       ${out}=                  Read Until       ${string}       loglevel=TRACE
                       ${out2}=                 Read             loglevel=TRACE        delay=${delay}
                       Append To File           ${RESULTS_FOLDER}/output_${machine}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
                       Append To File           ${RESULTS_FOLDER_SUITE}/output_${machine}.log    *** Time:${currdate} Response: ${out}${out2}${\n}
                       [Return]                 ${out}${out2}

