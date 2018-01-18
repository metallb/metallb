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
	"os"

	"go.universe.tf/metallb/internal/allocator"
	"go.universe.tf/metallb/internal/arp"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/version"
	"k8s.io/api/core/v1"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

var announcing = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "metallb",
	Subsystem: "speaker",
	Name:      "announced",
	Help:      "Services being announced from this node. This is desired state, it does not guarantee that the routing protocols have converged.",
}, []string{
	"service",
	"protocol",
	"node",
	"ip",
})

func main() {
	prometheus.MustRegister(announcing)

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	myIPstr := flag.String("node-ip", "", "IP address of this Kubernetes node")
	myNode := flag.String("node-name", "", "name of this Kubernetes node")
	port := flag.Int("port", 80, "HTTP listening port")
	flag.Parse()

	glog.Infof("MetalLB speaker %s", version.String())

	if *myIPstr == "" {
		*myIPstr = os.Getenv("METALLB_NODE_IP")
	}
	if *myNode == "" {
		*myNode = os.Getenv("METALLB_NODE_NAME")
	}

	myIP := net.ParseIP(*myIPstr).To4()
	if myIP == nil {
		glog.Fatalf("Invalid --node-ip %q, must be an IPv4 address", *myIPstr)
	}

	if *myNode == "" {
		glog.Fatalf("Must specify --node-name")
	}

	// Setup both ARP and BGP clients and speakers, config decides what is being done runtime.

	ctrl, err := newController(myIP, *myNode)
	if err != nil {
		glog.Fatalf("Error getting controller: %s", err)
	}

	client, err := k8s.New(speaker, *master, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}
	// Hacky: dispatch to both controllers for now.
	client.HandleServiceAndEndpoints(func(k string, svc *v1.Service, eps *v1.Endpoints) error {
		if err := ctrl.SetBalancerBGP(k, svc, eps); err != nil {
			return err
		}
		return ctrl.SetBalancerARP(k, svc, eps)
	})
	client.HandleConfig(func(cfg *config.Config) error {
		if err := ctrl.SetConfigBGP(cfg); err != nil {
			return err
		}
		return ctrl.SetConfigARP(cfg)
	})
	client.HandleLeadership(*myNode, ctrl.SetLeaderARP)

	glog.Fatal(client.Run(*port))
}

type controller struct {
	myIP   net.IP
	myNode string

	arpConfig *config.Config
	arpIPs    *allocator.Allocator
	arpAnn    *arp.Announce

	bgpConfig *config.Config
	bgpPeers  []*peer
	bgpSvcAds map[string][]*bgp.Advertisement
	bgpIPs    *allocator.Allocator
}

func newController(myIP net.IP, myNode string) (*controller, error) {
	arpAnn, err := arp.New(myIP)
	if err != nil {
		return nil, fmt.Errorf("making ARP announcer: %s", err)
	}

	ret := &controller{
		myIP:   myIP,
		myNode: myNode,

		arpIPs: allocator.New(),
		arpAnn: arpAnn,

		bgpSvcAds: map[string][]*bgp.Advertisement{},
		bgpIPs:    allocator.New(),
	}
	// just start this as a goroutine, the life time is bound to this process, so there is no need to stop it.
	go arpAnn.Run()

	return ret, nil
}

const speaker = "metallb-speaker"
