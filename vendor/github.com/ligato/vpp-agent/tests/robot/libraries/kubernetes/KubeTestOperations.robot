*** Settings ***
Resource    KubeEnv.robot
Resource    ../SshCommons.robot

*** Keywords ***
Verify Pod Connectivity - Unix Ping
    [Documentation]    Run ping command from the named pod to the specified
    ...    IP address. Check that no packets were lost.
    [Arguments]    ${source_pod_name}    ${destination_ip}     ${count}=5
    ${stdout} =    Run Command In Pod    ping -c ${count} -s 1400 ${destination_ip}    ${source_pod_name}
    BuiltIn.Log Many    ${source_pod_name}    ${destination_ip}     ${count}
    BuiltIn.Should Contain    ${stdout}    ${count} received, 0% packet loss

Verify Pod Connectivity - VPP Ping
    [Documentation]    Run ping command from the named pod's VPP command line
    ...    to the specified IP address. Check that no packets were lost.
    [Arguments]    ${source_pod_name}    ${destination_ip}     ${count}=5
    BuiltIn.Log Many    ${source_pod_name}    ${destination_ip}     ${count}
    ${stdout} =    Run Command In Pod    vppctl ping ${destination_ip} repeat ${count}    ${source_pod_name}
    BuiltIn.Should Contain    ${stdout}    ${count} received, 0% packet loss

Trigger Pod Restart - VPP SIGSEGV
    [Documentation]    Trigger a pod restart by sending signal 11 to the pod's
    ...    running VPP process.
    [Arguments]    ${pod_name}
    BuiltIn.Log    ${pod_name}
    ${stdout} =    Run Command In Pod    pkill --signal 11 -f /usr/bin/vpp    ${pod_name}

Trigger Pod Restart - Pod Deletion
    [Documentation]    Trigger a pod restart by deleting the pod using kubectl.
    [Arguments]    ${ssh_session}    ${pod_name}    ${vswitch}=${FALSE}
    BuiltIn.Log Many    ${ssh_session}    ${pod_name}    ${vswitch}
    ${stdout} =    Switch_And_Execute_Command    ${ssh_session}    kubectl delete pod ${pod_name}
    Wait Until Keyword Succeeds    20sec    1sec    KubeEnv.Verify_Pod_Not_Terminating    ${ssh_session}    ${pod_name}
    Run Keyword If    ${vswitch}    Get Vswitch Pod Name    ${ssh_session}

Ping Until Success - Unix Ping
    [Documentation]    Execute ping loop on the named pod to the specified
    ...    IP address. Blocks until the ping succeeds or this keyword times out.
    [Arguments]    ${source_pod_name}    ${destination_ip}    ${timeout}
    [Timeout]    ${timeout}
    BuiltIn.Log Many    ${source_pod_name}    ${destination_ip}    ${timeout}
    ${stdout} =    Run Command In Pod    /bin/bash -c "until ping -c1 -w1 ${destination_ip} &>/dev/null; do :; done"    ${source_pod_name}

Ping Until Success - VPP Ping
    [Documentation]    Repeatedly execute ping from the named pod's VPP until
    ...    the ping succeeds or timeout is reached.
    [Arguments]    ${source_pod_name}    ${destination_ip}    ${timeout}
    BuiltIn.Log Many    ${source_pod_name}    ${destination_ip}    ${timeout}
    BuiltIn.Wait Until Keyword Succeeds    ${timeout}    1s    Verify Pod Connectivity - VPP Ping    ${source_pod_name}    ${destination_ip}    count=1

Get Vswitch Pod Name
    [Documentation]    Get the kubernetes name of currently deployed vswitch pod.
    ...    Note that a new name is generated after each pod restart.
    [Arguments]    ${ssh_session}
    BuiltIn.Log Many    ${ssh_session}
    ${vswitch_pod_name} =    Get_Deployed_Pod_Name    ${ssh_session}    vswitch-deployment-
    Set Global Variable    ${vswitch_pod_name}

Restart Topology With Startup Sequence
    [Documentation]    Shutdown the kubernetes topology, then bring it back up
    ...    in the specified order. Note that some pods will not start
    ...    if ETCD is  not present.
    [Arguments]    @{sequence}
    BuiltIn.Log Many    @{sequence}
    Cleanup_Restarts_Deployment_On_Cluster    ${testbed_connection}
    :FOR    ${item}    IN    @{sequence}
    \    Run Keyword If    "${item}"=="etcd"       KubeEnv.Deploy_Etcd_And_Verify_Running    ${testbed_connection}
    \    Run Keyword If    "${item}"=="vswitch"    KubeEnv.Deploy_Vswitch_Pod_And_Verify_Running    ${testbed_connection}
    \    Run Keyword If    "${item}"=="sfc"        KubeEnv.Deploy_SFC_Pod_And_Verify_Running    ${testbed_connection}
    \    Run Keyword If    "${item}"=="vnf"        KubeEnv.Deploy_VNF_Pods    ${testbed_connection}    ${vnf_count}
    \    Run Keyword If    "${item}"=="novpp"      KubeEnv.Deploy_NoVPP_Pods    ${testbed_connection}    ${novpp_count}

Scale Verify Connectivity - Unix Ping
    [Documentation]    Verify connectivity between pods in scale test scenario.
    ...    each non-VPP container attempts to ping every VNF pod.
    [Arguments]    ${timeout}=30m
    [Timeout]    ${timeout}
    BuiltIn.Log Many    ${topology}    ${timeout}
    :FOR    ${bridge_segment}    IN    @{topology}
    \    Iterate_Over_VNFs    ${bridge_segment}

Iterate_Over_VNFs
    [Documentation]    Awkward implementation of nested for loop within robotframework.
    [Arguments]    ${bridge_segment}    ${timeout}=10m
    [Timeout]    ${timeout}
    BuiltIn.Log Many    ${bridge_segment}    ${timeout}
    :FOR    ${vnf_pod}    IN    @{bridge_segment["vnf"]}
    \    Iterate_Over_Novpps    ${bridge_segment}    ${vnf_pod}

Iterate_Over_Novpps
    [Documentation]    Awkward implementation of nested for loop within robotframework.
    [Arguments]    ${bridge_segment}    ${vnf_pod}    ${timeout}=10s
    BuiltIn.Log Many    ${bridge_segment}    ${vnf_pod}    ${timeout}
    :FOR    ${novpp_pod}    IN    @{bridge_segment["novpp"]}
    \    Ping Until Success - Unix Ping    ${novpp_pod["name"]}    ${vnf_pod["ip"]}    ${timeout}

Wait For Reconnect - Unix Ping
    [Documentation]    Run "Ping Until Success", measure and report execution time.
    [Arguments]    ${source_pod_Name}     ${destination_ip}    ${timeout}    ${duration_list_name}
    BuiltIn.Log Many    ${source_pod_Name}     ${destination_ip}    ${timeout}
    ${start_time} =    DateTime.Get Current Date    result_format=epoch
    Ping Until Success - Unix Ping    ${source_pod_Name}     ${destination_ip}    ${timeout}
    ${end_time} =    DateTime.Get Current Date    result_format=epoch
    ${duration} =    Datetime.Subtract Date from Date    ${start_time}    ${end_time}   result_format=verbose
    Collections.Append To List    ${duration_list_name}    ${duration}

Wait For Reconnect - VPP Ping
    [Documentation]    Run "Ping Until Success", measure and report execution time.
    [Arguments]    ${source_pod_Name}     ${destination_ip}    ${timeout}    ${duration_list_name}
    BuiltIn.Log Many    ${source_pod_Name}     ${destination_ip}    ${timeout}
    ${start_time} =    DateTime.Get Current Date    result_format=epoch
    Ping Until Success - VPP Ping    ${source_pod_Name}     ${destination_ip}    ${timeout}
    ${end_time} =    DateTime.Get Current Date    result_format=epoch
    ${duration} =    Datetime.Subtract Date from Date    ${start_time}    ${end_time}   result_format=verbose
    Collections.Append To List    ${duration_list_name}    ${duration}

Scale Pod Restart - Pod Deletion
    [Documentation]    Trigger pod restart in scale test scenario. Restart
    ...    the first pod of the specified type in each bridge segment.
    [Arguments]    ${pod_type}
    :FOR    ${bridge_segment}    IN    @{topology}
    \    Trigger Pod Restart - Pod Deletion    ${testbed_connection}    ${bridge_segment["${pod_type}"][0]["name"]}

Scale Pod Restart - VPP SIGSEGV
    [Documentation]    Trigger pod restart in scale test scenario. Restart
    ...    the first pod of the specified type in each bridge segment.
    [Arguments]    ${pod_type}
    :FOR    ${bridge_segment}    IN    @{topology}
    \    Trigger Pod Restart - VPP SIGSEGV    ${bridge_segment["${pod_type}"][0]["name"]}

Scale Wait For Reconnect - Unix Ping
    [Documentation]    Run "Ping Until Success" sequentially for each pod
    ...    restarted in scale test scenario.
    [Arguments]    ${timeout_per_bridge}=120s
    :FOR    ${bridge_segment}    IN    @{topology}
    \    Ping Until Success - Unix Ping    ${bridge_segment["novpp"][0]["name"]}    ${bridge_segment["vnf"][0]["ip"]}    ${timeout_per_bridge}

Scale Wait For Reconnect - VPP Ping
    [Documentation]    Run "Ping Until Success" sequentially for each pod
    ...    restarted in scale test scenario.
    [Arguments]    ${timeout_per_bridge}=120s
    :FOR    ${bridge_segment}    IN    @{topology}
    \    Ping Until Success - VPP Ping    ${bridge_segment["vnf"][0]["name"]}    ${bridge_segment["novpp"][0]["ip"]}    ${timeout_per_bridge}
