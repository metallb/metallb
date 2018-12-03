*** Settings ***
Library      OperatingSystem
#Library      RequestsLibrary
#Library      SSHLibrary      timeout=60s
#Library      String

Resource     ../../variables/${VARIABLES}_variables.robot

Resource     ../../libraries/all_libs.robot

Force Tags        crud     IPv4
Suite Setup       Testsuite Setup
Suite Teardown    Testsuite Teardown
Test Setup        TestSetup
Test Teardown     TestTeardown

*** Variables ***
${VARIABLES}=          common
${ENV}=                common
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
# wait for resync vpps after restart
${RESYNC_WAIT}=        30s
@{segmentList1}    B::    C::    D::
@{segmentList2}    C::    D::    E::
@{segmentList1index0weight1}    0    1    @{segmentList1}    # segment list's index, weight and segments
@{segmentList2index0weight1}    0    2    @{segmentList2}    # segment list's index, weight and segments
@{segmentLists1}    ${segmentList1index0weight1}
@{segmentLists2}    ${segmentList2index0weight1}
#@{segmentLists12}    ${segmentList1}    ${segmentList1}

*** Test Cases ***
Configure Environment
    [Tags]    setup
    Add Agent VPP Node                     agent_vpp_1
    vpp_ctl: Put Veth Interface With IP    node=agent_vpp_1    name=vpp1_veth1        mac=12:11:11:11:11:11    peer=vpp1_veth2    ip=10.10.1.1
    vpp_ctl: Put Veth Interface            node=agent_vpp_1    name=vpp1_veth2        mac=12:12:12:12:12:12    peer=vpp1_veth1
    vpp_ctl: Put Afpacket Interface        node=agent_vpp_1    name=vpp1_afpacket1    mac=a2:a1:a1:a1:a1:a1    host_int=vpp1_veth2

Check Local SID CRUD
    vpp_ctl: Put Local SID                node=agent_vpp_1    localsidName=A    sidAddress=A::    fibtable=0    outinterface=vpp1_afpacket1    nexthop=A::1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Local SID Presence    node=agent_vpp_1    sidAddress=A::    interface=host-vpp1_veth2    nexthop=A::1
    vpp_ctl: Put Local SID                node=agent_vpp_1    localsidName=A    sidAddress=A::    fibtable=0    outinterface=vpp1_afpacket1    nexthop=C::1   #modification
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Local SID Presence    node=agent_vpp_1    sidAddress=A::    interface=host-vpp1_veth2    nexthop=C::1
    vpp_ctl: Delete Local SID             node=agent_vpp_1    localsidName=A
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Local SID Deleted     node=agent_vpp_1    sidAddress=A::

Check Policy and Policy Segment CRUD
    vpp_ctl: Put SRv6 Policy                    node=agent_vpp_1    name=AtoE            bsid=A::E          fibtable=0         srhEncapsulation=true      sprayBehaviour=true

    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Nonexistence    node=agent_vpp_1    bsid=A::E
    vpp_ctl: Put SRv6 Policy Segment            node=agent_vpp_1    name=firstSegment    policyName=AtoE    policyBSID=A::E    weight=1                   segmentlist=${segmentList1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0         behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists1}
    vpp_ctl: Delete SRv6 Policy Segment         node=agent_vpp_1    name=firstSegment    policyName=AtoE
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0         behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists1}    # special handling of empty policy (VPP doesn't allow this)
    vpp_ctl: Put SRv6 Policy Segment            node=agent_vpp_1    name=secondSegment   policyName=AtoE    policyBSID=A::E    weight=2                   segmentlist=${segmentList2}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0         behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists2}
    vpp_ctl: Delete SRv6 Policy Segment         node=agent_vpp_1    name=secondSegment   policyName=AtoE
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0         behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists2}    # special handling of empty policy (VPP doesn't allow this)
    vpp_ctl: Delete SRv6 Policy                 node=agent_vpp_1    name=AtoE
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Nonexistence    node=agent_vpp_1    bsid=A::E

Check Steering CRUD
    vpp_ctl: Put SRv6 Policy                    node=agent_vpp_1    name=AtoE            bsid=A::E            fibtable=0         srhEncapsulation=true    sprayBehaviour=true
    vpp_ctl: Put SRv6 Policy Segment            node=agent_vpp_1    name=firstSegment    policyName=AtoE      policyBSID=A::E    weight=1                 segmentlist=${segmentList1}
    vpp_ctl: Put SRv6 Steering                  node=agent_vpp_1    name=toE             bsid=A::E            fibtable=0         prefixAddress=B::/64
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Steering Presence      node=agent_vpp_1    bsid=A::E            prefixAddress=B::/64
    vpp_ctl: Put SRv6 Steering                  node=agent_vpp_1    name=toE             bsid=A::E            fibtable=0         prefixAddress=C::/64   # modification
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Steering Presence      node=agent_vpp_1    bsid=A::E            prefixAddress=C::/64
    vpp_ctl: Delete SRv6 Steering               node=agent_vpp_1    name=toE
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Steering NonExistence  node=agent_vpp_1    bsid=A::E            prefixAddress=B::/64
    vpp_ctl: Delete SRv6 Policy                 node=agent_vpp_1    name=AtoE   #cleanup

#TODO Steering can reference policy also by index -> add test (currently NOT WORKING on VPP side!)

Check delayed configuration
    vpp_ctl: Put SRv6 Steering                  node=agent_vpp_1    name=toE             bsid=A::E            fibtable=0               prefixAddress=E::/64
    Sleep                                       5s    # checking that vpp doesn't change (if previous command affects VPP it takes time to arrive in VPP )
    vpp_term: Check SRv6 Steering NonExistence  node=agent_vpp_1    bsid=A::E            prefixAddress=E::/64
    vpp_term: Check SRv6 Policy Nonexistence    node=agent_vpp_1    bsid=A::E
    vpp_ctl: Put SRv6 Policy                    node=agent_vpp_1    name=AtoE            bsid=A::E            fibtable=0         srhEncapsulation=true    sprayBehaviour=true
    Sleep                                       5s    # checking that vpp doesn't change (if previous command affects VPP it takes time to arrive in VPP )
    vpp_term: Check SRv6 Steering NonExistence  node=agent_vpp_1    bsid=A::E            prefixAddress=E::/64
    vpp_term: Check SRv6 Policy Nonexistence    node=agent_vpp_1    bsid=A::E
    vpp_ctl: Put SRv6 Policy Segment            node=agent_vpp_1    name=firstSegment    policyName=AtoE      policyBSID=A::E    weight=1                 segmentlist=${segmentList1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Steering Presence      node=agent_vpp_1    bsid=A::E            prefixAddress=E::/64
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0           behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists1}
    vpp_ctl: Delete SRv6 Steering               node=agent_vpp_1    name=toE             #cleanup
    vpp_ctl: Delete SRv6 Policy                 node=agent_vpp_1    name=AtoE            #cleanup

Check Resynchronization for clean VPP start
    vpp_ctl: Put Local SID                      node=agent_vpp_1    localsidName=A       sidAddress=A::               fibtable=0                 outinterface=vpp1_afpacket1    nexthop=A::1
    vpp_ctl: Put SRv6 Policy                    node=agent_vpp_1    name=AtoE            bsid=A::E                    fibtable=0                 srhEncapsulation=true    sprayBehaviour=true
    vpp_ctl: Put SRv6 Policy Segment            node=agent_vpp_1    name=firstSegment    policyName=AtoE              policyBSID=A::E            weight=1                 segmentlist=${segmentList1}
    vpp_ctl: Put SRv6 Steering                  node=agent_vpp_1    name=toE             bsid=A::E                    fibtable=0                 prefixAddress=E::/64
    Remove All VPP Nodes
    Sleep                                       3s
    Add Agent VPP Node                          agent_vpp_1
    Sleep                                       8s
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check Local SID Presence          node=agent_vpp_1    sidAddress=A::       interface=host-vpp1_veth2    nexthop=A::1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Policy Presence        node=agent_vpp_1    bsid=A::E            fibtable=0                   behaviour=Encapsulation    type=Spray    index=0    segmentlists=${segmentLists1}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Check SRv6 Steering Presence      node=agent_vpp_1    bsid=A::E            prefixAddress=E::/64

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown
