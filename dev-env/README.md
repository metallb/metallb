# dev-env

This directory contains supporting files for the included MetalLB development
environment. The environment is run using:

```
inv dev-env
```

There is also support for configuring MetalLB to peer with a BGP router running
in a conatiner. For more information, see [the BGP dev-env
README](bgp/README.md).

```
inv dev-env --protocol bgp
```
