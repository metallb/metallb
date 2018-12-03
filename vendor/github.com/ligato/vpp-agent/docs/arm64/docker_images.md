## Get the official image for AMR64 platform

### Development images
For a quick start with the development image, you can download 
the [official image for ARM64 platform](https://hub.docker.com/r/ligato/dev-vpp-agent-arm64/) from **DockerHub**.

```sh
$ docker pull docker.io/ligato/dev-vpp-agent-arm64	# latest release (stable)
$ docker pull docker.io/ligato/dev-vpp-agent-arm64:pantheon-dev	# bleeding edge (unstable)
```

List of all available docker image tags for development image can 
be found [here for ARM64](https://hub.docker.com/r/ligato/dev-vpp-agent-arm64/tags/).

### Production images
For a quick start with the VPP Agent, you can use pre-build Docker images with
the Agent and VPP on Dockerhub:
the [official image for ARM64 platform](https://hub.docker.com/r/ligato/vpp-agent-arm64/).
```
docker pull ligato/vpp-agent-arm64
```

## Quick start command for arm64 docker image
```
docker pull ligato/vpp-agent-arm64
docker run -it --name vpp --rm ligato/vpp-agent-arm64
```
