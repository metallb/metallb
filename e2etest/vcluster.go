package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	vk "go.universe.tf/virtuakube"
)

type cluster struct {
	universe *vk.Universe
	cluster  *vk.Cluster
	client   *vk.VM
}

func startCluster(ctx context.Context) (*cluster, error) {
	img, err := getVMImage()
	if err != nil {
		return nil, err
	}

	u, err := vk.New(ctx)
	if err != nil {
		return nil, err
	}

	ccfg := &vk.ClusterConfig{
		NumNodes: 1,
		VMConfig: &vk.VMConfig{
			Image:      img,
			MemoryMiB:  2048,
			CommandLog: os.Stdout,
		},
		NetworkAddon: "calico",
		ExtraAddons:  []string{},
	}
	c, err := u.NewCluster(ccfg)
	if err != nil {
		u.Close()
		return nil, err
	}
	if err := c.Start(); err != nil {
		u.Close()
		return nil, err
	}

	vcfg := &vk.VMConfig{
		Image:      img,
		Hostname:   "client",
		MemoryMiB:  1024,
		CommandLog: os.Stdout,
	}
	v, err := u.NewVM(vcfg)
	if err != nil {
		u.Close()
		return nil, err
	}
	if err := v.Start(); err != nil {
		u.Close()
		return nil, err
	}

	ret := &cluster{
		universe: u,
		cluster:  c,
		client:   v,
	}

	if err := ret.configureBird(); err != nil {
		u.Close()
		return nil, err
	}

	if err := ret.setupMetalLB(); err != nil {
		u.Close()
		return nil, err
	}

	return ret, nil
}

const (
	birdBaseCfg = `
router id %s;

protocol kernel {
  persist;
  merge paths;
  scan time 60;
  import none;
  export all;
}

protocol device {
  scan time 60;
}

protocol direct {
  interface "ens4";
  check link;
  import all;
}`
	birdPeerCfg = `
protocol bgp %s {
  local as 64513;
  neighbor %s as 64512;
  passive;
  import keep filtered;
  import all;
  export none;  
}`
)

func (c *cluster) configureBird() error {
	var bird4, bird6 bytes.Buffer
	fmt.Fprintf(&bird4, birdBaseCfg, c.client.IPv4())
	fmt.Fprintf(&bird6, birdBaseCfg, c.client.IPv4())

	fmt.Fprintf(&bird4, birdPeerCfg, "controller", c.cluster.Controller().IPv4())
	fmt.Fprintf(&bird6, birdPeerCfg, "controller", c.cluster.Controller().IPv6())

	for i, vm := range c.cluster.Nodes() {
		fmt.Fprintf(&bird4, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv4())
		fmt.Fprintf(&bird6, birdPeerCfg, "node"+strconv.Itoa(i), vm.IPv6())
	}

	if err := c.client.WriteFile("/etc/bird/bird.conf", bird4.Bytes()); err != nil {
		return err
	}
	if err := c.client.WriteFile("/etc/bird/bird6.conf", bird6.Bytes()); err != nil {
		return err
	}

	err := c.client.RunMultiple(
		"systemctl restart bird",
		"systemctl restart bird6",
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *cluster) setupMetalLB() error {
	cmd := exec.CommandContext(
		c.universe.Context(),
		"make", "push-images",
		"REGISTRY=localhost:"+strconv.Itoa(c.cluster.Registry()),
		"TAG=e2e",
		"ARCH=amd64",
	)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
