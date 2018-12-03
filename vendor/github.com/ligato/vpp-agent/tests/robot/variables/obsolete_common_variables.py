DOCKER_HOST_IP = '192.168.1.67'
DOCKER_HOST_USER = 'frinx'
DOCKER_HOST_PSWD = 'frinx'
DOCKER_SOCKET_FOLDER = '/tmp/vpp_socket'
DOCKER_COMMAND = 'sudo docker'

ETCD_SERVER_CREATE = 'sudo docker run -p 2379:2379 --name etcd --rm quay.io/coreos/etcd:v3.0.16 /usr/local/bin/etcd -advertise-client-urls http://0.0.0.0:2379 -listen-client-urls http://0.0.0.0:2379'
ETCD_SERVER_DESTROY = 'sudo docker rm -f etcd'

KAFKA_SERVER_CREATE = 'sudo docker run -itd -p 2181:2181 -p 9092:9092  --env ADVERTISED_PORT=9092 --name kafka spotify/kafka'
KAFKA_SERVER_DESTROY = 'sudo docker rm -f kafka'

VPP_AGENT_CTL_IMAGE_NAME = 'containers.cisco.com/amarcine/prod_vpp_agent_shrink'
AGENT_VPP_IMAGE_NAME = 'containers.cisco.com/amarcine/prod_vpp_agent_shrink'
AGENT_VPP_ETCD_CONF_PATH = '/opt/vnf-agent/dev/etcd.conf'

AGENT_VPP_1_DOCKER_IMAGE = AGENT_VPP_IMAGE_NAME
AGENT_VPP_1_VPP_PORT = '5002'
AGENT_VPP_1_VPP_HOST_PORT = '5001'
AGENT_VPP_1_SOCKET_FOLDER = '/tmp'
AGENT_VPP_1_VPP_TERM_PROMPT = 'vpp#'
AGENT_VPP_1_VPP_VAT_PROMPT = 'vat#'

AGENT_VPP_2_DOCKER_IMAGE = AGENT_VPP_IMAGE_NAME
AGENT_VPP_2_VPP_PORT = '5002'
AGENT_VPP_2_VPP_HOST_PORT = '5002'
AGENT_VPP_2_SOCKET_FOLDER = '/tmp'
AGENT_VPP_2_VPP_TERM_PROMPT = 'vpp#'
AGENT_VPP_2_VPP_VAT_PROMPT = 'vat#'

AGENT_VPP_3_DOCKER_IMAGE = AGENT_VPP_IMAGE_NAME
AGENT_VPP_3_VPP_PORT = '5002'
AGENT_VPP_3_VPP_HOST_PORT = '5003'
AGENT_VPP_3_SOCKET_FOLDER = '/tmp'
AGENT_VPP_3_VPP_TERM_PROMPT = 'vpp#'
AGENT_VPP_3_VPP_VAT_PROMPT = 'vat#'


VAT_START_COMMAND = 'vpp_api_test json'
RESULTS_FOLDER = 'results'
TEST_DATA_FOLDER = 'test_data'
REST_CALL_SLEEP = '0'
SSH_READ_DELAY = '3'

EXAMPLE_PLUGIN_NAME = 'example_plugin.so'

# temporary vars
DEV_IMAGE = 'dev_vpp_agent'
