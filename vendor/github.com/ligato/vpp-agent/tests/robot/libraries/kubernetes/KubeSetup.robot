*** Settings ***
Library    OperatingSystem
Library       SSHLibrary            timeout=15s        loglevel=TRACE
Resource    ../setup-teardown.robot
Resource    ../SshCommons.robot
Documentation     Contains keywords used to setup and teardown Kubernetes tests.

*** Variables ***


*** Keywords ***
Kubernetes Suite Setup
    [Arguments]    ${cluster_id}
    [Documentation]    Perform actions common for setup of every suite.
    BuiltIn.Log    ${cluster_id}
    setup-teardown.Discard Old Results
    Create_Connections_To_Kube_Cluster      ${cluster_id}
    BuiltIn.Set_Suite_Variable    ${testbed_connection}    vm_1

Kubernetes Suite Teardown
    [Arguments]    ${cluster_id}
    [Documentation]    Perform actions common for teardown of every suite.
    BuiltIn.Log    ${cluster_id}
    Kubernetes Log SSH Output    ${cluster_id}
    SSHLibrary.Get_Connections
    SSHLibrary.Close_All_Connections

Kubernetes Log SSH Output
    [Arguments]    ${cluster_id}
    [Documentation]    Call Log_\${vm}_Output for every cluster node.
    [Timeout]    ${SSH_LOG_OUTPUTS_TIMEOUT}
    BuiltIn.Log    ${cluster_id}
    : FOR    ${index}    IN RANGE    1    ${K8_CLUSTER_${cluster_id}_NODES}+1
    \    Kubernetes Log ${VM_SSH_ALIAS_PREFIX}${index} Output

Kubernetes Log ${vm} Output
    [Documentation]    Switch to \${vm} SSH connection, read with delay of ${SSH_READ_DELAY}, Log and append to log file.
    BuiltIn.Log_Many    ${vm}
    BuiltIn.Comment    TODO: Rewrite this keyword with ${vm} being explicit argument.
    SSHLibrary.Switch_Connection    ${vm}
    ${out} =    SSHLibrary.Read    delay=${SSH_READ_DELAY}s
    BuiltIn.Log    ${out}
    OperatingSystem.Append_To_File    ${RESULTS_FOLDER}/output_${vm}.log    ${out}

Get Kubernetes VM Status
    [Arguments]    ${vm}
    [Documentation]    Execute df, free, ifconfig -a, ps -aux... on vm, assuming ssh connection there is active.
    BuiltIn.Log_Many    ${vm}
    SshCommons.Execute_Command_And_Log    whoami
    SshCommons.Execute_Command_And_Log    pwd
    SshCommons.Execute_Command_And_Log    df
    SshCommons.Execute_Command_And_Log    free
    SshCommons.Execute_Command_And_Log    ip address
    SshCommons.Execute_Command_And_Log    ps aux
    SshCommons.Execute_Command_And_Log    export
    SshCommons.Execute_Command_And_Log    docker images
    SshCommons.Execute_Command_And_Log    docker ps -as
    BuiltIn.Return_From_Keyword_If    """${vm}""" != """${VM_SSH_ALIAS_PREFIX}1"""
    SshCommons.Execute_Command_And_Log    kubectl get nodes    ignore_stderr=True    ignore_rc=True
    SshCommons.Execute_Command_And_Log    kubectl get pods    ignore_stderr=True    ignore_rc=True
    
Create Connections To Kube Cluster
    [Arguments]    ${cluster_id}
    [Documentation]    Create connection and log machine status for each node.
    BuiltIn.Log    ${cluster_id}
    : FOR    ${index}    IN RANGE    1    ${K8_CLUSTER_${cluster_id}_NODES}+1
    \    SshCommons.Open_Ssh_Connection_Kube    ${VM_SSH_ALIAS_PREFIX}${index}    ${K8_CLUSTER_${cluster_id}_VM_${index}_PUBLIC_IP}    ${K8_CLUSTER_${cluster_id}_VM_${index}_USER}    ${K8_CLUSTER_${cluster_id}_VM_${index}_PSWD}
    \    SSHLibrary.Set_Client_Configuration    prompt=${K8_CLUSTER_${cluster_id}_VM_${index}_PROMPT}
    \    Get_Machine_Status    ${VM_SSH_ALIAS_PREFIX}${index}
    BuiltIn.Set_Suite_Variable    ${testbed_connection}    ${VM_SSH_ALIAS_PREFIX}1
    SSHLibrary.Switch_Connection  ${testbed_connection}

Make K8 ETCD Snapshots
    [Arguments]    ${tag}=notag
    [Documentation]    Log ${tag}, compute next prefix and log ETCD status with the prefix.
    BuiltIn.Log_Many    ${tag}
    ${prefix} =    Create_K8_Next_Snapshot_Prefix
    setup-teardown.Take ETCD Snapshots    ${prefix}_${tag}    ${testbed_connection}

Create_K8_Next_Snapshot_Prefix
    [Documentation]    Contruct new prefix, store next snapshot num. Return the prefix.
    ${prefix} =    BuiltIn.Evaluate    str(${snapshot_num}).zfill(3)
    ${snapshot_num} =    BuiltIn.Evaluate    ${snapshot_num}+1
    BuiltIn.Set_Global_Variable    ${snapshot_num}
    [Return]    ${prefix}
