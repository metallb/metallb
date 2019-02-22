*** Variables ***
${DOCKER_HOST_IP}                  192.168.1.67
${DOCKER_HOST_USER}                frinx
${DOCKER_HOST_PSWD}                frinx
${DOCKER_SOCKET_FOLDER}            /tmp/vpp_socket
${DOCKER_WORKDIR}                  /tmp
${DOCKER_COMMAND}                  sudo docker
${DOCKER_PHYSICAL_INT_1}           0000:00:09.0
${DOCKER_PHYSICAL_INT_1_VPP_NAME}  GigabitEthernet0/9/0
${DOCKER_PHYSICAL_INT_1_MAC}       08:00:27:0e:22:53
${DOCKER_PHYSICAL_INT_2}           0000:00:0a.0
${DOCKER_PHYSICAL_INT_2_VPP_NAME}  GigabitEthernet0/a/0
${DOCKER_PHYSICAL_INT_2_MAC}       08:00:27:e2:08:b9

${ETCD_SERVER_CREATE}              ${DOCKER_COMMAND} create -p 2379:2379 --name etcd -e ETCDCTL_API=3 quay.io/coreos/etcd:v3.0.16 /usr/local/bin/etcd -advertise-client-urls http://0.0.0.0:2379 -listen-client-urls http://0.0.0.0:2379
${ETCD_SERVER_DESTROY}             ${DOCKER_COMMAND} rm -f etcd

${KAFKA_SERVER_CREATE}             ${DOCKER_COMMAND} create -it -p 2181:2181 -p 9092:9092 --env ADVERTISED_PORT=9092 --name kafka spotify/kafka
${KAFKA_SERVER_DESTROY}            ${DOCKER_COMMAND} rm -f kafka

#${SFC_CONTROLLER_IMAGE_NAME}       containers.cisco.com/odpm_jenkins_gen/dev_sfc_controller:master
${SFC_CONTROLLER_IMAGE_NAME}       ligato/dev_sfc_controller
${SFC_CONTROLLER_CONF_PATH}        /opt/sfc-controller/dev/sfc.conf

# Variables for container with agent and VPP
${AGENT_VPP_IMAGE_NAME}            prod_vpp_agent
#${AGENT_VPP_ETCD_CONF_PATH}        /opt/vnf-agent/dev/etcd.conf
#${AGENT_VPP_KAFKA_CONF_PATH}       /opt/vnf-agent/dev/kafka.conf
${AGENT_VPP_ETCD_CONF_PATH}        /opt/vpp-agent/dev/etcd.conf
${AGENT_VPP_KAFKA_CONF_PATH}       /opt/vpp-agent/dev/kafka.conf
${VPP_AGENT_CTL_IMAGE_NAME}        ${AGENT_VPP_IMAGE_NAME}
${VPP_AGENT_HOST_MEMIF_SOCKET_FOLDER}    /run/vpp

${VPP_CONF_PATH}                   /etc/vpp/vpp.conf

${AGENT_VPP_1_DOCKER_IMAGE}        ${AGENT_VPP_IMAGE_NAME}
${AGENT_VPP_1_VPP_PORT}            5002
${AGENT_VPP_1_VPP_HOST_PORT}       5001
${AGENT_VPP_1_REST_API_PORT}       9191
${AGENT_VPP_1_REST_API_HOST_PORT}  9191
${AGENT_VPP_1_SOCKET_FOLDER}       /tmp
${AGENT_VPP_1_MEMIF_SOCKET_FOLDER}    /run/vpp
${AGENT_VPP_1_VPP_TERM_PROMPT}     vpp#
${AGENT_VPP_1_VPP_VAT_PROMPT}      vat#

${AGENT_VPP_2_DOCKER_IMAGE}        ${AGENT_VPP_IMAGE_NAME}
${AGENT_VPP_2_VPP_PORT}            5002
${AGENT_VPP_2_VPP_HOST_PORT}       5002
${AGENT_VPP_2_REST_API_PORT}       9191
${AGENT_VPP_2_REST_API_HOST_PORT}  9192
${AGENT_VPP_2_SOCKET_FOLDER}       /tmp
${AGENT_VPP_2_MEMIF_SOCKET_FOLDER}    /run/vpp
${AGENT_VPP_2_VPP_TERM_PROMPT}     vpp#
${AGENT_VPP_2_VPP_VAT_PROMPT}      vat#

${AGENT_VPP_3_DOCKER_IMAGE}        ${AGENT_VPP_IMAGE_NAME}
${AGENT_VPP_3_VPP_PORT}            5002
${AGENT_VPP_3_VPP_HOST_PORT}       5003
${AGENT_VPP_3_REST_API_PORT}       9191
${AGENT_VPP_3_REST_API_HOST_PORT}  9193
${AGENT_VPP_3_SOCKET_FOLDER}       /tmp
${AGENT_VPP_3_MEMIF_SOCKET_FOLDER}    /run/vpp
${AGENT_VPP_3_VPP_TERM_PROMPT}     vpp#
${AGENT_VPP_3_VPP_VAT_PROMPT}      vat#

${AGENT_VPP_4_DOCKER_IMAGE}        ${AGENT_VPP_IMAGE_NAME}
${AGENT_VPP_4_VPP_PORT}            5002
${AGENT_VPP_4_VPP_HOST_PORT}       5004
${AGENT_VPP_4_REST_API_PORT}       9191
${AGENT_VPP_4_REST_API_HOST_PORT}  9194
${AGENT_VPP_4_SOCKET_FOLDER}       /tmp
${AGENT_VPP_4_MEMIF_SOCKET_FOLDER}    /run/vpp
${AGENT_VPP_4_VPP_TERM_PROMPT}     vpp#
${AGENT_VPP_4_VPP_VAT_PROMPT}      vat#

# Variables for container with agent and without vpp
#${AGENT_IMAGE_NAME}                ligato/dev-cn-infra:latest
${AGENT_IMAGE_NAME}                ligato/vpp-agent:dev
${AGENT_ETCD_CONF_PATH}            /opt/vpp-agent/dev/etcd.conf
${AGENT_KAFKA_CONF_PATH}           /opt/vpp-agent/dev/kafka.conf

${AGENT_1_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${AGENT_1_REST_API_PORT}           9191
${AGENT_1_REST_API_HOST_PORT}      9195

${AGENT_2_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${AGENT_2_REST_API_PORT}           9191
${AGENT_2_REST_API_HOST_PORT}      9196

${AGENT_3_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${AGENT_3_REST_API_PORT}           9191
${AGENT_3_REST_API_HOST_PORT}      9197

${NODE_1_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${NODE_1_REST_API_PORT}           9191
${NODE_1_REST_API_HOST_PORT}      9195

${NODE_2_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${NODE_2_REST_API_PORT}           9191
${NODE_2_REST_API_HOST_PORT}      9196

${NODE_3_DOCKER_IMAGE}            ${AGENT_IMAGE_NAME}
${NODE_3_REST_API_PORT}           9191
${NODE_3_REST_API_HOST_PORT}      9197

${NODE_1_AGAIN_REST_API_PORT}     9191
${NODE_1_AGAIN_REST_API_HOST_PORT}  9198

# Variables for container with libmemif without vpp
${AGENT_LIBMEMIF_IMAGE_NAME}       ligato/libmemif-sample-service:test


${AGENT_LIBMEMIF_1_DOCKER_IMAGE}   ${AGENT_LIBMEMIF_IMAGE_NAME}
${AGENT_LIBMEMIF_1_MEMIF_SOCKET_FOLDER}    /run/vpp

${AGENT_LIBMEMIF_2_DOCKER_IMAGE}   ${AGENT_LIBMEMIF_IMAGE_NAME}
${AGENT_LIBMEMIF_2_MEMIF_SOCKET_FOLDER}    /run/vpp

${AGENT_LIBMEMIF_3_DOCKER_IMAGE}   ${AGENT_LIBMEMIF_IMAGE_NAME}
${AGENT_LIBMEMIF_3_MEMIF_SOCKET_FOLDER}    /run/vpp

${AGENT_LIBMEMIF_4_DOCKER_IMAGE}   ${AGENT_LIBMEMIF_IMAGE_NAME}
${AGENT_LIBMEMIF_4_MEMIF_SOCKET_FOLDER}    /run/vpp


# Other variables
${VAT_START_COMMAND}               vpp_api_test json
${RESULTS_FOLDER}                  results
${RESULTS_FOLDER_SUITE}            ${RESULTS_FOLDER}/${SUITE_NAME}
${TEST_DATA_FOLDER}                test_data
${REST_CALL_SLEEP}                 0
${SSH_READ_DELAY}                  1

${EXAMPLE_PLUGIN_NAME}             example_plugin.so

${AGENT_VER}                       v2

# temporary vars
${DEV_IMAGE}                       dev_vpp_agent

# Integration test VM 1 node BASIC
${K8_CLUSTER_INTEGRATION1_NODES}            1
${K8_CLUSTER_INTEGRATION1_VM_1_PUBLIC_IP}   localhost
${K8_CLUSTER_INTEGRATION1_VM_1_LOCAL_IP}    localhost
${K8_CLUSTER_INTEGRATION1_VM_1_HOST_NAME}   dp-profile-master-01
${K8_CLUSTER_INTEGRATION1_VM_1_USER}        jenkins_ccmts
${K8_CLUSTER_INTEGRATION1_VM_1_PSWD}        rsa_id
${K8_CLUSTER_INTEGRATION1_VM_1_PROMPT}      $
${K8_CLUSTER_INTEGRATION1_DOCKER_COMMAND}   docker
${K8_GENERATED_CONFIG_FOLDER}         generated_config

# Can use separate image for vswitch and VNF agents
${VNF_IMAGE_NAME}                 ligato/vpp-agent:pantheon-dev

${SSH_TIMEOUT}                        30
${SSH_LOG_OUTPUTS_TIMEOUT}            30
${POD_DEPLOY_TIMEOUT}                 60
${POD_DEPLOY_MULTIREPLICA_TIMEOUT}    600