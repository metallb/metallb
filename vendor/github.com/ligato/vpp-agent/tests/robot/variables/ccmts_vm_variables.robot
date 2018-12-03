*** Settings ***
Resource                          common_variables.robot

*** Variables ***
${DOCKER_HOST_IP}                 172.22.127.216
${DOCKER_HOST_USER}               jenkins_ccmts
${DOCKER_HOST_PSWD}               rsa_id

${AGENT_VPP_IMAGE_NAME}           ligato/vpp-agent:pantheon-dev
${VNF_IMAGE_NAME}                 ligato/vpp-agent:pantheon-dev
${SFC_CONTROLLER_IMAGE_NAME}      ligato/dev_sfc_controller:latest
