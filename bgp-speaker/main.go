// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"reflect"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"

	"github.com/golang/glog"
	"github.com/kr/pretty"
	bgpconfig "github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/server"

	"k8s.io/api/core/v1"
)

type service interface {
	Infof(svc *v1.Service, desc, msg string, args ...interface{})
	Errorf(svc *v1.Service, desc, msg string, args ...interface{})
}

type controller struct {
	client service
	config *config.Config
	peers  []peer
}

type peer struct {
	cfg *config.Peer
	bgp *server.BgpServer
}

func (c *controller) SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc == nil {
		return c.deleteBalancer(name)
	}

	if svc.Spec.Type != "LoadBalancer" {
		return nil
	}

	if c.config == nil {
		// Config hasn't been read, nothing we can do just yet.
		glog.Infof("%q skipped, no config loaded", name)
		return nil
	}

	glog.Infof("TODO: do stuff with balancer")

	return nil
}

func (c *controller) deleteBalancer(name string) error {
	glog.Infof("TODO: delete balancer")
	return nil
}

func (c *controller) SetConfig(cfg *config.Config) error {
	pretty.Printf("Newconfig! %# v\n", cfg)

	newPeers := make([]peer, len(cfg.Peers))
	for i, p := range cfg.Peers {
		if i <= len(c.peers)-1 {
			if reflect.DeepEqual(p, c.peers[i].cfg) {
				newPeers[i] = c.peers[i]
				c.peers[i].bgp = nil
				continue
			}
		}

		newPeers[i].cfg = &p
		glog.Infof("New BGP peer %#v", p)
		s, err := mkBGP(p)
		if err != nil {
			return fmt.Errorf("creating BGP session for %q: %s", p.Addr, err)
		}
		newPeers[i].bgp = s
	}

	c.config = cfg
	oldPeers := c.peers
	c.peers = newPeers
	for _, p := range oldPeers {
		if p.bgp == nil {
			continue
		}
		if err := p.bgp.Stop(); err != nil {
			return fmt.Errorf("shutting down BGP session to %q: %s", p.cfg.Addr, err)
		}
	}
	glog.Infof("New config loaded")
	return nil
}

func mkBGP(cfg config.Peer) (*server.BgpServer, error) {
	s := server.NewBgpServer()
	go s.Serve()

	bgpCfg := &bgpconfig.Global{
		Config: bgpconfig.GlobalConfig{
			As:       cfg.MyASN,
			RouterId: "192.168.16.45", // TODO
			Port:     -1,              // Don't listen anywhere, we only connect out.
		},
	}
	if err := s.Start(bgpCfg); err != nil {
		return nil, fmt.Errorf("starting BGP server: %s", err)
	}

	n := &bgpconfig.Neighbor{
		Config: bgpconfig.NeighborConfig{
			NeighborAddress: cfg.Addr.String(),
			PeerAs:          cfg.ASN,
		},
		Timers: bgpconfig.Timers{
			Config: bgpconfig.TimersConfig{
				ConnectRetry:           1.0,
				IdleHoldTimeAfterReset: 1.0,
			},
		},
	}
	if err := s.AddNeighbor(n); err != nil {
		return nil, fmt.Errorf("adding neighbor: %s", err)
	}

	if err := s.EnableNeighbor(cfg.Addr.String()); err != nil {
		return nil, fmt.Errorf("starting neighbor: %s", err)
	}

	return s, nil
}

func (c *controller) MarkSynced() {}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	flag.Parse()

	c := &controller{}

	client, err := k8s.NewClient("metallb-bgp-speaker", *master, *kubeconfig, c, false)
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	c.client = client

	glog.Fatal(client.Run())
}
