*** Settings ***

Library      OperatingSystem
Library      String

Resource     ../../../variables/${VARIABLES}_variables.robot

Resource     ../../../libraries/all_libs.robot
Resource     ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv4
Suite Setup       Run Keywords    Discard old results

*** Variables ***
${VARIABLES}=          common
${ENV}=                common

*** Test Cases ***
Create agents in Bridge Domain with Memif interfaces and try traffic
    [Setup]     Test Setup
    [Teardown]  Test Teardown

    Add Agent VPP Node                 agent_vpp_1
    Add Agent VPP Node                 agent_vpp_2
    Create Loopback Interface bvi_loop0 On agent_vpp_1 With Ip 10.1.1.1/24 And Mac 8a:f1:be:90:00:00
    Create Master memif0 On agent_vpp_1 With MAC 02:f1:be:90:00:00, Key 1 And m1.sock Socket
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With Ip 10.1.1.2/24 And Mac 8a:f1:be:90:00:02
    Create Slave memif0 On agent_vpp_2 With MAC 02:f1:be:90:00:02, Key 1 And m1.sock Socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, memif0
    Create Bridge Domain bd1 With Autolearn On agent_vpp_2 With Interfaces bvi_loop0, memif0
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_2 To 10.1.1.1
    Add Agent VPP Node                 agent_vpp_3
    Create Master memif1 On agent_vpp_1 With MAC 02:f1:be:90:00:10, Key 2 And m2.sock Socket
    Create Loopback Interface bvi_loop0 On agent_vpp_3 With Ip 10.1.1.3/24 And Mac 8a:f1:be:90:00:03
    Create Slave memif0 On agent_vpp_3 With MAC 02:f1:be:90:00:03, Key 2 And m2.sock Socket
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, memif0, memif1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_3 With Interfaces bvi_loop0, memif0
    Ping From agent_vpp_2 To 10.1.1.3
    Ping From agent_vpp_3 To 10.1.1.2

First configure Bridge Domain with Memif interfaces and VXLan then add two agents and try traffic
    [Setup]     Test Setup
    [Teardown]  Test Teardown

    Create Master memif0 On agent_vpp_1 With IP 10.1.1.1, MAC 02:f1:be:90:00:00, Key 1 And m0.sock Socket
    Create Slave memif0 On agent_vpp_2 With IP 10.1.1.2, MAC 02:f1:be:90:00:02, Key 1 And m0.sock Socket
    Create Loopback Interface bvi_loop0 On agent_vpp_1 With Ip 20.1.1.1/24 And Mac 8a:f1:be:90:00:00
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With Ip 20.1.1.2/24 And Mac 8a:f1:be:90:00:02
    Create VXLan vxlan1 From 10.1.1.1 To 10.1.1.2 With Vni 13 On agent_vpp_1
    Create VXLan vxlan1 From 10.1.1.2 To 10.1.1.1 With Vni 13 On agent_vpp_2
    Create Bridge Domain bd1 With Autolearn On agent_vpp_1 With Interfaces bvi_loop0, vxlan1
    Create Bridge Domain bd1 With Autolearn On agent_vpp_2 With Interfaces bvi_loop0, vxlan1

    Add Agent VPP Node                 agent_vpp_1
    Add Agent VPP Node                 agent_vpp_2
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_2 To 10.1.1.1
    Ping From agent_vpp_1 To 20.1.1.2
    Ping From agent_vpp_2 To 20.1.1.1


Create Bridge Domain without autolearn
    [Setup]     Test Setup
    [Teardown]  Test Teardown

    Add Agent VPP Node                 agent_vpp_1
    Add Agent VPP Node                 agent_vpp_2
    # setup first agent
    Create Loopback Interface bvi_loop0 On agent_vpp_1 With Ip 10.1.1.1/24 And Mac 8a:f1:be:90:00:00
    Create Master memif0 On agent_vpp_1 With MAC 02:f1:be:90:00:00, Key 1 And m1.sock Socket
    Create Bridge Domain bd1 Without Autolearn On agent_vpp_1 With Interfaces bvi_loop0, memif0
    # setup second agent
    Create Loopback Interface bvi_loop0 On agent_vpp_2 With Ip 10.1.1.2/24 And Mac 8a:f1:be:90:00:02
    Create Slave memif0 On agent_vpp_2 With MAC 02:f1:be:90:00:02, Key 1 And m1.sock Socket
    Create Bridge Domain bd1 Without Autolearn On agent_vpp_2 With Interfaces bvi_loop0, memif0
    # without static fib entries ping should fail
    Command: Ping From agent_vpp_1 To 10.1.1.2 should fail
    Command: Ping From agent_vpp_2 To 10.1.1.1 should fail
    Add fib entry for 8a:f1:be:90:00:02 in bd1 over memif0 on agent_vpp_1
    Add fib entry for 02:f1:be:90:00:02 in bd1 over memif0 on agent_vpp_1
    Add fib entry for 8a:f1:be:90:00:00 in bd1 over memif0 on agent_vpp_2
    Add fib entry for 02:f1:be:90:00:00 in bd1 over memif0 on agent_vpp_2
    # and now ping must pass
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_2 To 10.1.1.1

*** Keywords ***