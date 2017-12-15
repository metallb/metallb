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
	"errors"
	"flag"
	"fmt"
	"net"
	"os"

	"go.universe.tf/metallb/internal/allocator"
	"go.universe.tf/metallb/internal/arp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
)

type controller struct {
	myIP   net.IP
	myNode string

	config *config.Config
	ips    *allocator.Allocator
	ann    *arp.Announce

	// Metrics
	announcing *prometheus.GaugeVec
}

func (c *controller) SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc == nil {
		return c.deleteBalancer(name, "service deleted")
	}

	if svc.Spec.Type != "LoadBalancer" {
		return nil
	}

	glog.Infof("%s: start update", name)
	defer glog.Infof("%s: end update", name)

	if c.config == nil {
		glog.Infof("%s: skipped, waiting for config", name)
		return nil
	}

	if len(svc.Status.LoadBalancer.Ingress) != 1 {
		glog.Infof("%s: no IP allocated by controller", name)
		return c.deleteBalancer(name, "no IP allocated by controller")
	}

	// Should we advertise? Yes, if externalTrafficPolicy is Cluster,
	// or Local && there's a ready local endpoint.
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal && !k8s.NodeHasHealthyEndpoint(eps, c.myNode) {
		glog.Infof("%s: externalTrafficPolicy is Local, and no healthy local endpoints", name)
		return c.deleteBalancer(name, "no healthy local endpoints")
	}

	lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP).To4()
	if lbIP == nil {
		glog.Errorf("%s: invalid LoadBalancer IP %q", name, svc.Status.LoadBalancer.Ingress[0].IP)
		return c.deleteBalancer(name, "invalid IP allocated by controller")
	}

	if err := c.ips.Assign(name, lbIP); err != nil {
		glog.Errorf("%s: IP %q assigned by controller is not allowed by config", name, lbIP)
		return c.deleteBalancer(name, "invalid IP allocated by controller")
	}

	poolName := c.ips.GetPool(name)
	pool := c.config.Pools[c.ips.GetPool(name)]
	if pool == nil {
		glog.Errorf("%s: could not find pool %q that definitely should exist!", name, poolName)
		return c.deleteBalancer(name, "can't find pool")
	}

	glog.Infof("%s: announcable, making advertisement", name)

	c.ann.SetBalancer(name, lbIP)

	c.announcing.With(prometheus.Labels{
		"service": name,
		"node":    c.myNode,
		"ip":      lbIP.String(),
	}).Set(1)

	return nil
}

func (c *controller) deleteBalancer(name, reason string) error {
	if !c.ann.AnnounceName(name) {
		return nil
	}

	glog.Infof("%s: stopping announcements, %s", name, reason)
	c.announcing.Delete(prometheus.Labels{
		"service": name,
		"node":    c.myNode,
		"ip":      c.ips.GetIP(name).String(),
	})
	c.ips.Unassign(name)
	c.ann.DeleteBalancer(name)
	return nil
}

func (c *controller) SetConfig(cfg *config.Config) error {
	glog.Infof("Start config update")
	defer glog.Infof("End config update")

	if cfg == nil {
		glog.Errorf("No MetalLB configuration in cluster")
		return errors.New("configuration missing")
	}

	if err := c.ips.SetPools(cfg.Pools); err != nil {
		glog.Errorf("Applying new configuration failed: %s", err)
		return fmt.Errorf("configuration rejected: %s", err)
	}

	return nil
}

func (c *controller) MarkSynced() {}

func newController(myIP net.IP, myNode string) (*controller, error) {
	ann, err := arp.New(myIP)
	if err != nil {
		return nil, err
	}
	c := &controller{
		myIP:   myIP,
		myNode: myNode,
		ips:    allocator.New(),
		ann:    ann,

		announcing: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "metallb",
			Subsystem: "speaker",
			Name:      "announced",
			Help:      "Services being announced from this node. This is desired state, it does not guarantee that the routing protocols have converged.",
		}, []string{
			"service",
			"node",
			"ip",
		}),
	}

	prometheus.MustRegister(c.announcing)
	// just start this as a goroutine, the life time is bound to this process, so there is no need to stop it.
	go func() { ann.Run() }()
	return c, nil
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	master := flag.String("master", "", "master url")
	myIPstr := flag.String("node-ip", "", "IP address of this Kubernetes node")
	myNode := flag.String("node-name", "", "name of this Kubernetes node")
	port := flag.Int("port", 80, "HTTP listening port")
	flag.Parse()

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

	c, err := newController(myIP, *myNode)
	if err != nil {
		glog.Fatalf("Error getting controller: %s", err)
	}

	client, err := k8s.NewClient(identity, *master, *kubeconfig, c, true)
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	le, err := client.NewLeaderElector(c.ann, identity)
	if err != nil {
		glog.Fatalf("Error setting up leader election: %s", err)
	}
	le.Run()

	glog.Fatal(client.Run(*port))
}

const identity = "metallb-arp-speaker"
