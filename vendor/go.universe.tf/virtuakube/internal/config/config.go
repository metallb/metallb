package config

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"time"
)

type Universe struct {
	Snapshots map[string]*Snapshot
}

type Snapshot struct {
	Name     string
	ID       string
	NextPort int
	NextNet  int
	Clock    time.Time

	Networks map[string]*Network
	Images   map[string]*Image
	VMs      map[string]*VM
	Clusters map[string]*Cluster
}

type Network struct {
	Name     string
	NextIPv4 net.IP
	NextIPv6 net.IP
}

type Image struct {
	Name string
	File string
}

type VM struct {
	Name         string
	DiskFile     string
	MemoryMiB    int
	PortForwards map[int]int
	Networks     []string
	MAC          map[string]string // network name -> MAC in that network
	IPv4         map[string]net.IP // network name -> IP in that network
	IPv6         map[string]net.IP
}

type Cluster struct {
	Name       string
	NumNodes   int
	Kubeconfig []byte
}

func Read(path string) (*Universe, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ret Universe
	if err := json.Unmarshal(bs, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func Write(path string, cfg *Universe) error {
	bs, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path, bs, 0600); err != nil {
		return err
	}
	return nil
}
