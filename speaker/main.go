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

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/layer2"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/version"
	"k8s.io/api/core/v1"

	"github.com/go-kit/kit/log"
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

	logger.Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "msg", "MetalLB speaker starting "+version.String())

	if *myNode == "" {
		*myNode = os.Getenv("METALLB_NODE_NAME")
	}

	if *myNode == "" {
		logger.Log("op", "startup", "error", "must specify --node-name", "msg", "missing configuration flag")
		os.Exit(1)
	}

	// Setup all clients and speakers, config decides what is being done runtime.
	ctrl, err := newController(controllerConfig{
		MyNode: *myNode,
		Logger: logger,
	})
	if err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to create MetalLB controller")
		os.Exit(1)
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
	})
	if err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to create k8s client")
	}

	if err := client.Run(); err != nil {
		logger.Log("op", "startup", "error", err, "msg", "failed to run k8s client")
	}
}

type controller struct {
	myNode string

	config *config.Config

	protocols map[config.Proto]Protocol
	announced map[string]config.Proto // service name -> protocol advertising it
	svcIP     map[string]net.IP       // service name -> assigned IP
}

type controllerConfig struct {
	MyNode string
	Logger log.Logger

	// For testing only, and will be removed in a future release.
	// See: https://github.com/google/metallb/issues/152.
	DisableLayer2 bool
}

func newController(cfg controllerConfig) (*controller, error) {
	protocols := map[config.Proto]Protocol{
		config.BGP: &bgpController{
			logger: cfg.Logger,
			myNode: cfg.MyNode,
			svcAds: make(map[string][]*bgp.Advertisement),
		},
	}

	if !cfg.DisableLayer2 {
		a, err := layer2.New(cfg.Logger)
		if err != nil {
			return nil, fmt.Errorf("making layer2 announcer: %s", err)
		}
		protocols[config.Layer2] = &layer2Controller{
			announcer: a,
			myNode:    cfg.MyNode,
		}
	}

	ret := &controller{
		protocols: protocols,
		announced: map[string]config.Proto{},
		svcIP:     map[string]net.IP{},
	}

	return ret, nil
}

func (c *controller) SetBalancer(l log.Logger, name string, svc *v1.Service, eps *v1.Endpoints) bool {
	if svc == nil {
		return c.deleteBalancer(l, name, "serviceDeleted")
	}

	l.Log("event", "startUpdate", "msg", "start of service update")
	defer l.Log("event", "endUpdate", "msg", "end of service update")

	if svc.Spec.Type != "LoadBalancer" {
		return c.deleteBalancer(l, name, "notLoadBalancer")
	}

	if c.config == nil {
		l.Log("event", "noConfig", "msg", "not processing, still waiting for config")
		return true
	}

	if len(svc.Status.LoadBalancer.Ingress) != 1 {
		return c.deleteBalancer(l, name, "noIPAllocated")
	}

	lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
	if lbIP == nil {
		l.Log("op", "setBalancer", "error", fmt.Sprintf("invalid LoadBalancer IP %q", svc.Status.LoadBalancer.Ingress[0].IP), "msg", "invalid IP allocated by controller")
		return c.deleteBalancer(l, name, "invalidIP")
	}

	l = log.With(l, "ip", lbIP)

	poolName := poolFor(c.config.Pools, lbIP)
	if poolName == "" {
		l.Log("op", "setBalancer", "error", "assigned IP not allowed by config", "msg", "IP allocated by controller not allowed by config")
		return c.deleteBalancer(l, name, "ipNotAllowed")
	}

	l = log.With(l, "pool", poolName)
	pool := c.config.Pools[poolName]
	if pool == nil {
		l.Log("bug", "true", "msg", "internal error: allocated IP has no matching address pool")
		return c.deleteBalancer(l, name, "internalError")
	}

	if proto, ok := c.announced[name]; ok && proto != pool.Protocol {
		if !c.deleteBalancer(l, name, "protocolChanged") {
			return false
		}
	}

	l = log.With(l, "protocol", pool.Protocol)
	handler := c.protocols[pool.Protocol]
	if handler == nil {
		l.Log("bug", "true", "msg", "internal error: unknown balancer protocol!")
		return c.deleteBalancer(l, name, "internalError")
	}

	if deleteReason := handler.ShouldAnnounce(l, name, svc, eps); deleteReason != "" {
		return c.deleteBalancer(l, name, deleteReason)
	}

	if err := handler.SetBalancer(l, name, lbIP, pool); err != nil {
		l.Log("op", "setBalancer", "error", err, "msg", "failed to announce service")
		return false
	}

	if c.announced[name] == "" {
		c.announced[name] = pool.Protocol
		c.svcIP[name] = lbIP
	}

	announcing.With(prometheus.Labels{
		"protocol": string(pool.Protocol),
		"service":  name,
		"node":     c.myNode,
		"ip":       lbIP.String(),
	}).Set(1)
	l.Log("event", "serviceAnnounced", "msg", "service has IP, announcing")

	return true
}

func (c *controller) deleteBalancer(l log.Logger, name, reason string) bool {
	proto, ok := c.announced[name]
	if !ok {
		return true
	}

	if err := c.protocols[proto].DeleteBalancer(l, name, reason); err != nil {
		l.Log("op", "deleteBalancer", "error", err, "msg", "failed to clear balancer state")
		return false
	}

	announcing.Delete(prometheus.Labels{
		"protocol": string(proto),
		"service":  name,
		"node":     c.myNode,
		"ip":       c.svcIP[name].String(),
	})
	delete(c.announced, name)
	delete(c.svcIP, name)

	l.Log("event", "serviceWithdrawn", "ip", c.svcIP[name], "reason", reason, "msg", "withdrawing service announcement")

	return true
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

func (c *controller) SetConfig(l log.Logger, cfg *config.Config) bool {
	l.Log("event", "startUpdate", "msg", "start of config update")
	defer l.Log("event", "endUpdate", "msg", "end of config update")

	if cfg == nil {
		l.Log("op", "setConfig", "error", "no MetalLB configuration in cluster", "msg", "configuration is missing, MetalLB will not function")
		return false
	}

	for svc, ip := range c.svcIP {
		if pool := poolFor(cfg.Pools, ip); pool == "" {
			l.Log("op", "setConfig", "service", svc, "ip", ip, "error", "service has no configuration under new config", "msg", "new configuration rejected")
			return false
		}
	}

	for proto, handler := range c.protocols {
		if err := handler.SetConfig(l, cfg); err != nil {
			l.Log("op", "setConfig", "protocol", proto, "error", err, "msg", "applying new configuration to protocol handler failed")
			return false
		}
	}

	c.config = cfg

	return true
}

func (c *controller) SetNode(l log.Logger, node *v1.Node) bool {
	for proto, handler := range c.protocols {
		if err := handler.SetNode(l, node); err != nil {
			l.Log("op", "setNode", "error", err, "protocol", proto, "msg", "failed to propagate node info to protocol handler")
			return false
		}
	}
	return true
}

// A Protocol can advertise an IP address.
type Protocol interface {
	SetConfig(log.Logger, *config.Config) error
	ShouldAnnounce(log.Logger, string, *v1.Service, *v1.Endpoints) string
	SetBalancer(log.Logger, string, net.IP, *config.Pool) error
	DeleteBalancer(log.Logger, string, string) error
	SetNode(log.Logger, *v1.Node) error
}
