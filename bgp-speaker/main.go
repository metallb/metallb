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
	"net"
	"reflect"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
)

type service interface {
	Infof(svc *v1.Service, desc, msg string, args ...interface{})
	Errorf(svc *v1.Service, desc, msg string, args ...interface{})
}

type controller struct {
	client service
	config *config.Config
	peers  []*peer
}

type peer struct {
	cfg *config.Peer
	bgp *bgp.Session
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

	if len(svc.Status.LoadBalancer.Ingress) != 1 {
		glog.Infof("%q skipped, no IP allocated", name)
	}

	for _, p := range c.peers {
		err := p.bgp.Advertise(&bgp.Advertisement{
			Prefix: &net.IPNet{
				IP:   net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP),
				Mask: net.CIDRMask(32, 32),
			},
			NextHop:     net.ParseIP("192.168.16.42"),
			Communities: []uint32{1, 2, 3},
		})
		if err != nil {
			glog.Errorf("Advertising %q: %s", name, err)
		}
	}

	glog.Infof("TODO: do stuff with balancer")

	return nil
}

func (c *controller) deleteBalancer(name string) error {
	glog.Infof("TODO: delete balancer")
	return nil
}

func (c *controller) SetConfig(cfg *config.Config) error {
	glog.Infof("Converging configuration...")

	newPeers := make([]*peer, len(cfg.Peers))
	for i, p := range cfg.Peers {
		if i <= len(c.peers)-1 {
			if reflect.DeepEqual(p, c.peers[i].cfg) {
				newPeers[i] = c.peers[i]
				c.peers[i].bgp = nil
				continue
			}
		}

		newPeers[i] = &peer{
			cfg: &p,
		}
		glog.Infof("New BGP peer %#v", p)
	}

	c.config = cfg
	oldPeers := c.peers
	glog.Infof("oldPeers %#v", oldPeers)
	glog.Infof("newPeers %#v", newPeers)
	c.peers = newPeers
	for _, p := range oldPeers {
		if p.bgp == nil {
			continue
		}
		glog.Infof("CLOSE BGP %q", p.cfg.Addr)
		if err := p.bgp.Close(); err != nil {
			return fmt.Errorf("shutting down BGP session to %q: %s", p.cfg.Addr, err)
		}
	}

	for _, p := range c.peers {
		if p.bgp != nil {
			continue
		}

		s, err := mkBGP(p.cfg)
		if err != nil {
			return fmt.Errorf("creating BGP session for %q: %s", p.cfg.Addr, err)
		}
		p.bgp = s
	}

	glog.Infof("New config loaded")
	return nil
}

func mkBGP(cfg *config.Peer) (*bgp.Session, error) {
	// TODO: router ID
	s, err := bgp.New(fmt.Sprintf("%s:179", cfg.Addr), cfg.MyASN, net.ParseIP("192.168.18.65"), cfg.ASN, cfg.HoldTime)
	if err != nil {
		return nil, err
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
