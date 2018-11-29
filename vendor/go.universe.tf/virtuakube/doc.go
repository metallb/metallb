// Package virtuakube sets up virtual Kubernetes clusters for tests.
//
// The top-level object is a Universe. Everything else exists within a
// Universe, and is cleaned up when the Universe is closed. Within a
// Universe, you can create either bare VMs (using universe.NewVM), or
// Kubernetes clusters (using universe.NewCluster).
//
// VMs
//
// VMs have a preset form that can be customized somewhat, but not
// fundamentally changed.
//
// Each VM has a single virtual disk, which is a copy-on-write fork of
// a "backing" qemu qcow2 disk image. It is your responsibility to
// provide the backing image for your VMs. The "vmimg" subdirectory of
// this package's git repository contains a makefile that can
// construct such an image. If you bring your own image, it must
// conform to some conventions described in the "VM Backing Images"
// section below.
//
// In addition to its main disk, each VM gets access to a shared
// directory with the host. The host directory given by vm.Dir is
// mounted as "/host" inside the VM.
//
// Each VM gets two network interfaces. The first is a qemu user-mode
// network that provides internet access, and the second connects to a
// virtual LAN shared by all VMs in the same Universe. LAN IP
// addresses are ephemeral and non-deterministic, so VMs should
// resolve each others' hostnames to reliably communicate.
//
// Network access to VMs from the host machine is only possible via
// port forwards, which are specified in the VM's configuration at
// creation time. The vm.ForwardedPort method specifies the localhost
// port to connect to in order to reach a given port on the VM.
//
// If a boot script is provided in the VM configuration, it is
// executed during startup, after network configuration
// completes.
//
// Kubernetes Clusters
//
// Clusters consist of one control plane VM, and a configurable number
// of additional worker VMs. The control plane VM is not exclusive to
// the control plane, so a zero-worker cluster is fully functional.
//
// VM Backing Images
//
// If you bring your own VM backing image, it must conform to the
// following conventions for virtuakube to function correctly.
//
// The VM must mount the "host0" tag as "/host", using the virtio 9p
// transport.
//
// The VM should autoconfigure the network interface with MAC address
// 52:54:00:12:34:56 using DHCP, and set the machine hostname to the
// hostname provided by the DHCP server. The other network interface,
// which has a randomized MAC address, should be configured with the
// IPs listed in /host/ip, using /24 netmasks.
//
// If the file /host/bootscript.sh is present, the VM should execute
// it after the network has been configured. Once both network
// configuration and bootscript execution (if applicable) is complete,
// the VM should create /host/boot-done to signal to the host that the
// VM is ready for use.
//
// If you want to use NewCluster with your VM image, the VM must have
// docker, kubectl and kubeadm preinstalled. Dependencies and
// prerequisites must be satisfied such that `kubeadm init` produces a
// working single-node cluster.
package virtuakube // import "go.universe.tf/virtuakube"
