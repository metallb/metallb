*** Settings ***
Library     SSHLibrary
Library     ${CURDIR}/kube_config_gen.py
Resource    ${CURDIR}/KubeEnv.robot
Resource    ${CURDIR}/KubeSetup.robot
Resource    ${CURDIR}/KubeCtl.robot
Resource    ${CURDIR}/../SshCommons.robot
Documentation    Contains keywords uset to setup and teardown test suites
...    focused on restart scenarios.

*** Keywords ***
Restarts Suite Setup with ${vnf_count} VNFs at ${memif_per_vnf} memifs each and ${novpp_count} non-VPP containers
    [Documentation]    Clear any existing Kubernetes elements and deploy the topology.
    ${vnf_count}=    BuiltIn.Convert_to_Integer    ${vnf_count}
    ${novpp_count}=    BuiltIn.Convert_to_Integer    ${novpp_count}
    Cleanup_Restarts_Deployment_On_Cluster    ${testbed_connection}
    ${topology}=    Generate YAML Config Files    ${vnf_count}    ${novpp_count}    ${memif_per_vnf}
    Set Suite Variable    ${topology}
    Set Suite Variable    ${vnf_count}
    Set Suite Variable    ${novpp_count}
    KubeEnv.Deploy_Etcd_And_Verify_Running    ${testbed_connection}
    KubeEnv.Deploy_Vswitch_Pod_And_Verify_Running    ${testbed_connection}
    KubeEnv.Deploy_VNF_Pods    ${testbed_connection}    ${vnf_count}
    KubeEnv.Deploy_NoVPP_Pods    ${testbed_connection}    ${novpp_count}
    KubeEnv.Deploy_SFC_Pod_And_Verify_Running    ${testbed_connection}
    Open_Restarts_Connections    node_index=1    cluster_id=${CLUSTER_ID}

Restarts_Suite_Teardown
    [Documentation]    Log leftover output from pods, remove pods, execute common teardown.
    KubeEnv.Log_Pods_For_Debug    ${testbed_connection}    exp_nr_vswitch=1
    Close_Restarts_Connections
    Cleanup_Restarts_Deployment_On_Cluster    ${testbed_connection}
    KubeSetup.Kubernetes Suite Teardown    ${CLUSTER_ID}

Cleanup_Restarts_Deployment_On_Cluster
    [Arguments]    ${testbed_connection}
    [Documentation]    Delete all Kubernetes elements and wait for completion.
    SSHLibrary.Switch_Connection  ${testbed_connection}
    SshCommons.Execute_Command_And_Log    kubectl delete all --all --namespace=default
    Wait_Until_Pod_Removed    ${testbed_connection}

Open_Restarts_Connections
    [Documentation]    Open and save SSH connections to ETCD and SFC pods.
    [Arguments]    ${node_index}=1    ${cluster_id}=INTEGRATION1
    BuiltIn.Log Many    ${node_index}    ${cluster_id}

    ${etcd_connection}=    KubeEnv.Open_Connection_To_Node    etcd    ${cluster_id}     ${node_index}
    BuiltIn.Set_Suite_Variable    ${etcd_connection}
    KubeEnv.Get_Into_Container_Prompt_In_Pod    ${etcd_connection}    ${etcd_pod_name}    prompt=#

    ${sfc_connection}=    KubeEnv.Open_Connection_To_Node    sfc    ${cluster_id}     ${node_index}
    BuiltIn.Set_Suite_Variable    ${sfc_connection}
    KubeEnv.Get_Into_Container_Prompt_In_Pod    ${sfc_connection}    ${sfc_pod_name}    prompt=#

Close_Restarts_Connections
    [Documentation]    Close SSH connections to ETCD and SFC pods.
    BuiltIn.Log Many    ${sfc_connection}    ${etcd_connection}
    KubeEnv.Leave_Container_Prompt_In_Pod    ${sfc_connection}
    KubeEnv.Leave_Container_Prompt_In_Pod    ${etcd_connection}

    SSHLibrary.Switch_Connection    ${sfc_connection}
    SSHLibrary.Close_Connection
    SSHLibrary.Switch_Connection    ${etcd_connection}
    SSHLibrary.Close_Connection

Generate_YAML_Config_Files
    [Documentation]    Generate YAML config files for the desired topology.
    [Arguments]    ${vnf_count}    ${novpp_count}    ${memif_per_vnf}
    BuiltIn.Log Many    ${vnf_count}    ${novpp_count}    ${memif_per_vnf}
    ${topology}=    kube_config_gen.generate_config
    ...    ${vnf_count}    ${novpp_count}    ${memif_per_vnf}
    ...    ${CURDIR}/../../resources/k8-yaml    ${K8_GENERATED_CONFIG_FOLDER}
    ...    ${AGENT_VPP_IMAGE_NAME}    ${VNF_IMAGE_NAME}    ${SFC_CONTROLLER_IMAGE_NAME}
    [Return]    ${topology}
