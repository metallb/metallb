# Dev Docker Image

This image is used for development and debugging.

### What is included:
- **VPP-Agent** with default config files and ready to use
- **VPP** (compatible with VPP-Agent) including source code and build artifacts
- development environment with all requirements needed
  to build both the **VPP** and **VPP-Agent** itself
- code generation tools for **proto models** (gogoprotobuf) 
  and also **VPP binary API** using GoVPP generator)
- several other development utilities (agentctl, vpp-agent-ctl)

## Get the official image

Supported architectures are:
* AMD64 (a.k.a. x86_64)
* ARM64 (a.k.a. aarch64) - see [arm64 docker image][3].

Here follows the information related to AMD64 architecture:
For a quick start with the development image, you can download 
the [official image](https://hub.docker.com/r/ligato/dev-vpp-agent/) from **DockerHub**.

```sh
$ docker pull docker.io/ligato/dev-vpp-agent	# latest release (stable)
$ docker pull docker.io/ligato/dev-vpp-agent:pantheon-dev	# bleeding edge (unstable)
```

List of all available docker image tags for development image can 
be found [here](https://hub.docker.com/r/ligato/dev-vpp-agent/tags/).

## Building the image locally

To build the docker image on your local machine run:

```
$ ./build.sh
```

This will by default build image with name `dev_vpp_agent` 
and use following sources:
- **VPP-Agent** version from local repository
- **VPP** from repository and commit specified in `vpp.env` file

Note: The script build.sh will recognize the architecture (AMD64 or ARM64) and build the proper image.

### Pushing docker image to repository
To push the docker image into your repository, type:
```
REPO_OWNER=yourdockerhubreponame ./push_image.sh
```
Warning: use only IMMEDIATELY after docker/dev/build.sh to prevent INCONSISTENCIES such as:
* after building image you switch to other branch which will result in mismatch of version of image and its tag
* you do not build the new image but only simply run this script which will result in mismatch version of image and its tag because the image is older than repository

### Build VPP in _debug_ mode

By default the image builds **VPP** _.deb_ packages in **release** mode. 
You can change mode to **debug** by setting the environmental variable:

```
$ VPP_DEBUG_DEB=y ./build.sh
```

*Note:* This will only build the _.deb_ packages in **debug** mode. 
The running mode of the **VPP** can be changed to **debug** by 
setting `RUN_VPP_DEBUG=y` environmental variable for the container.

### Verifying the built image

You can verify the newly built image using:

```
$ docker images
``` 

You should see something like this:

```
REPOSITORY                       TAG                 IMAGE ID            CREATED             SIZE
dev_vpp_agent                    latest              0692f574f21a        11 minutes ago      3.58 GB
...
```

To inspect the details or see the history of the image use:

```
$ docker image inspect dev_vpp_agent
$ docker image history dev_vpp_agent
```

---

## Starting the Image

By default, the VPP & the Agent processes will be started automatically 
in the container. This is useful e.g. for deployments with Kubernetes, 
as described in [this README](../../k8s/dev-setup/README.md). However, this option is
not really required for development purposes, and it can be overridden by
specifying a different container entry point, e.g. bash, as shown below.

To start the image, type:
```
sudo docker run -it --name vpp_agent --privileged --rm dev_vpp_agent bash
```
To open another terminal into the image:
```
sudo docker exec -it vpp_agent bash
```

These environment variables can be set:
- `OMIT_AGENT` - whether the start of vpp-agent should be omitted (default is unset, agent will be started normally)
- `RETAIN_SUPERVISOR` - whether the supervisord should quit on unexpected exit of vpp or vpp-agent (default is unset, supervisord will quit)

## Running VPP in Debug or Release Mode

You can run VPP in debug or release mode. By default the image runs in release mode.
The environmental variable `RUN_VPP_DEBUG=y` can be used to run VPP in debug mode.

To start the image with VPP in debug mode:
```
sudo docker run -it --env RUN_VPP_DEBUG=y --rm --privileged dev_vpp_agent bash
```

## Mounting VPP Build Using Volume

You can run custom build of VPP by using volume mount. The VPP build in the image is located at `/opt/vpp-agent/dev/vpp`.

To start the image with custom VPP build mounted from host:
```
sudo docker run -it --volume $HOME/myvpp:/opt/vpp-agent/dev/vpp --rm --privileged dev_vpp_agent bash
```

## Running VPP and the Agent

**NOTE: The Agent will terminate if it cannot connect to VPP and to a Etcd
server. If Kafka config is specified, a successful connection to Kafka is
also required. If Kafka config is not specified, the Agent will run without
it, but all Kafka-related functionality will be disabled.** 

Start VPP in one of two modes: 
 - If you don't need (or don't have) DPDK, use "vpp lite":

```
vpp unix { interactive } plugins { plugin dpdk_plugin.so { disable } }
```
Note: you most likely do not have DPDK support if you're doing development
on your local laptop.

- If you want DPDK (with no PCI devices), use:

```
vpp unix { interactive } dpdk { no-pci }
```
Note that for DPDK, you would need to run the container in privileged mode
(add `--privileged` option to `docker run`). For more options, please refer
to [VPP documentation](https://wiki.fd.io/view/VPP/Command-line_Arguments).

To run the Agent, do the following:
- Edit `/opt/vpp-agent/dev/etcd.conf` to point the agent to an ETCD 
  server that runs outside of the VPP/Agent container. The
  default configuration is:

```
insecure-transport: true
dial-timeout: 1000000000
endpoints: 
 - "172.17.0.1:2379"
```
*Note that if you start Etcd in its own container on the same
host as the VPP/Agent container (as described below), you can
use the default endpoint configuration as is. ETCD is by default
mapped onto the host at port 2379; the host's IP address 
will typically be 172.17.0.1, unless you change your Docker 
networking settings.*

- Edit `/opt/vpp-agent/dev/kafka.conf` to point the agent to a Kafka broker.
  The default configuration is:

```
addrs:
 - "172.17.0.1:9092"
```

*Note that if you start Kafka in its own container on the same
host as the VPP/Agent container (as described below), you can
use the default broker address configuration as is. Kafka will
be mapped  onto the host at port 9092; the host's IP address 
will typically be 172.17.0.1, unless you change your Docker 
networking settings.*

- Start the Agent:
```
vpp-agent --etcd-config=/opt/vpp-agent/dev/etcd.conf --kafka-config=/opt/vpp-agent/dev/kafka.conf
```

## Running Etcd Server on Local Host

You can run an ETCD server in a separate container on your local
host as follows:
```
sudo docker run -p 2379:2379 --name etcd --rm \
    quay.io/coreos/etcd:v3.1.0 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```
The ETCD server will be available on your host OS IP (most likely 
`172.17.0.1` in the default docker environment) on port `2379`.

Call the agent via ETCD using the testing client:
```
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -tap
vpp-agent-ctl /opt/vpp-agent/dev/etcd.conf -tapd
```
Note: **For ARM64 see the information about [etcd][2]**.

## Running Kafka on Local Host

You can start Kafka in a separate container:
```
sudo docker run -p 2181:2181 -p 9092:9092 --name kafka --rm \
 --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 spotify/kafka
```
Note: **For ARM64 see the information about [kafka][1]**.

## Rebuilding the Agent
```
cd $GOPATH/src/github.com/ligato/vpp-agent/
git pull      # if needed
make
make test     # optional
make install
```

This should update the `agent` binary in yor `$GOPATH/bin` directory.

## Rebuilding of the container image:

Use the `--no-cache` flag for `docker build`:
```
sudo docker build --no-cache -t dev_vpp_agent .
```

## Mounting of a host directory into the Docker container:

Use `-v` option of the docker command:
```
sudo docker run -v /host/folder:/container/folder -it --name vpp_agent --privileged --rm dev_vpp_agent bash
```

E.g. if you have the vpp-agent code in `~/go/src/github.com/ligato/vpp-agent/` 
on your host OS, you can mount it as `/root/go/src/github.com/ligato/vpp-agent/` 
in the container as follows:
```
sudo docker run -v ~/go/src/github.com/ligato/vpp-agent/:/root/go/src/github.com/ligato/vpp-agent/ -it --name vpp_agent --rm dev_vpp_agent bash
```
Then you can modify the code on you host OS and us the container for 
building and testing it.

---

## Example: Using the Development Environment on a MacBook with Gogland

This section describes the setup of a lightweight, portable development
environment on your local notebook (MacBook or MacBook Pro in this example).
The MacBook will be the host for the Development Environment container 
and the folder containing the agent sources will be shared between the 
host and the container. Then, you can run the IDE, git, and other tools 
on the host and the compilations/testing in the container.

### Prerequisites

1. Get [Docker for Mac](https://docs.docker.com/docker-for-mac/). If you
   don't have it already installed, follow these
   [install instructions](https://docs.docker.com/docker-for-mac/install/).

2. Install Go and the [Gogland](https://www.jetbrains.com/go/) IDE. Go 
   can be downloaded from this [repository](https://golang.org/dl/), and 
   its install instructions are [here](https://golang.org/doc/install). 
   The download and install instructions for Gogland  are 
   [here](https://www.jetbrains.com/go/download/).   

2. Once you have Docker up & running on your Mac, build and verify the 
   Development Environment container [as described above](#getting-the-image).

### Building and Running the Agent

For the mixed host-container environment, the folder holding the 
Agent source code must be setup properly on both the host and in
 the container. You must also set GOPATH and GOROOT appropriately
in Gogland and in the development container. We will now walk you 
through these steps.

**On your Mac**:
- Create the Go home folder, for example `~/go/`

- Create the Go source folder in your Go home folder. The Go source
  folder must be called `src`. So you will have: ` ~/go/src`
  
- In the Go source folder, create the folder that will hold the 
  local clone of the vpp-agent repository. The path for the folder
  must reflect the import path in Go source code. So, the vpp-agent
  repository folder will be created as follows:
```
   cd  ~/go/src
   mkdir github.com
   mkdir github.com/ligato
```
  
- The agent repository is located at `https://github.com/ligato/vpp-agent`. 
  Go into your local vpp-agent repo folder and checkout the Agent code:
```
   cd github.com/ligato
   git clone https://github.com/ligato/vpp-agent.git 
```

- In Gogland, set `GOROOT` and `GOPATH` (`Preferences->Go->GOROOT`, `Preferences->Go->GOPATH`). Set `GOROOT` to where Go is installed (default: `/usr/local/go`) and the global `GOPATH` to the location of your Go home folder (`/Users/jmedved/Documents/Git/go-home/` in our example).

- Create a new project in Gogland (`File->New->Project`, ) will popup the 
  project creation window. Enter your newly created Go home folder 
  (`/Users/jmedved/Documents/Git/go-home/` in our example) as the location
  and accept the default value for the SDK. Click `Create`, and you 
  should now be able to browse the Agent source code in Gogland.

- Start the Development Environment container with the -v option, mounting
  the Go home folder in the container. With our example, and assuming that 
  we want to mount our Go home folder into the `root/go-home` folder, type:

```
   sudo docker run -v ~/go/src/github.com/ligato/vpp-agent/:/root/go/src/github.com/ligato/vpp-agent/ -it --name vpp_agent --rm dev_vpp_agent bash
```
The above command will put you into the Development Environment container 
console. 

**In the container console**:

- Setup the GOPATH and PATH variables in the Development Environment container:

```
   export GOPATH=/root/go
   export PATH=/$GOPATH/bin:$PATH
```
- Change directory into the vpp-agent folder and build & install the Agent:
```
   cd /root/go-home/src/github.com/ligato/vpp-agent
   make
   make install
```
 
- Use the newly built agent as described in Section
  '[Running VPP and the Agent](#running-vpp-and-the-agent)'.

[1]: ../../docs/arm64/kafka.md
[2]: ../../docs/arm64/etcd.md
[3]: ../../docs/arm64/docker_images.md
