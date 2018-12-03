[Documentation]     ENV specific configurations

*** Settings ***

*** Keywords ***

Configure Environment 1
    Add Agent VPP Node    agent_vpp_1
    Add Agent VPP Node    agent_vpp_2
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_2    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as

Configure Environment 2
    [Arguments]        ${sfc_conf}
    [Documentation]    Setup environment with sfc_setup
    Add Agent VPP Node   agent_vpp_1       vswitch=${TRUE}
    Add Agent Node        node_1
    Add Agent Node        node_2
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as
    Start SFC Controller Container With Own Config    ${sfc_conf}
    Sleep    ${SYNC_SLEEP}


Configure Environment 3
    Add Agent VPP Node         agent_vpp_1
    Add Agent VPP Node         agent_vpp_2
    Add Agent Libmemif Node    agent_libmemif_1
    Execute In Container       agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container       agent_vpp_1    ls -al
    Execute In Container       agent_vpp_2    echo $MICROSERVICE_LABEL
    Execute In Container       agent_vpp_2    ls -al
    Execute On Machine         docker    ${DOCKER_COMMAND} images
    Execute On Machine         docker    ${DOCKER_COMMAND} ps -as
    Sleep    ${SYNC_SLEEP}

Configure Environment 4
    [Arguments]        ${sfc_conf}
    [Documentation]    Setup environment with sfc_setup
    Add Agent VPP Node   agent_vpp_1       vswitch=${TRUE}
    Add Agent Node        node_1
    Add Agent Node        node_2
    Add Agent Node        node_3
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as
    Start SFC Controller Container With Own Config    ${sfc_conf}
    Sleep    ${SYNC_SLEEP}

Configure Environment 5
    Add Agent VPP Node    agent_vpp_1
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as

Configure Environment 6
    [Documentation]    Setup environment with 1 vpp and 2 non vpp nodes
    Add Agent VPP Node   agent_vpp_1       vswitch=${TRUE}
    Add Agent Node        node_1
    Add Agent Node        node_2
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as
    Start SFC Controller Container With Own Config    ${sfc_conf}
    Sleep    ${SYNC_SLEEP}

Configure Environment 7
    [Documentation]    Setup environment with 1 vpp and 3 vpp nodes (same as conf 4 but without sfc_setup)
    Add Agent VPP Node   agent_vpp_1       vswitch=${TRUE}
    Add Agent Node        node_1
    Add Agent Node        node_2
    Add Agent Node        node_3
    Execute In Container    agent_vpp_1    echo $MICROSERVICE_LABEL
    Execute In Container    agent_vpp_1    ls -al
    Execute On Machine    docker    ${DOCKER_COMMAND} images
    Execute On Machine    docker    ${DOCKER_COMMAND} ps -as
    Sleep    ${SYNC_SLEEP}