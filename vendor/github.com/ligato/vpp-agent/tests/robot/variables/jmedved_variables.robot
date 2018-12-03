*** Settings ***
Resource                          common_variables.robot

*** Variables ***
${DOCKER_HOST_IP}                 localhost
${DOCKER_HOST_USER}               localadmin
${DOCKER_HOST_PSWD}               cisco123
${DOCKER_COMMAND}                 docker
${DOCKER_PHYSICAL_INT_1}           0000:00:04.0
${DOCKER_PHYSICAL_INT_1_VPP_NAME}  GigabitEthernet0/4/0
${DOCKER_PHYSICAL_INT_1_MAC}       fa:16:3e:33:97:cb
${DOCKER_PHYSICAL_INT_2}           0000:00:05.0
${DOCKER_PHYSICAL_INT_2_VPP_NAME}  GigabitEthernet0/5/0
${DOCKER_PHYSICAL_INT_2_MAC}       fa:16:3e:90:dd:00

${K8_CLUSTER_INTEGRATION1_VM_1_PUBLIC_IP}   localhost
${K8_CLUSTER_INTEGRATION1_VM_1_LOCAL_IP}    localhost
${K8_CLUSTER_INTEGRATION1_VM_1_HOST_NAME}   ubuntu-slave-6
${K8_CLUSTER_INTEGRATION1_VM_1_USER}        localadmin
${K8_CLUSTER_INTEGRATION1_VM_1_PSWD}        cisco123
${K8_CLUSTER_INTEGRATION1_VM_1_PROMPT}      $
