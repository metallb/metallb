# Virtuakube

Virtuakube sets up virtual Kubernetes clusters for testing.

![Project maturity: alpha](https://img.shields.io/badge/maturity-alpha-red.svg) [![license](https://img.shields.io/github/license/google/metallb.svg?maxAge=2592000)](https://github.com/danderson/virtuakube/blob/master/LICENSE) [![GoDoc](https://godoc.org/go.universe.tf/virtuakube?status.svg)](https://godoc.org/go.universe.tf/virtuakube)

It has several advantages compared to minikube or cloud clusters:

 - Support any number of nodes, limited only by system RAM.
 - Can run without root privileges (sort of - currently still requires
   docker privileges for image building).
 - Can run without internet access.
 - Because it emulates a full ethernet LAN, can be used to test
   networked systems.
 - After initial setup, can recreate a complex VM and network topology
   in <10s, ideal for running lots of unit tests.

It's a very young system, and is being built for the needs of testing
[MetalLB](https://metallb.universe.tf), but seems like it could be
generally useful for both playing with Kubernetes and testing
scenarios. However, as of now you should expect the API to change
frequently. Users and contributions are welcome, but be aware you're
using a very young piece of software.

Your host machine must have `qemu`, `qemu-img`, `vde_switch` and
`guestfish` installed.

## Example usage

The CLI under `cmd/vkube` is a quick way to get started. All resources
in virtuakube live in a `Universe`. Within a universe there are three
resources: `Image`s are base disk images for VMs, `VM`s are machines
that can talk to each other and the internet, and `Cluster`s are
Kubernetes clusters bootstrapped on VMs.

First, let's create a universe and build a VM base image inside it:

```
vkube newimage --universe ./my-test-universe --name base
```

This will create the `my-test-universe` directory to store universe
state, and build a VM base disk that contains Kubernetes tools and
prepulled control plane images. Building the image takes about 5
minutes.

Once we've done that, we can run a cluster in our universe:

```
vkube newcluster --universe ./my-test-universe --name example --image base
```

Bootstrapping a cluster takes a couple of minutes, but when done vkube
will print something like:

```
Created cluster "example"
Done (took 1m32s). Resources available:

  Cluster "example": export KUBECONFIG="/home/dave/my-test-universe/cluster/example/kubeconfig"
  VM "example-controller": ssh -p50000 root@localhost
  VM "example-node1": ssh -p50003 root@localhost

Hit ctrl+C to shut down
```

From there you can use the cluster as you wish, or SSH into VMs to do
more stuff. The VMs created here are ephemeral, so when you ctrl+C
they will be deleted, and you'll have to run `newcluster` again to
rebuild it.

If you want to suspend and resume your cluster instead of
creating/deleting, just add `--save` to the commandline. With
`--save`, all running VMs will be snapshotted to disk, and will all
resume as if nothing happened the next time the universe is opened. To
open a universe and resume whatever was saved, but without creating
new resources, use the `resume` command:

```
vkube resume --universe ./my-test-universe
```

Assuming you created and saved a cluster previously, you'll get
familiar output:

```
Done (took 791ms). Resources available:

  Cluster "example": export KUBECONFIG="/home/dave/my-test-universe/cluster/example/kubeconfig"
  VM "example-controller": ssh -p50000 root@localhost
  VM "example-node1": ssh -p50003 root@localhost

Hit ctrl+C to shut down
```

The cluster is back, but this time it took _less than a second_ to
come up (your mileage may vary, depending on disk and CPU
performance - but it should be much faster than creating VMs from
scratch).

All vkube commands accept `--save` to mean "resume from the current
state next time, instead of reverting to the last savepoint. Saving is
off by default for all commands except `newimage` (which is why the
base image we created stuck around - vkube implicitly saved the
universe after creating the image).

All vkube commands accept `--wait`. If `--wait` is true, vkube will
pause after the requested command has executed, print the available
resources (as above), and wait for ctrl+C before closing the universe
(with or without saving, depending on `--save`). Waiting is on by
default for all commands except `newimage`.

So, if you wanted to non-interactively create a universe, build a base
image, build a cluster and then immediately save it for future use,
you'd run:

```
vkube newimage --universe ./my-new-universe --name base
vkube newcluster --universe ./my-new-universe --name example --image base --save --wait=false
```

Then, later, play with your cluster:

```
vkube resume --universe ./my-new-universe
```
