# VPP Agent on ARM64

###### Please note that the content of this repository is currently **WORK IN PROGRESS**.

The VPP Agent is successfully built also for ARM64 platform.
In this folder you find the documentation related to ARM64:
- Notes about [etcd on ARM6][3]
- Notes about [kafka on ARM64][2]
- Short description how to pull the [ARM64 images][4] from dockerhub and to start it. 

## Quickstart

For a quick start with the VPP Agent, you can use pre-built Docker images with
the Agent and VPP on [Dockerhub][1].

0. Start ETCD and Kafka on your host (e.g. in Docker as described [here][3] and [here][2] ).
   Note: **The Agent in the pre-built Docker image will not start if it can't 
   connect to both Etcd and Kafka**.

1. Run VPP + VPP Agent in a Docker image:
```
docker pull ligato/vpp-agent-arm64
docker run -it --name vpp --rm ligato/vpp-agent-arm64
```

2. Configure the VPP agent using agentctl:
```
docker exec -it vpp agentctl -h
```

3. Check the configuration (using agentctl or directly using VPP console):
```
docker exec -it vpp agentctl -e 172.17.0.1:2379 show
docker exec -it vpp vppctl -s localhost:5002
```

## Next Steps
Read the README for the [Development Docker Image](../../docker/dev/README.md) for more details.

[1]: https://hub.docker.com/r/ligato/vpp-agent-arm64/
[2]: kafka.md
[3]: etcd.md
[4]: docker_images.md
