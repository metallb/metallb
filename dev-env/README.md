# dev-env

This directory contains supporting files for the included MetalLB development
environment. The environment is run using the following command:

```
inv dev-env
```

There is also support for configuring MetalLB to peer with a BGP router running
in a container:

```
inv dev-env --protocol bgp
```

For more information, see [the BGP dev-env README](bgp/README.md).
