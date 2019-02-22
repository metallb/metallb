*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv6
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
${RESYNC_SLEEP}=       15s
# wait for resync vpps after restart
${RESYNC_WAIT}=        30s
${VETH_IP1}=              fd33::1:b:0:0:1
${VETH_IP2}=              fd33::1:b:0:0:2
${VETH_IP3}=              fd31::1:a:0:0:1
${VETH_IP4}=              fd31::1:a:0:0:2
${VETH_IP5}=              fd33::1:b:0:0:5
${VETH_IP6}=              fd33::1:b:0:0:6
${VETH_IP7}=              fd33::1:b:0:0:7
${GOOGLE_IP}=             2001:4860:4860::8888
${QUAD9_IP}=              2620:fe::fe
*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node    agent_vpp_1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1
    Write To Machine    vpp_agent_ctl    vpp-agent-ctl ${AGENT_VPP_ETCD_CONF_PATH} -ps

Setup Interfaces
    etcdctl.Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns1    name=ns1_veth1    host_if_name=ns1_veth1_linux    mac=d2:74:8c:12:67:d2    peer=ns2_veth2    ip=${VETH_IP1}     prefix=64
    etcdctl.Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns2    name=ns2_veth2    host_if_name=ns2_veth2_linux    mac=92:c7:42:67:ab:cd    peer=ns1_veth1    ip=${VETH_IP2}     prefix=64

Chceck Interfaces
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces    node=agent_vpp_1    namespace=ns2    interface=ns2_veth2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces Link    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces Link    node=agent_vpp_1    namespace=ns2    interface=ns2_veth2

Ping
    # This should work by default after veth interface setup
    Ping6 in namespace    node=agent_vpp_1    namespace=ns1    ip=${VETH_IP2}
    Ping6 in namespace    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP1}

Create Linux Routes
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1    routename=pingingveth2    ip=${VETH_IP5}    prefix=128    next_hop=${VETH_IP2}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns2    interface=ns2_veth2    routename=pingingveth1    ip=${VETH_IP6}    prefix=128    next_hop=${VETH_IP1}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1    routename=pinginggoogl    ip=${GOOGLE_IP}    prefix=128    next_hop=${VETH_IP2}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns2    interface=ns2_veth2    routename=pinging9    ip=${QUAD9_IP}    prefix=128    next_hop=${VETH_IP1}

Check Linux Routes
    #sleep  36
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns1    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP6}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns1    ip=${GOOGLE_IP}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${QUAD9_IP}


    # created routes should not exist in other namespace
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns1    ip=${VETH_IP6}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns2    ip=${GOOGLE_IP}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns1    ip=${QUAD9_IP}

Read Route Information From Setup Database
    etcdctl.Get Linux Route As Json    node=agent_vpp_1    routename=pingingveth2
    etcdctl.Get Linux Route As Json    node=agent_vpp_1    routename=pingingveth1
    etcdctl.Get Linux Route As Json    node=agent_vpp_1    routename=pinginggoogl
    etcdctl.Get Linux Route As Json    node=agent_vpp_1    routename=pinging9

Change Linux Routes Without Deleting Key (Changing Metric)
    # changing of gateway - this is incorrect/ the record would not be put in the database  - Let us change metric
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1    routename=pinginggoogl    ip=${GOOGLE_IP}    prefix=128    next_hop=${VETH_IP2}    metric=55


    # testing if there is the new metric
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Metric    node=agent_vpp_1    namespace=ns1    ip=${GOOGLE_IP}    metric=55

Change Linux Routes At First Deleting Key And Putting The Same Secondly Deleting Key Then Putting It To Other Namespace
    etcdctl.Delete Linux Route    node=agent_vpp_1    routename=pinging9

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns2    ip=${QUAD9_IP}

    # we create exactly the same as deleted route
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns2    interface=ns2_veth2    routename=pinging9    ip=${QUAD9_IP}    prefix=128    next_hop=${VETH_IP1}

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${QUAD9_IP}

    # delete again
    etcdctl.Delete Linux Route    node=agent_vpp_1    routename=pinging9

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns2    ip=${QUAD9_IP}

    # we try to transfer route to other namespace - there is also need to change appropriately gateway
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns1    interface=ns1_veth1    routename=pinging9    ip=${QUAD9_IP}    prefix=128    next_hop=${VETH_IP2}

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Removed Linux Route IPv6    node=agent_vpp_1    namespace=ns2    ip=${QUAD9_IP}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Gateway    node=agent_vpp_1    namespace=ns1    ip=${QUAD9_IP}    next_hop=${VETH_IP2}

At first create route and after that create inteface in namespace 3
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns3    interface=ns3_veth3    routename=pingingns2_veth3    ip=${VETH_IP5}    prefix=128    next_hop=${VETH_IP4}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns3    interface=ns3_veth3    routename=pingingns2_veth2    ip=${VETH_IP2}    prefix=128   next_hop=${VETH_IP4}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns3    interface=ns3_veth3    routename=pingingns1_veth1    ip=${VETH_IP1}    prefix=128    next_hop=${VETH_IP4}
    etcdctl.Put Linux Route    node=agent_vpp_1    namespace=ns2    interface=ns2_veth3    routename=pingingns3_veth3    ip=${VETH_IP7}    prefix=128    next_hop=${VETH_IP3}

    etcdctl.Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns3    name=ns3_veth3    host_if_name=ns3_veth3_linux    mac=92:c7:42:67:ab:ce    peer=ns2_veth3    ip=${VETH_IP3}
    etcdctl.Put Veth Interface Via Linux Plugin    node=agent_vpp_1    namespace=ns2    name=ns2_veth3    host_if_name=ns2_veth3_linux    mac=92:c7:42:67:ab:cf    peer=ns3_veth3    ip=${VETH_IP4}


    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces    node=agent_vpp_1    namespace=ns3    interface=ns3_veth3
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Interfaces    node=agent_vpp_1    namespace=ns2    interface=ns2_veth3

    Ping6 in namespace    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP3}
    Ping6 in namespace    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP4}

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP7}

    # tested also above, but repeat after giving exact routes
    Ping6 in namespace    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP4}
    Ping6 in namespace    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP3}
    # this works
    Ping6 in namespace    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP2}

    # this does not work
    # https://serverfault.com/questions/568839/linux-network-namespaces-ping-fails-on-specific-veth
    # https://unix.stackexchange.com/questions/391193/how-to-forward-traffic-between-linux-network-namespaces
    #Ping6 in namespace    node=agent_vpp_1    namespace=ns3    ip=192.168.22.1

    # routy sa zalozia po uspesnom pingu zo ns3 ?! or ping fails
    # Ping6 in namespace    node=agent_vpp_1    namespace=ns1    ip=192.169.22.3

Check linux Routes On VPP1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns1    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP6}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Gateway    node=agent_vpp_1    namespace=ns1    ip=${GOOGLE_IP}    next_hop=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Gateway    node=agent_vpp_1    namespace=ns1    ip=${QUAD9_IP}    next_hop=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP7}

Remove VPP Nodes
    Remove All Nodes
    Sleep    ${RESYNC_SLEEP}

Start VPP1 Again
    Add Agent VPP Node    agent_vpp_1
    Sleep    ${RESYNC_WAIT}

Check Linux Interfaces On VPP1 After Resync
    ${out}=    Execute In Container    agent_vpp_1    ip netns exec ns1 ip a
    Should Contain    ${out}    ns1_veth1_linux

    ${out}=    Execute In Container    agent_vpp_1    ip netns exec ns2 ip a
    Should Contain    ${out}    ns2_veth2_linux
    Should Contain    ${out}    ns2_veth3_linux

    ${out}=    Execute In Container    agent_vpp_1    ip netns exec ns3 ip a
    Should Contain    ${out}    ns3_veth3_linux

Check linux Routes On VPP1 After Resync
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns1    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP6}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Gateway    node=agent_vpp_1    namespace=ns1    ip=${GOOGLE_IP}    next_hop=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes Gateway    node=agent_vpp_1    namespace=ns1    ip=${QUAD9_IP}    next_hop=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns3    ip=${VETH_IP5}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    Check Linux Routes IPv6    node=agent_vpp_1    namespace=ns2    ip=${VETH_IP7}


*** Keywords ***
Check Linux Interfaces
    [Arguments]    ${node}    ${namespace}    ${interface}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 a
    Should Contain    ${out}    ${interface}

Check Linux Interfaces Link
    [Arguments]    ${node}    ${namespace}    ${interface}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 link
    Log    ${out}

Check Linux Routes
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip route show
    Should Contain    ${out}    ${ip} via

Check Linux Routes IPv6
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 route show
    Should Contain    ${out}    ${ip} via

Check Linux Routes Gateway
    [Arguments]    ${node}    ${namespace}    ${ip}    ${next_hop}=${EMPTY}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 route show
    Should Contain    ${out}    ${ip} via ${next_hop}

Check Linux Routes Metric
    [Arguments]    ${node}    ${namespace}    ${ip}    ${metric}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 route show
    Should Match Regexp    ${out}    ${ip} via.*metric ${metric}\\s

Check Removed Linux Route
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 route show
    Should Not Contain    ${out}    ${ip} via

Check Removed Linux Route IPv6
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ip -6 route show
    Should Not Contain    ${out}    ${ip} via

Ping in namespace
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ping -c 5 ${ip}
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss

Ping6 in namespace
    [Arguments]    ${node}    ${namespace}    ${ip}
    ${out}=    Execute In Container    ${node}    ip netns exec ${namespace} ping6 -c 5 ${ip}
    Should Contain     ${out}    from ${ip}
    Should Not Contain    ${out}    100% packet loss


*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
    
