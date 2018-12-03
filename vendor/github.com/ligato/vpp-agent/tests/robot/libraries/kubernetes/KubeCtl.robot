*** Settings ***
Documentation     This is a library to handle kubectl commands on the remote machine, towards which
...               ssh connection is opened.
Resource          ${CURDIR}/../all_libs.robot

*** Keywords ***
Apply_F
    [Arguments]    ${ssh_session}    ${file_path}
    [Documentation]    Execute "kubectl apply -f" with given local file.
    BuiltIn.Log_Many    ${ssh_session}    ${file_path}
    SshCommons.Switch_And_Execute_With_Copied_File    ${ssh_session}    ${file_path}    kubectl apply -f

Apply_F_Url
    [Arguments]    ${ssh_session}    ${url}
    [Documentation]    Execute "kubectl apply -f" with given \${url}.
    BuiltIn.Log_Many    ${ssh_session}    ${url}
    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl apply -f ${url}

Delete_F
    [Arguments]    ${ssh_session}    ${file_path}    ${expected_rc}=0    ${ignore_stderr}=${False}
    [Documentation]    Execute "kubectl delete -f" with given local file.
    BuiltIn.Log_Many    ${ssh_session}    ${file_path}    ${expected_rc}    ${ignore_stderr}
    SshCommons.Switch_And_Execute_With_Copied_File    ${ssh_session}    ${file_path}    kubectl delete -f    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}

Delete_F_Url
    [Arguments]    ${ssh_session}    ${url}    ${expected_rc}=0    ${ignore_stderr}=${False}
    [Documentation]    Execute "kubectl delete -f" with given \${url}.
    BuiltIn.Log_Many    ${ssh_session}    ${url}    ${expected_rc}    ${ignore_stderr}
    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl delete -f ${url}    expected_rc=${expected_rc}    ignore_stderr=${ignore_stderr}

Get_Pod
    [Arguments]    ${ssh_session}    ${pod_name}    ${namespace}=default
    [Documentation]    Execute "kubectl get pod -n" for given \${namespace} and \${pod_name}, parse, log and return the parsed result.
    Builtin.Log_Many    ${ssh_session}    ${pod_name}    ${namespace}
    ${stdout} =    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl get pod -n ${namespace} ${pod_name}
    ${output} =    kube_parser.parse_kubectl_get_pods    ${stdout}
    BuiltIn.Log    ${output}
    [Return]    ${output}

Get_Pods
    [Arguments]    ${ssh_session}    ${namespace}=default
    [Documentation]    Execute "kubectl get pods -n" for given \${namespace} tolerating zero resources, parse, log and return the parsed output.
    BuiltIn.Log_Many    ${ssh_session}    ${namespace}
    ${status}    ${message} =    BuiltIn.Run_Keyword_And_Ignore_Error    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl get pods -n ${namespace}
    BuiltIn.Run_Keyword_If    """${status}""" == """FAIL""" and """No resources found""" not in """${message}"""    BuiltIn.Fail    msg=${message}
    ${output} =    kube_parser.parse_kubectl_get_pods    ${message}
    BuiltIn.Log    ${output}
    [Return]    ${output}

Get_Pods_Wide
    [Arguments]    ${ssh_session}
    [Documentation]    Execute "kubectl get pods -o wide", parse, log and return the parsed outpt.
    Builtin.Log_Many    ${ssh_session}
    ${stdout} =    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl get pods -o wide
    ${output} =    kube_parser.parse_kubectl_get_pods    ${stdout}
    BuiltIn.Log    ${output}
    [Return]    ${output}

Get_Pods_All_Namespaces
    [Arguments]    ${ssh_session}
    [Documentation]    Execute "kubectl get pods --all-namespaces", parse, log and return the parsed outpt.
    Builtin.Log_Many    ${ssh_session}
    ${stdout} =    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl get pods --all-namespaces
    ${output} =    kube_parser.parse_kubectl_get_pods    ${stdout}
    BuiltIn.Log    ${output}
    [Return]    ${output}

Get_Nodes
    [Arguments]    ${ssh_session}
    [Documentation]    Execute "kubectl get nodes", parse, log and return the parsed outpt.
    Builtin.Log_Many    ${ssh_session}
    ${stdout} =    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl get nodes
    ${output} =    kube_parser.parse_kubectl_get_nodes    ${stdout}
    BuiltIn.Log    ${output}
    [Return]    ${output}

Logs
    [Arguments]    ${ssh_session}    ${pod_name}    ${container}=${EMPTY}    ${namespace}=${EMPTY}    ${compress}=${False}
    [Documentation]    Execute "kubectl logs" with given params, log output into a result file.
    BuiltIn.Log_Many    ${ssh_session}    ${pod_name}    ${container}    ${namespace}    ${compress}
    ${nsparam} =     BuiltIn.Set_Variable_If    """${namespace}""" != """${EMPTY}"""    --namespace ${namespace}    ${EMPTY}
    ${cntparam} =    BuiltIn.Set_Variable_If    """${container}""" != """${EMPTY}"""    ${container}    ${EMPTY}
    SshCommons.Switch_Execute_And_Log_To_File    ${ssh_session}    kubectl logs ${nsparam} ${pod_name} ${cntparam}    compress=${compress}

Describe_Pod
    [Arguments]    ${ssh_session}    ${pod_name}
    [Documentation]    Execute "kubectl describe pod" with given \${pod_name}, parse, log and return the parsed details.
    BuiltIn.Log_Many    ${ssh_session}    ${pod_name}
    ${output} =    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl describe pod ${pod_name}
    ${details} =   kube_parser.parse_kubectl_describe_pod    ${output}
    BuiltIn.Log    ${details}
    [Return]    ${details}

Taint
    [Arguments]    ${ssh_session}    ${cmd_parameters}
    [Documentation]    Execute "kubectl taint" with given \${cmd_parameters}, return the result.
    Builtin.Log_Many    ${ssh_session}    ${cmd_parameters}
    BuiltIn.Run_Keyword_And_Return    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl taint ${cmd_parameters}

Label_Nodes
    [Arguments]    ${ssh_session}    ${node_name}   ${label_key}    ${label_value}
    [Documentation]    Execute "kubectl label nodes" with given parameters, return the result.
    Builtin.Log_Many    ${ssh_session}    ${node_name}   ${label_key}    ${label_value}
    BuiltIn.Run_Keyword_And_Return    SshCommons.Switch_And_Execute_Command    ${ssh_session}    kubectl label nodes ${node_name} ${label_key}=${label_value}

Get_Container_Id
    [Arguments]    ${ssh_session}    ${pod_name}    ${container}=${EMPTY}
    [Documentation]    Return \${container} or describe pod, parse for first container ID, log and return that.
    ...    As kubectl is usually only present only on master node, switch to \${ssh_session} is done after.
    BuiltIn.Log_Many    ${ssh_session}    ${pod_name}    ${container}
    Builtin.Return_From_Keyword_If    """${container}"""    ${container}
    ${output} =    SshCommons.Switch_And_Execute_Command    ${VM_SSH_ALIAS_PREFIX}1    kubectl describe pod ${pod_name}
    SSHLibrary.Switch_Connection    ${ssh_session}
    ${id} =    kube_parser.parse_for_first_container_id    ${output}
    Builtin.Log    ${id}
    [Return]    ${id}

Execute_On_Pod
    [Arguments]    ${ssh_session}    ${pod_name}    ${cmd}    ${container}=${EMPTY}    ${tty}=${False}    ${stdin}=${False}    ${privileged}=${True}    ${ignore_stderr}=${False}    ${ignore_rc}=${False}
    [Documentation]    Execute "docker exec" with given parameters, return the result.
    ...    Container ID is autodetected if empty. This only works if \${ssh_session} points to the correct host.
    Builtin.Log_Many    ${ssh_session}    ${pod_name}    ${cmd}    ${container}    ${tty}    ${stdin}    ${privileged}    ${ignore_stderr}    ${ignore_rc}
    ${container_id} =    Get_Container_Id    ${ssh_session}    ${pod_name}    ${container}
    ${t_param} =    BuiltIn.Set_Variable_If    ${tty}    -t    ${EMPTY}
    ${i_param} =    BuiltIn.Set_Variable_If    ${stdin}    -i    ${EMPTY}
    ${p_param} =    BuiltIn.Set_Variable_If    ${privileged}    --privileged=true    --privileged=false
    ${docker} =    BuiltIn.Set_Variable    ${KUBE_CLUSTER_${CLUSTER_ID}_DOCKER_COMMAND}
    BuiltIn.Run_Keyword_And_Return    SshCommons.Switch_And_Execute_Command    ${ssh_session}    ${docker} exec ${i_param} ${t_param} ${p_param} ${container_id} ${cmd}    ignore_stderr=${ignore_stderr}    ignore_rc=${ignore_rc}

# TODO: Add more commands like Get Deployment, Get statefulset describe deployment, describe statefulset etc...