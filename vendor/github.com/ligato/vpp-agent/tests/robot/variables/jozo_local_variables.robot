*** Settings ***
Resource                          common_variables.robot

*** Variables ***
${DOCKER_HOST_IP}                 192.168.100.20
${DOCKER_HOST_USER}               msestak
${DOCKER_HOST_PSWD}               Heslo9999
${AGENT_VPP_IMAGE_NAME}           ligato/vpp-agent:dev

${vpp1_DOCKER_IMAGE}              ${AGENT_VPP_IMAGE_NAME}
${vpp1_VPP_PORT}                  5002
${vpp1_VPP_HOST_PORT}             5004
${vpp1_SOCKET_FOLDER}             /tmp
${vpp1_VPP_TERM_PROMPT}           vpp#
${vpp1_VPP_VAT_PROMPT}            vat#
