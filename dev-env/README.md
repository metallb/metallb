# dev-env

This directory contains supporting files for the included MetalLB development
environment. The environment is run using the following command from the root of your git clone:

```
inv dev-env
```

For configuring MetalLB to peer with a BGP router running
in a container:

```
inv dev-env --protocol bgp
```

For more information, see [the BGP dev-env README](bgp/README.md).

You may also launch a dev environment with layer2, with:

```
inv dev-env --protocol layer2
```

## Requirements

* Go 1.15+
* Python 3
* [KIND - Kubernetes in Docker](https://kind.sigs.k8s.io/docs/user/quick-start/)
  * And optionally [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)

You may install the required python modules using the `requirements.txt` in this directory, with:

```
pip install -r requirements.txt
```


