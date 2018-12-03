*** Settings ***
Resource    ../../../libraries/kubernetes/Restarts_Setup.robot
Resource    ../../../libraries/kubernetes/KubeTestOperations.robot

Resource     ../../../variables/${VARIABLES}_variables.robot

Library    SSHLibrary

Force Tags        integration
Suite Setup       Run Keywords
...    KubeSetup.Kubernetes Suite Setup    ${CLUSTER_ID}
...    AND    Restarts Suite Setup with ${1} VNFs at ${1} memifs each and ${1} non-VPP containers
Suite Teardown    Restarts Suite Teardown
Test Setup        Run Keywords
...           Set Test Variable    @{upgrade_durations}    @{EMPTY}
...    AND    Set Test Variable    @{restart_durations}    @{EMPTY}
Test Teardown     Run Keywords
...           Log Many    ${upgrade_durations}    ${restart_durations}
...    AND    Recreate Topology If Test Failed

Documentation    Test suite for Kubernetes pod restarts using a single VNF pod
...    and a single non-VPP pod.
...
...    Restart performed through kubernetes pod deletion and through
...    segmentation fault signal sent to VPP.
...
...    Connectivity verified using "ping" command to and from the VNF
...    and non-VPP containers.

*** Variables ***
${VARIABLES}=       common
${ENV}=             common
${CLUSTER_ID}=      INTEGRATION1
${vnf0_ip}=         192.168.1.1
${novpp0_ip}=       192.168.1.2
@{novpp_pods}=      novpp-0
@{vnf_pods}=        vnf-vpp-0

${repeats}=         5

*** Test Cases ***
Basic restart scenario - VNF
    Repeat Keyword    ${repeats}    Basic restart scenario - VNF

Basic restart scenario - noVPP
    Repeat Keyword    ${repeats}    Basic restart scenario - noVPP

Basic restart scenario - VSwitch
    Repeat Keyword    ${repeats}    Basic restart scenario - VSwitch

Basic Restart Scenario - VSwitch and VNF
    Repeat Keyword    ${repeats}    Basic Restart Scenario - VSwitch and VNF

Basic Restart Scenario - VSwitch and noVPP
    Repeat Keyword    ${repeats}    Basic Restart Scenario - VSwitch and noVPP

Basic Restart Scenario - VSwitch, noVPP and VNF
    Repeat Keyword    ${repeats}    Basic Restart Scenario - VSwitch, noVPP and VNF

Basic Restart Scenario - full topology in sequence etcd-vswitch-pods-sfc
    Repeat Keyword    ${repeats}    Basic Restart Scenario - full topology in sequence etcd-vswitch-pods-sfc

Basic Restart Scenario - full topology in sequence etcd-vswitch-sfc-pods
    Repeat Keyword    ${repeats}    Basic Restart Scenario - full topology in sequence etcd-vswitch-sfc-pods

Basic Restart Scenario - full topology in sequence etcd-sfc-vswitch-pods
    Repeat Keyword    ${repeats}    Basic Restart Scenario - full topology in sequence etcd-sfc-vswitch-pods

Basic Restart Scenario - full topology in sequence etcd-sfc-pods-vswitch
    Repeat Keyword    ${repeats}    Basic Restart Scenario - full topology in sequence etcd-sfc-pods-vswitch

#TODO: verify connectivity with traffic (iperf,tcpkali,...) longer than memif ring size
#TODO: measure pod restart time

*** Keywords ***
Recreate Topology If Test Failed
    [Documentation]    After a failed test, delete the kubernetes topology
    ...    and create it again.
    BuiltIn.Run Keyword If Test Failed    Run Keywords
    ...    Log Pods For Debug    ${testbed_connection}
    ...    AND    Cleanup_Restarts_Deployment_On_Cluster    ${testbed_connection}
    ...    AND    Restarts Suite Setup with ${1} VNFs at ${1} memifs each and ${1} non-VPP containers

Basic restart scenario - VNF
    [Documentation]    Restart VNF node, ping it's IP address from the non-VPP
    ...    node until a reply is received, then verify connectivity both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    vnf-vpp-0
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

    Trigger Pod Restart - VPP SIGSEGV        ${vnf_pods[0]}
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${restart_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic restart scenario - noVPP
    [Documentation]    Restart non-VPP node, ping it's IP address from the VNF
    ...    node until a reply is received, then verify connectivity both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    novpp-0
    Wait For Reconnect - VPP Ping            ${vnf_pods[0]}           ${novpp0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}           ${novpp0_ip}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}         ${vnf0_ip}

Basic restart scenario - VSwitch
    [Documentation]    Restart the vswitch, ping the VNF's IP address from
    ...    the non-VPP node until a reply is received, then verify connectivity
    ...    both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    ${vswitch_pod_name}    vswitch=${TRUE}
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

    Trigger Pod Restart - VPP SIGSEGV        ${vswitch_pod_name}
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${restart_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - VSwitch and VNF
    [Documentation]    Restart vswitch and VNF, ping the VNF's IP address from
    ...    the non-VPP node until a reply is received, then verify connectivity
    ...    both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    vnf-vpp-0
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    ${vswitch_pod_name}    vswitch=${TRUE}
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

    Trigger Pod Restart - VPP SIGSEGV        ${vnf_pods[0]}
    Trigger Pod Restart - VPP SIGSEGV        ${vswitch_pod_name}
    Wait For Reconnect - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s    duration_list_name=${restart_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - VSwitch and noVPP
    [Documentation]    Restart vswitch and non-VPP pod, ping the non-VPP
    ...    pod's IP address from the VNF node until a reply is received, then
    ...    verify connectivity both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    novpp-0
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    ${vswitch_pod_name}    vswitch=${TRUE}
    Wait For Reconnect - VPP Ping            ${vnf_pods[0]}           ${novpp0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    novpp-0
    Trigger Pod Restart - VPP SIGSEGV        ${vswitch_pod_name}
    Wait For Reconnect - VPP Ping            ${vnf_pods[0]}           ${novpp0_ip}    timeout=120s    duration_list_name=${restart_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - VSwitch, noVPP and VNF
    [Documentation]    Restart vswitch, VNF and non-VPP pod, ping the non-VPP
    ...    pod's IP address from the VNF node until a reply is received, then
    ...    verify connectivity both ways.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    vnf-vpp-0
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    novpp-0
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    ${vswitch_pod_name}    vswitch=${TRUE}
    Wait For Reconnect - VPP Ping            ${vnf_pods[0]}           ${novpp0_ip}    timeout=120s    duration_list_name=${upgrade_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

    Trigger Pod Restart - VPP SIGSEGV        ${vnf_pods[0]}
    Trigger Pod Restart - Pod Deletion       ${testbed_connection}    novpp-0
    Trigger Pod Restart - VPP SIGSEGV        ${vswitch_pod_name}
    Wait For Reconnect - VPP Ping            ${vnf_pods[0]}           ${novpp0_ip}    timeout=120s    duration_list_name=${restart_durations}
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - full topology in sequence etcd-vswitch-pods-sfc
    [Documentation]    Restart the full topology, then bring it back up in the
    ...    specified sequence and verify connectivity between VNF and non-VPP
    ...    pods.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Restart Topology With Startup Sequence    etcd    vswitch    vnf    novpp    sfc
    Ping Until Success - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - full topology in sequence etcd-vswitch-sfc-pods
    [Documentation]    Restart the full topology, then bring it back up in the
    ...    specified sequence and verify connectivity between VNF and non-VPP
    ...    pods.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Restart Topology With Startup Sequence    etcd    vswitch    sfc    vnf    novpp
    Ping Until Success - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - full topology in sequence etcd-sfc-vswitch-pods
    [Documentation]    Restart the full topology, then bring it back up in the
    ...    specified sequence and verify connectivity between VNF and non-VPP
    ...    pods.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Restart Topology With Startup Sequence    etcd    sfc    vswitch    vnf    novpp
    Ping Until Success - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}

Basic Restart Scenario - full topology in sequence etcd-sfc-pods-vswitch
    [Documentation]    Restart the full topology, then bring it back up in the
    ...    specified sequence and verify connectivity between VNF and non-VPP
    ...    pods.

    Ping Until Success - Unix Ping    ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Restart Topology With Startup Sequence    etcd    sfc    vnf    novpp    vswitch
    Ping Until Success - Unix Ping           ${novpp_pods[0]}    ${vnf0_ip}    timeout=120s
    Verify Pod Connectivity - Unix Ping      ${novpp_pods[0]}    ${vnf0_ip}
    Verify Pod Connectivity - VPP Ping       ${vnf_pods[0]}      ${novpp0_ip}