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

	ctrl, err := newController(myIP, *myNode, false)
	if err != nil {
		glog.Fatalf("Error getting controller: %s", err)
	}

	client, err := k8s.New("metallb-speaker", *master, *kubeconfig)
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}
	// Hacky: dispatch to both controllers for now.
	client.HandleServiceAndEndpoints(ctrl.SetBalancer)
	client.HandleConfig(ctrl.SetConfig)
	client.HandleLeadership(*myNode, ctrl.SetLeader)

	glog.Fatal(client.Run(*port))
}

type controller struct {
	myIP   net.IP
	myNode string

	config *config.Config
	ips    *allocator.Allocator

	protocols map[config.Proto]Protocol
	announced map[string]config.Proto // service name -> protocol advertising it
}

func newController(myIP net.IP, myNode string, noARP bool) (*controller, error) {
	var arpCtrl *arpController
	if !noARP {
		arpAnn, err := arp.New(myIP)
		if err != nil {
			return nil, fmt.Errorf("making ARP announcer: %s", err)
		}
		arpCtrl = &arpController{
			announcer: arpAnn,
		}
	}

	ret := &controller{
		myIP:   myIP,
		myNode: myNode,

		ips: allocator.New(),

		protocols: map[config.Proto]Protocol{
			config.ARP: arpCtrl,
			config.BGP: &bgpController{
				myIP:   myIP,
				svcAds: make(map[string][]*bgp.Advertisement),
			},
		},
		announced: map[string]config.Proto{},
	}

	return ret, nil
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

	poolName := c.ips.Pool(name)
	pool := c.config.Pools[c.ips.Pool(name)]
	if pool == nil {
		glog.Errorf("%s: could not find pool %q that definitely should exist!", name, poolName)
		return c.deleteBalancer(name, "can't find pool")
	}

	if proto, ok := c.announced[name]; ok && proto != pool.Protocol {
		if err := c.protocols[proto].DeleteBalancer(name, fmt.Sprintf("protocol changed to %q", pool.Protocol)); err != nil {
			return fmt.Errorf("deleting balancer %q from %q protocol: %s", name, proto, err)
		}
		delete(c.announced, name)
	}
	handler := c.protocols[pool.Protocol]
	if handler == nil {
		glog.Errorf("%s: unknown balancer protocol %q. This should not happen, please file a bug!", name, pool.Protocol)
		return c.deleteBalancer(name, "internal error (unknown balancer protocol)")
	}
	if err := handler.SetBalancer(name, lbIP, pool); err != nil {
		return err
	}

	c.announced[name] = pool.Protocol
	announcing.With(prometheus.Labels{
		"protocol": string(pool.Protocol),
		"service":  name,
		"node":     c.myNode,
		"ip":       lbIP.String(),
	}).Set(1)

	return nil
}

func (c *controller) deleteBalancer(name, reason string) error {
	if _, ok := c.announced[name]; !ok {
		return nil
	}

	glog.Infof("%s: stopping announcements, %s", name, reason)

	proto, ok := c.announced[name]
	if !ok {
		glog.Errorf("%s: unknown balancer protocol. This should not happen, please file a bug!", name)
		return errors.New("internal error (unknown balancer protocol)")
	}

	if err := c.protocols[proto].DeleteBalancer(name, reason); err != nil {
		return err
	}

	delete(c.announced, name)
	c.ips.Unassign(name)
	announcing.Delete(prometheus.Labels{
		"protocol": string(proto),
		"service":  name,
		"node":     c.myNode,
		"ip":       c.ips.IP(name).String(),
	})
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

	for proto, handler := range c.protocols {
		if err := handler.SetConfig(cfg); err != nil {
			glog.Errorf("Applying new configuration to protocol %q failed: %s", proto, err)
			return fmt.Errorf("configuration rejected: %s", err)
		}
	}

	c.config = cfg

	return nil
}

func (c *controller) SetLeader(isLeader bool) {
	for _, handler := range c.protocols {
		handler.SetLeader(isLeader)
	}
}

// A Protocol can advertise an IP address.
type Protocol interface {
	SetConfig(*config.Config) error
	SetBalancer(name string, lbIP net.IP, pool *config.Pool) error
	DeleteBalancer(name, reason string) error
	SetLeader(bool)
}
