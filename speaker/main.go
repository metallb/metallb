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

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/layer2"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/version"
	"k8s.io/api/core/v1"

	"github.com/go-kit/kit/log"
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

	logger, err := logging.Init()
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	var (
		myNode = flag.String("node-name", "", "name of this Kubernetes node")
		port   = flag.Int("port", 80, "HTTP listening port")
		config = flag.String("config", "config", "Kubernetes ConfigMap containing MetalLB's configuration")
	)
	flag.Parse()

	glog.Infof("MetalLB speaker %s", version.String())

	if *myNode == "" {
		*myNode = os.Getenv("METALLB_NODE_NAME")
	}

	if *myNode == "" {
		glog.Fatalf("Must specify --node-name")
	}

	// Setup all clients and speakers, config decides what is being done runtime.
	ctrl, err := newController(controllerConfig{
		MyNode: *myNode,
	})
	if err != nil {
		glog.Fatalf("Error getting controller: %s", err)
	}

	client, err := k8s.New(&k8s.Config{
		ProcessName:   "metallb-speaker",
		ConfigMapName: *config,
		NodeName:      *myNode,
		Logger:        logger,

		MetricsPort:   *port,
		ReadEndpoints: true,

		ServiceChanged: ctrl.SetBalancer,
		ConfigChanged:  ctrl.SetConfig,
		NodeChanged:    ctrl.SetNode,
		LeaderChanged:  ctrl.SetLeader,
	})
	if err != nil {
		glog.Fatalf("Error getting k8s client: %s", err)
	}

	glog.Fatal(client.Run())
}

type controller struct {
	myNode string

	config *config.Config

	protocols map[config.Proto]Protocol
	announced map[string]config.Proto // service name -> protocol advertising it
	svcIP     map[string]net.IP       // service name -> assigned IP
	ipRefcnt  map[string]int          // IP string -> number of consumers
}

type controllerConfig struct {
	MyNode string

	// For testing only, and will be removed in a future release.
	// See: https://github.com/google/metallb/issues/152.
	DisableLayer2 bool
}

func newController(cfg controllerConfig) (*controller, error) {
	protocols := map[config.Proto]Protocol{
		config.BGP: &bgpController{
			svcAds: make(map[string][]*bgp.Advertisement),
		},
	}

	if !cfg.DisableLayer2 {
		a, err := layer2.New()
		if err != nil {
			return nil, fmt.Errorf("making layer2 announcer: %s", err)
		}
		protocols[config.Layer2] = &layer2Controller{
			announcer: a,
		}
	}

	ret := &controller{
		myNode: cfg.MyNode,

		protocols: protocols,
		announced: map[string]config.Proto{},
		svcIP:     map[string]net.IP{},
		ipRefcnt:  map[string]int{},
	}

	return ret, nil
}

func (c *controller) SetBalancer(l log.Logger, name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc == nil {
		return c.deleteBalancer(l, name, "service deleted")
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
		return c.deleteBalancer(l, name, "no IP allocated by controller")
	}

	// Should we advertise? Yes, if externalTrafficPolicy is Cluster,
	// or Local && there's a ready local endpoint.
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal && !k8s.NodeHasHealthyEndpoint(eps, c.myNode) {
		glog.Infof("%s: externalTrafficPolicy is Local, and no healthy local endpoints", name)
		return c.deleteBalancer(l, name, "no healthy local endpoints")
	}

	lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
	if lbIP == nil {
		glog.Errorf("%s: invalid LoadBalancer IP %q", name, svc.Status.LoadBalancer.Ingress[0].IP)
		return c.deleteBalancer(l, name, "invalid IP allocated by controller")
	}

	poolName := poolFor(c.config.Pools, lbIP)
	if poolName == "" {
		glog.Errorf("%s: IP %q assigned by controller is not allowed by config", name, lbIP)
		return c.deleteBalancer(l, name, "invalid IP allocated by controller")
	}

	pool := c.config.Pools[poolName]
	if pool == nil {
		glog.Errorf("%s: could not find pool %q that definitely should exist!", name, poolName)
		return c.deleteBalancer(l, name, "can't find pool")
	}

	if proto, ok := c.announced[name]; ok && proto != pool.Protocol {
		if err := c.deleteBalancer(l, name, fmt.Sprintf("protocol changed to %q", pool.Protocol)); err != nil {
			return fmt.Errorf("deleting balancer %q: %s", name, err)
		}
	}

	handler := c.protocols[pool.Protocol]
	if handler == nil {
		glog.Errorf("%s: unknown balancer protocol %q. This should not happen, please file a bug!", name, pool.Protocol)
		return c.deleteBalancer(l, name, "internal error (unknown balancer protocol)")
	}
	if err := handler.SetBalancer(name, lbIP, pool); err != nil {
		return err
	}

	if c.announced[name] == "" {
		c.announced[name] = pool.Protocol
		c.svcIP[name] = lbIP
		c.ipRefcnt[lbIP.String()]++
	}

	announcing.With(prometheus.Labels{
		"protocol": string(pool.Protocol),
		"service":  name,
		"node":     c.myNode,
		"ip":       lbIP.String(),
	}).Set(1)
	glog.Infof("%s: announcing IP %s using protocol %q", name, lbIP, string(pool.Protocol))

	return nil
}

func (c *controller) deleteBalancer(l log.Logger, name, reason string) error {
	proto, ok := c.announced[name]
	if !ok {
		return nil
	}

	glog.Infof("%s: stopping announcements, %s", name, reason)

	c.ipRefcnt[c.svcIP[name].String()]--
	ref := c.ipRefcnt[c.svcIP[name].String()]
	announcing.Delete(prometheus.Labels{
		"protocol": string(proto),
		"service":  name,
		"node":     c.myNode,
		"ip":       c.svcIP[name].String(),
	})
	delete(c.announced, name)
	delete(c.svcIP, name)

	if ref == 0 {
		if err := c.protocols[proto].DeleteBalancer(name, reason); err != nil {
			return err
		}
	}

	return nil
}

func poolFor(pools map[string]*config.Pool, ip net.IP) string {
	for pname, p := range pools {
		for _, cidr := range p.CIDR {
			if cidr.Contains(ip) {
				return pname
			}
		}
	}
	return ""
}

func (c *controller) SetConfig(l log.Logger, cfg *config.Config) error {
	glog.Infof("Start config update")
	defer glog.Infof("End config update")

	if cfg == nil {
		glog.Errorf("No MetalLB configuration in cluster")
		return errors.New("configuration missing")
	}

	for svc, ip := range c.svcIP {
		if pool := poolFor(cfg.Pools, ip); pool == "" {
			glog.Errorf("Applying new configuration failed: service %q has no configuration under new config", svc)
			return fmt.Errorf("configuration rejected: service %q has no configuration under new config", svc)
		}
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

func (c *controller) SetLeader(l log.Logger, isLeader bool) {
	for _, handler := range c.protocols {
		handler.SetLeader(isLeader)
	}
}

func (c *controller) SetNode(l log.Logger, node *v1.Node) error {
	for proto, handler := range c.protocols {
		if err := handler.SetNode(node); err != nil {
			return fmt.Errorf("propagating node info to protocol %q: %s", proto, err)
		}
	}
	return nil
}

// A Protocol can advertise an IP address.
type Protocol interface {
	SetConfig(*config.Config) error
	SetBalancer(name string, lbIP net.IP, pool *config.Pool) error
	DeleteBalancer(name, reason string) error
	SetLeader(bool)
	SetNode(*v1.Node) error
}
