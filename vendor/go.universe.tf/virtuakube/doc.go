// Package virtuakube sets up virtual Kubernetes clusters for tests.
//
// The top-level object is a Universe. Everything else exists within a
// Universe, and is cleaned up when the Universe is closed. Within a
// Universe, you can create either bare VMs (using universe.NewVM), or
// Kubernetes clusters (using universe.NewCluster).
//
// Universes
//
// A universe requires a directory on disk to store its state. You get
// a universe either by calling Create to make a new blank one, or by
// calling Open to reuse an existing one.
//
// There are three ways of closing a universe. Calling Destroy wipes
// all the universe's state. Save snapshots and preserve all current
// resources, such that the next call to Open will resume the universe
// exactly as it was when last saved. Close preserves the universe,
// but rewinds its state to the last time it was saved, or to its
// pristine post-creation state if it was never saved.
//
// A typical use of virtuakube in tests is to do expensive setup ahead
// of time: use Create to make a new universe, then create and
// configure resources within, and finally call Save to snapshot
// it. Then, tests can simply Open this universe to get a fully
// working system immediately. When each test is done, they can Close
// the universe to undo all the changes that happened during the test,
// ready for the next test to Open.
//
// VMs
//
// VMs have a preset form that can be customized somewhat, but not
// fundamentally changed.
//
// Each VM has a single virtual disk, which is a copy-on-write fork of
// a base image. You can build base images with universe.NewImage.
//
// Each VM gets two network interfaces. The first provides internet
// access, and the second connects to a virtual LAN shared by all VMs
// in the same Universe.
//
// Network access to VMs from the host machine is only possible via
// port forwards, which are specified in the VM's configuration at
// creation time. Use vm.ForwardedPort to find out the local port to
// use to reach a given port on the VM.
//
// The VM type provides some helpers for running commands and
// reading/writing files on the VM.
//
// Kubernetes Clusters
//
// Clusters consist of one control plane VM, and a configurable number
// of additional worker VMs. Once created, the Cluster type has
// helpers to retrieve a kubeconfig file to talk to the cluster, a Go
// kubernetes client connected to the cluster, and the port to talk to
// the in-cluster registry to push images.
//
// VM Images
//
// VM images belong to a universe, and can be created with
// NewImage. NewImage accepts customization functions to do things
// like install extra packages, or configure services that all VMs
// using the image will need.
//
// If you customize your own VM image, it must conform to the
// following conventions for virtuakube to function correctly.
//
// The VM should autoconfigure the first network interface (ens3)
// using DHCP. Other ens* network interfaces should be left
// unconfigured. The `ip` tool must be installed so that virtuakube
// can configure those other interfaces.
//
// The VM should disable any kind of time synchronization, and rely
// purely on the VM's local virtual clock. VMs may spend hours or more
// suspended, and to avoid issues associated with timejumps on resume,
// virtuakube wants to maintain the illusion that no time has passed
// since suspend.
//
// If you want to use NewCluster with your VM image, the VM must have
// docker, kubectl and kubeadm preinstalled. Dependencies and
// prerequisites must be satisfied such that `kubeadm init` produces a
// working single-node cluster. Virtuakube includes stock
// customization functions to install Kubernetes prerequisites, and to
// pre-pull the Docker images for faster cluster startup when
// NewCluster is called.
package virtuakube // import "go.universe.tf/virtuakube"
