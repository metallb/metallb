## Development Docker Image

This image can be used to get started with the simple agent Go code. It contains:


- A pre-built Go agent.

### Getting the Image
You can either download a pre-built image (TODO: in progress) or build the image yourself on your local machine.

#### Building Locally
To build the image on your local machine,  type:
```
./build.sh
```
This will build dev_cn-infra image with default parameters:  
- simple-agent - latest commit number from the cloned repo,


To build specific commits (one or both), use `build.sh` with parameters:  
- `-a` or `--agent` to specify agent commit number,


Example:
```
./build.sh --agent 9c35e43e9bfad377f3c2186f30d9853e3f3db3ad
```

You can still build image using docker build command, but you must specify agent commit number:
```
sudo docker build --force-rm=true -t dev_cn_infra --build-arg AGENT_COMMIT=2c2b0df32201c9bc814a167e0318329c78165b5c --build-arg --no-cache .
```

#### Verifying a Created or Downloaded Image
You can verify the newly built or downloaded image as follows:

```
docker images
```

You should see something like this:

```
REPOSITORY                       TAG                 IMAGE ID            CREATED             SIZE
dev_cn_infra                    latest              0692f574f21a        11 minutes ago      1.5 GB
...
```
Get the details of the newly built image:

```
docker image inspect dev_cn_infra
docker image history dev_cn_infra
```


### Starting the Image

To start the image, type:
```
sudo docker run -it --name simple_agent --privileged --rm dev_cn_infra bash
```
To open another terminal:
```
sudo docker exec -it simple_agent bash
```

#### How to run examples
There are examples as an simple illustration of the cn-infra functionality.
You can find more info about how to run examples in [this README](../../examples/README.md) .
