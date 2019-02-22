*** Settings ***

Library     OperatingSystem
Library     String
#Library     RequestsLibrary

Resource     ../../../variables/${VARIABLES}_variables.robot
Resource    ../../../libraries/all_libs.robot
Resource    ../../../libraries/pretty_keywords.robot

Force Tags        traffic     IPv4
Suite Setup       Run Keywords    Discard old results

*** Variables ***
${VARIABLES}=          common
${ENV}=                common

*** Test Cases ***
# Default VRF table ...
Start Three Agents And Then Configure
    [Setup]         Test Setup
    [Teardown]      Test Teardown
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3
    #setup one side with agent2
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip 10.1.1.1/24 and mac 8a:f1:be:90:00:00
    Create Master memif0 on agent_vpp_1 with MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_1 with interfaces bvi_loop0, memif0
    #setup second side with agent3
    Create loopback interface bvi_loop1 on agent_vpp_1 with ip 20.1.1.1/24 and mac 8a:f1:be:90:02:00
    Create Master memif1 on agent_vpp_1 with MAC 02:f1:be:90:02:00, key 2 and m1.sock socket
    Create bridge domain bd2 With Autolearn on agent_vpp_1 with interfaces bvi_loop1, memif1
    # prepare second agent
    Create loopback interface bvi_loop0 on agent_vpp_2 with ip 10.1.1.2/24 and mac 8a:f1:be:90:00:02
    Create Slave memif0 on agent_vpp_2 with MAC 02:f1:be:90:00:02, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_2 with interfaces bvi_loop0, memif0
    # prepare third agent
    Create loopback interface bvi_loop0 on agent_vpp_3 with ip 20.1.1.2/24 and mac 8a:f1:be:90:00:03
    Create Slave memif0 on agent_vpp_3 with MAC 02:f1:be:90:00:03, key 2 and m1.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_3 with interfaces bvi_loop0, memif0
    # setup routes
    Create Route On agent_vpp_2 With IP 20.1.1.0/24 With Next Hop 10.1.1.1 And Vrf Id 0
    Create Route On agent_vpp_3 With IP 10.1.1.0/24 With Next Hop 20.1.1.1 And Vrf Id 0

    Sleep    10

    # try ping
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_1 To 20.1.1.2
    Ping From agent_vpp_2 To 20.1.1.2
    Ping From agent_vpp_3 To 10.1.1.2

First Configure Three Agents And Then Start Agents
    [Setup]         Test Setup
    [Teardown]      Test Teardown
    #prepare first agent
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip 10.1.1.1/24 and mac 8a:f1:be:90:00:00
    Create Master memif0 on agent_vpp_1 with MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create loopback interface bvi_loop1 on agent_vpp_1 with ip 20.1.1.1/24 and mac 8a:f1:be:90:02:00
    Create Master memif1 on agent_vpp_1 with MAC 02:f1:be:90:02:00, key 2 and m1.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_1 with interfaces bvi_loop0, memif0
    Create bridge domain bd2 With Autolearn on agent_vpp_1 with interfaces bvi_loop1, memif1
    #prepare second agent
    Create loopback interface bvi_loop0 on agent_vpp_2 with ip 10.1.1.2/24 and mac 8a:f1:be:90:00:02
    Create Slave memif0 on agent_vpp_2 with MAC 02:f1:be:90:00:02, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_2 with interfaces bvi_loop0, memif0
    #prepare third agent
    Create loopback interface bvi_loop0 on agent_vpp_3 with ip 20.1.1.2/24 and mac 8a:f1:be:90:00:03
    Create Slave memif0 on agent_vpp_3 with MAC 02:f1:be:90:00:03, key 2 and m1.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_3 with interfaces bvi_loop0, memif0
    #setup routes
    Create Route On agent_vpp_2 With IP 20.1.1.0/24 With Next Hop 10.1.1.1 And Vrf Id 0
    Create Route On agent_vpp_3 With IP 10.1.1.0/24 With Next Hop 20.1.1.1 And Vrf Id 0
    #start agents
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3

    Sleep    10

    #check ping
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_1 To 20.1.1.2
    Ping From agent_vpp_2 To 20.1.1.2
    Ping From agent_vpp_3 To 10.1.1.2

# Non default VRF table 2 used in Agent VPP Node agent_vpp_2
Start Two Agents And Then Configure One With Non Default VRF
    [Setup]         Test Setup
    [Teardown]      Test Teardown
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Create Master memif0 on agent_vpp_1 with IP 10.1.1.1, MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create Slave memif0 on agent_vpp_2 with VRF 2, IP 10.1.1.2, MAC 02:f1:be:90:00:02, key 1 and m0.sock socket

    Sleep    14

    List of interfaces On agent_vpp_1 Should Contain Interface memif1/1
    List of interfaces On agent_vpp_2 Should Contain Interface memif1/1
    IP Fib Table 2 On agent_vpp_2 Should Contain Route With IP 10.1.1.2/32

    # try ping
    Ping From agent_vpp_1 To 10.1.1.2
    # this does not work for non default vrf: Ping From agent_vpp_2 To 10.1.1.1
    ${int}=    Get Interface Internal Name    agent_vpp_2    memif0
    Ping On agent_vpp_2 With IP 10.1.1.1, Source ${int}

# Non default VRF table 2 used in Agent VPP Node agent_vpp_2
# Non default VRF table 3 used in Agent VPP Node agent_vpp_3
Start Three Agents, Then Configure With Interfaces Assigned To Non Default VRF
    [Setup]         Test Setup
    [Teardown]      Test Teardown
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Add Agent VPP Node    agent_vpp_3
    #setup one side with agent2
    Create loopback interface bvi_loop0 on agent_vpp_1 with ip 10.1.1.1/24 and mac 8a:f1:be:90:00:00
    Create Master memif0 on agent_vpp_1 with MAC 02:f1:be:90:00:00, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_1 with interfaces bvi_loop0, memif0
    #setup second side with agent3
    Create loopback interface bvi_loop1 on agent_vpp_1 with ip 20.1.1.1/24 and mac 8a:f1:be:90:02:00
    Create Master memif1 on agent_vpp_1 with MAC 02:f1:be:90:02:00, key 2 and m1.sock socket
    Create bridge domain bd2 With Autolearn on agent_vpp_1 with interfaces bvi_loop1, memif1

    # prepare second agent
    Create loopback interface bvi_loop0 on agent_vpp_2 with VRF 2, ip 10.1.1.2/24 and mac 8a:f1:be:90:00:02
    Create Slave memif0 on agent_vpp_2 with MAC 02:f1:be:90:00:02, key 1 and m0.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_2 with interfaces bvi_loop0, memif0

    # prepare third agent
    Create loopback interface bvi_loop0 on agent_vpp_3 with VRF 3, ip 20.1.1.2/24 and mac 8a:f1:be:90:00:03
    Create Slave memif0 on agent_vpp_3 with MAC 02:f1:be:90:00:03, key 2 and m1.sock socket
    Create bridge domain bd1 With Autolearn on agent_vpp_3 with interfaces bvi_loop0, memif0

    # setup routes
    Create Route On agent_vpp_2 With IP 20.1.1.0/24 With Next Hop 10.1.1.1 And Vrf Id 2
    Create Route On agent_vpp_3 With IP 10.1.1.0/24 With Next Hop 20.1.1.1 And Vrf Id 3

    Sleep    10

    Show Interfaces On agent_vpp_1
    Show Interfaces Address On agent_vpp_2
    Show IP Fib On agent_vpp_2
    IP Fib Table 2 On agent_vpp_2 Should Contain Route With IP 20.1.1.0/24
    Show Interfaces Address On agent_vpp_3
    Show IP Fib On agent_vpp_3
    IP Fib Table 3 On agent_vpp_3 Should Contain Route With IP 10.1.1.0/24

    # try ping
    Ping From agent_vpp_1 To 10.1.1.2
    Ping From agent_vpp_1 To 20.1.1.2
    #Ping From agent_vpp_2 To 20.1.1.2
    ${int}=    Get Interface Internal Name    agent_vpp_2    bvi_loop0
    Ping On agent_vpp_2 With IP 20.1.1.2, Source ${int}
    #Ping From agent_vpp_3 To 10.1.1.2
    ${int}=    Get Interface Internal Name    agent_vpp_3    bvi_loop0
    Ping On agent_vpp_3 With IP 10.1.1.2, Source ${int}

*** Keywords ***
List of interfaces On ${node} Should Contain Interface ${int}
    ${out}=   vpp_term: Show Interfaces    ${node}
    Should Match Regexp        ${out}  ${int}