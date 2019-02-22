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
${MEMIF11_MAC}=          1a:00:00:11:11:11
${MEMIF11_SEC_MAC}=      1a:00:00:11:11:12
${MEMIF21_MAC}=          2a:00:00:22:22:22
${MEMIF21_SEC_MAC}=      2a:00:00:22:22:23
${MEMIF12_MAC}=          3a:00:00:33:33:33
${MEMIF22_MAC}=          4a:00:00:44:44:44
${IP_1}=                 fd30::1:e:0:0:1
${IP_2}=                 fd30::1:e:0:0:2
${IP_3}=                 fd31::1:e:0:0:1
${IP_4}=                 fd31::1:e:0:0:2
${IP_5}=                 fd32::1:e:0:0:1
${IP_6}=                 fd32::1:e:0:0:2
${PREFIX}=               64
${WAIT_TIMEOUT}=     20s
${SYNC_SLEEP}=       3s
*** Test Cases ***
Configure Environment
    [Tags]    setup
    Configure Environment 1

Show Interfaces Before Setup
    vpp_term: Show Interfaces    agent_vpp_1

Add VPP1_memif1 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MEMIF11_MAC}
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=${MEMIF11_MAC}    master=true    id=1    ip=${IP_1}    prefix=64    socket=default.sock

Check That VPP1_memif1 Is Created But Not Connected
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MEMIF11_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_MAC}  role=master  id=1  ipv6=${IP_1}/64  connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Add VPP2_memif1 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_2    mac=${MEMIF21_MAC}
    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif1    mac=${MEMIF21_MAC}    master=false    id=1    ip=${IP_2}    prefix=64    socket=default.sock

Check That VPP2_memif1 Is Created And Connected With VPP1_memif1
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_2    mac=${MEMIF21_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_MAC}  role=slave  id=1  ipv6=${IP_2}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_MAC}  role=master  id=1  ipv6=${IP_1}/64  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Add VPP1_memif2 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_1    mac=${MEMIF12_MAC}
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif2    mac=${MEMIF12_MAC}    master=true    id=2    ip=${IP_3}    prefix=64    socket=default.sock

Check That VPP1_memif2 Is Created But Not Connected
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MEMIF12_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif2  mac=${MEMIF12_MAC}  role=master  id=2  ipv6=${IP_3}/64  connected=0  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Add VPP2_memif2 Interface
    vpp_term: Interface Not Exists    node=agent_vpp_2    mac=${MEMIF22_MAC}
    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif2    mac=${MEMIF22_MAC}    master=false    id=2    ip=${IP_4}    prefix=64    socket=default.sock

Check That VPP2_memif2 Is Created And Connected With VPP1_memif2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_2    mac=${MEMIF22_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif2  mac=${MEMIF22_MAC}  role=slave  id=2  ipv6=${IP_4}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif2  mac=${MEMIF12_MAC}  role=master  id=2  ipv6=${IP_3}/64  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP1_memif1 And VPP2_memif1 Interfaces Are Not Affected By VPP1_memif2 And VPP2_memif2 Interfaces
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_MAC}  role=master  id=1  ipv6=${IP_1}/64  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_MAC}  role=slave  id=1  ipv6=${IP_2}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Update VPP1_memif1 Interface
    Put Memif Interface With IP    node=agent_vpp_1    name=vpp1_memif1    mac=${MEMIF11_SEC_MAC}    master=true    id=1    ip=${IP_5}    prefix=30    socket=default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_1    mac=${MEMIF11_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_1    mac=${MEMIF11_SEC_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_SEC_MAC}  role=master  id=1  ipv6=${IP_5}/30  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP2_memif1 Is Still Configured And Connected
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_MAC}  role=slave  id=1  ipv6=${IP_2}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP1_memif2 And VPP2_memif2 Are Not Affected By VPP1_memif1 Update
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif2  mac=${MEMIF22_MAC}  role=slave  id=2  ipv6=${IP_4}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif2  mac=${MEMIF12_MAC}  role=master  id=2  ipv6=${IP_3}/64  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Update VPP2_memif1 Interface
    Put Memif Interface With IP    node=agent_vpp_2    name=vpp2_memif1    mac=${MEMIF21_SEC_MAC}    master=false    id=1    ip=${IP_6}    prefix=64    socket=default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_2    mac=${MEMIF21_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Created    node=agent_vpp_2    mac=${MEMIF21_SEC_MAC}
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_SEC_MAC}  role=slave  id=1  ipv6=${IP_6}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP1_memif1 Is Still Configured And Connected
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_SEC_MAC}  role=master  id=1  ipv6=${IP_5}/30  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP1_memif2 And VPP2_memif2 Are Not Affected By VPP2_memif1 Update
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif2  mac=${MEMIF22_MAC}  role=slave  id=2  ipv6=${IP_4}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif2  mac=${MEMIF12_MAC}  role=master  id=2  ipv6=${IP_3}/64  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock

Delete VPP1_memif2 Interface
    Delete VPP Interface    node=agent_vpp_1    name=vpp1_memif2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_1    mac=${MEMIF12_MAC}

Check That VPP2_memif2 Interface Is Disconnected
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif2  mac=${MEMIF22_MAC}  role=slave  id=2  ipv6=${IP_4}/64  connected=0  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Check That VPP1_memif1 And VPP2_memif1 Are Not Affected By VPP1_memif2 Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_SEC_MAC}  role=master  id=1  ipv6=${IP_5}/30  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_SEC_MAC}  role=slave  id=1  ipv6=${IP_6}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Delete VPP2_memif2 Interface
    Delete VPP Interface    node=agent_vpp_2    name=vpp2_memif2
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vpp_term: Interface Is Deleted    node=agent_vpp_2    mac=${MEMIF22_MAC}

Check That VPP1_memif1 And VPP2_memif1 Are Not Affected By VPP2_memif2 Delete
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_1  vpp1_memif1  mac=${MEMIF11_SEC_MAC}  role=master  id=1  ipv6=${IP_5}/30  connected=1  enabled=1  socket=${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}/default.sock
    Wait Until Keyword Succeeds   ${WAIT_TIMEOUT}   ${SYNC_SLEEP}    vat_term: Check Memif Interface State     agent_vpp_2  vpp2_memif1  mac=${MEMIF21_SEC_MAC}  role=slave  id=1  ipv6=${IP_6}/64  connected=1  enabled=1  socket=${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}/default.sock

Show Interfaces And Other Objects After Setup
    vpp_term: Show Interfaces    agent_vpp_1
    vpp_term: Show Interfaces    agent_vpp_2
    Write To Machine    agent_vpp_1_term    show int addr
    Write To Machine    agent_vpp_2_term    show int addr
    Write To Machine    agent_vpp_1_term    show h
    Write To Machine    agent_vpp_2_term    show h
    Write To Machine    agent_vpp_1_term    show memif
    Write To Machine    agent_vpp_2_term    show memif
    Write To Machine    agent_vpp_1_term    show err
    Write To Machine    agent_vpp_2_term    show err
    vat_term: Interfaces Dump    agent_vpp_1
    vat_term: Interfaces Dump    agent_vpp_2
    Execute In Container    agent_vpp_1    ip a
    Execute In Container    agent_vpp_2    ip a

*** Keywords ***
TestSetup
    Make Datastore Snapshots    ${TEST_NAME}_test_setup

TestTeardown
    Make Datastore Snapshots    ${TEST_NAME}_test_teardown

