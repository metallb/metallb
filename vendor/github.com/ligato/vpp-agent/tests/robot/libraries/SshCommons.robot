*** Settings ***
Documentation     This is a library for simple improvements over SSHLibrary for other robot libraries to use.
Resource          ${CURDIR}/all_libs.robot

*** Keywords ***
Open_Ssh_Connection_Kube
    [Arguments]    ${name}    ${ip}    ${user}    ${pswd}
    [Documentation]    Create SSH connection to \{ip} aliased as \${name} and log in using \${user} and \${pswd} (or rsa).
    ...    Log to output file. The new connection is left active.
    BuiltIn.Log_Many    ${name}    ${ip}    ${user}    ${pswd}
    ${time} =    DateTime.Get_Current_Date
    ${connection}=    Open Connection    ${ip}    alias=${name}    timeout=${SSH_TIMEOUT}
    #${out} =    BuiltIn.Run_Keyword_If    """${pswd}""" != "rsa_id"    SSHLibrary.Login    ${user}    ${pswd}
    #${out2} =    BuiltIn.Run_Keyword_If    """${pswd}""" == "rsa_id"    SSHLibrary.Login_With_Public_Key    ${user}    %{HOME}/.ssh/id_rsa    any
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${name}.log    ${time}${\n}*** Command: Login${\n}
    ${out} =    BuiltIn.Run_Keyword_If    """${pswd}""" != "rsa_id"     Login    ${user}    ${pswd}
    ${out2} =    BuiltIn.Run_Keyword_If    """${pswd}""" == "rsa_id"    SSHLibrary.Login_With_Public_Key    ${user}    %{HOME}/.ssh/id_rsa    any
    ${time} =    DateTime.Get_Current_Date
    BuiltIn.Run_Keyword_If    """${out}""" != "None"    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${name}.log    ${time}${\n}*** Response1: ${out}${\n}
    BuiltIn.Run_Keyword_If    """${out2}""" != "None"    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${name}.log    ${time}${\n}*** Response2: {out2}${\n}
    [Return]    ${connection}

Switch_And_Execute_With_Copied_File
    [Arguments]    ${ssh_session}    ${file_path}    ${command_prefix}    ${expected_rc}=0    ${ignore_stderr}=${False}    ${ignore_rc}=${False}
    [Documentation]    Switch to \${ssh_session} and continue with Execute_Command_With_Copied_File.
    BuiltIn.Log_Many    ${ssh_session}    ${file_path}    ${command_prefix}    ${expected_rc}    ${ignore_stderr}    ${ignore_rc}
    SSHLibrary.Switch_Connection    ${ssh_session}
    BuiltIn.Run_Keyword_And_Return    Execute_Command_With_Copied_File    ${file_path}    ${command_prefix}    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}    ignore_rc=${ignore_rc}

Execute_Command_With_Copied_File
    [Arguments]    ${file_path}    ${command_prefix}    ${expected_rc}=0    ${ignore_stderr}=${False}    ${ignore_rc}=${False}
    [Documentation]    Put file to current remote directory and execute command which takes computed file name as argument.
    BuiltIn.Log_Many    ${file_path}    ${command_prefix}    ${expected_rc}    ${ignore_stderr}    ${ignore_rc}
    Builtin.Comment    TODO: Do not pollute current remote directory.
    SSHLibrary.Put_File    ${file_path}    .
    ${splitted_path} =    String.Split_String    ${file_path}    separator=${/}
    BuiltIn.Run_Keyword_And_Return    Execute_Command_And_Log    ${command_prefix} @{splitted_path}[-1]    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}    ignore_rc=${ignore_rc}

Switch_Execute_And_Log_To_File
    [Arguments]    ${ssh_session}    ${command}    ${expected_rc}=0    ${ignore_stderr}=${False}    ${ignore_rc}=${False}    ${compress}=${False}
    [Documentation]    Call Switch_And_Execute_Command redirecting stdout to a remote file, download the file.
    ...    To distinguish separate invocations, suite name, test name, session alias
    ...    and full command are used to construct file name.
    BuiltIn.Log_Many    ${ssh_session}    ${command}    ${expected_rc}    ${ignore_stderr}    ${ignore_rc}    ${compress}
    SSHLibrary.Switch_Connection    ${ssh_session}
    ${connection} =    SSHLibrary.Get_Connection
    # In teardown, ${TEST_NAME} does not exist.
    ${testname} =    BuiltIn.Get_Variable_Value    ${TEST_NAME}    ${EMPTY}
    ${filename_with_spaces} =    BuiltIn.Set_Variable    ${testname}__${SUITE_NAME}__${connection.alias}__${command}.log
    ${filename} =    String.Replace_String    ${filename_with_spaces}    ${SPACE}    _
    BuiltIn.Log    ${filename}
    Execute_Command_And_Log    ${command} > ${filename}    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}    ignore_rc=${ignore_rc}
    BuiltIn.Run_Keyword_If    ${compress}    Execute_Command_And_Log    xz -9e ${filename}
    ${filename} =    Builtin.Set_Variable_If    ${compress}    ${filename}.xz    ${filename}
    SSHLibrary.Get_File    ${filename}    ${RESULTS_FOLDER}/${filename}
    SSHLibrary.Get_File    ${filename}    ${RESULTS_FOLDER_SUITE}/${filename}
    [Teardown]    Execute_Command_And_Log    rm ${filename}

Switch_And_Execute_Command
    [Arguments]    ${ssh_session}    ${command}    ${expected_rc}=0    ${ignore_stderr}=${False}    ${ignore_rc}=${False}
    [Documentation]    Switch to \${ssh_session}, and continue with Execute_Command_And_Log.
    BuiltIn.Log_Many    ${ssh_session}    ${command}    ${expected_rc}    ${ignore_stderr}    ${ignore_rc}
    SSHLibrary.Switch_Connection    ${ssh_session}
    ${out}=    Execute_Command_And_Log    ${command}    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}    ignore_rc=${ignore_rc}
    BuiltIn.Log    ${out}
    [Return]    ${out}

Execute_Command_And_Log
    [Arguments]    ${command}    ${expected_rc}=0    ${ignore_stderr}=${False}    ${ignore_rc}=${False}
    [Documentation]    Execute \${command} on current SSH session, log results, maybe fail on nonempty stderr, check \${expected_rc}, return stdout.
    BuiltIn.Log_Many    ${command}    ${expected_rc}    ${ignore_stderr}    ${ignore_rc}
    ${stdout}    ${stderr}    ${rc} =    SSHLibrary.Execute_Command    ${command}    return_stderr=True    return_rc=True
    BuiltIn.Log_Many    ${stdout}    ${stderr}    ${rc}
    Append_Command_Log    ${command}    ${stdout}    ${stderr}    ${rc}
    BuiltIn.Run_Keyword_Unless    ${ignore_stderr}    BuiltIn.Should_Be_Empty    ${stderr}
    BuiltIn.Run_Keyword_Unless    ${ignore_rc}    BuiltIn.Should_Be_Equal_As_Numbers    ${rc}    ${expected_rc}
    [Return]    ${stdout}

Switch_And_Write_Command
    [Arguments]    ${ssh_session}    ${command}    ${prompt}=vpp#
    [Documentation]    Switch to \${ssh_session}, and continue with Write_Command_And_Log
    BuiltIn.Log_Many    ${ssh_session}    ${command}    ${prompt}
    SSHLibrary.Switch_Connection    ${ssh_session}
    BuiltIn.Run_Keyword_And_Return    Write_Command_And_Log    ${command}    ${prompt}

Write_Command_And_Log
    [Arguments]    ${command}    ${prompt}=vpp#
    [Documentation]    Write \${command} on current SSH session, wait for prompt, log output, return output.
    BuiltIn.Log_Many    ${command}    ${prompt}
    SSHLibrary.Write    ${command}
    ${output} =    SSHLibrary.Read_Until    ${prompt}
    Append_Command_Log    ${command}    ${output}
    [Return]    ${output}

Append_Command_Log
    [Arguments]    ${command}    ${output}=${EMPTY}    ${stderr}=${EMPTY}    ${rc}=${EMPTY}
    [Documentation]    Detect connection alias and time, append line with command and output to appropriate log file.
    Builtin.Log_Many    ${command}    ${output}    ${stderr}    ${rc}
    ${connection} =    SSHLibrary.Get_Connection
    ${time} =    DateTime.Get_Current_Date
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${connection.alias}.log    ${time}${\n}*** Command: ${command}${\n}
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER_SUITE}/output_${connection.alias}.log    ${time}${\n}*** Command: ${command}${\n}
    ${output_length} =    BuiltIn.Get_Length    ${output}
    ${if_output} =    BuiltIn.Set_Variable_If    ${output_length}    ${output}${\n}    ${EMPTY}
    ${stderr_length} =    Builtin.Get_Length    ${stderr}
    ${if_stderr} =    BuiltIn.Set_Variable_If    ${stderr_length}    *** Stderr: ${stderr}${\n}    ${EMPTY}
    ${if_rc} =    BuiltIn.Set_Variable_If    """${rc}"""    *** Return code: ${rc}${\n}    ${EMPTY}
    ${time} =    DateTime.Get_Current_Date
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${connection.alias}.log    ${time}${\n}*** Response: ${if_stderr}${if_rc}${if_output}
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER_SUITE}/output_${connection.alias}.log    ${time}${\n}*** Response: ${if_stderr}${if_rc}${if_output}