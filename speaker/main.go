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
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	metallbcfg "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"
	"go.universe.tf/metallb/internal/k8s/controllers"
	"go.universe.tf/metallb/internal/k8s/epslices"
	"go.universe.tf/metallb/internal/layer2"
	"go.universe.tf/metallb/internal/logging"
	"go.universe.tf/metallb/internal/speakerlist"
	"go.universe.tf/metallb/internal/version"
	v1 "k8s.io/api/core/v1"
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

// Service offers methods to mutate a Kubernetes service object.
type service interface {
	UpdateStatus(svc *v1.Service) error
	Infof(svc *v1.Service, desc, msg string, args ...interface{})
	Errorf(svc *v1.Service, desc, msg string, args ...interface{})
}

func main() {
	prometheus.MustRegister(announcing)

	var (
		namespace         = flag.String("namespace", os.Getenv("METALLB_NAMESPACE"), "config file and speakers namespace")
		host              = flag.String("host", os.Getenv("METALLB_HOST"), "HTTP host address")
		mlBindAddr        = flag.String("ml-bindaddr", os.Getenv("METALLB_ML_BIND_ADDR"), "Bind addr for MemberList (fast dead node detection)")
		mlBindPort        = flag.String("ml-bindport", os.Getenv("METALLB_ML_BIND_PORT"), "Bind port for MemberList (fast dead node detection)")
		mlLabels          = flag.String("ml-labels", os.Getenv("METALLB_ML_LABELS"), "Labels to match the speakers (for MemberList / fast dead node detection)")
		mlSecretKeyPath   = flag.String("ml-secret-key-path", os.Getenv("METALLB_ML_SECRET_KEY_PATH"), "Path to where the MembeList's secret key is mounted")
		myNode            = flag.String("node-name", os.Getenv("METALLB_NODE_NAME"), "name of this Kubernetes node (spec.nodeName)")
		port              = flag.Int("port", 7472, "HTTP listening port")
		logLevel          = flag.String("log-level", "info", fmt.Sprintf("log level. must be one of: [%s]", logging.Levels.String()))
		disableEpSlices   = flag.Bool("disable-epslices", false, "Disable the usage of EndpointSlices and default to Endpoints instead of relying on the autodiscovery mechanism")
		enablePprof       = flag.Bool("enable-pprof", false, "Enable pprof profiling")
		loadBalancerClass = flag.String("lb-class", "", "load balancer class. When enabled, metallb will handle only services whose spec.loadBalancerClass matches the given lb class")
	)
	flag.Parse()

	// Note: Changing the MetalLB BGP implementation type should be considered
	//       experimental.
	bgpType, present := os.LookupEnv("METALLB_BGP_TYPE")
	if !present {
		bgpType = "native"
	}

	logger, err := logging.Init(*logLevel)
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	level.Info(logger).Log("version", version.Version(), "commit", version.CommitHash(), "branch", version.Branch(), "goversion", version.GoString(), "msg", "MetalLB speaker starting "+version.String())

	if *namespace == "" {
		bs, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			level.Error(logger).Log("op", "startup", "msg", "Unable to get namespace from pod service account data, please specify --namespace or METALLB_NAMESPACE", "error", err)
			os.Exit(1)
		}
		*namespace = string(bs)
	}

	if *myNode == "" {
		level.Error(logger).Log("op", "startup", "error", "must specify --node-name or METALLB_NODE_NAME", "msg", "missing configuration")
		os.Exit(1)
	}

	stopCh := make(chan struct{})
	go func() {
		c1 := make(chan os.Signal, 1)
		signal.Notify(c1, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
		<-c1
		level.Info(logger).Log("op", "shutdown", "msg", "starting shutdown")
		signal.Stop(c1)
		close(stopCh)
	}()
	defer level.Info(logger).Log("op", "shutdown", "msg", "done")

	var mlSecret string

	if *mlSecretKeyPath != "" {
		mlSecretBytes, err := os.ReadFile(filepath.Join(*mlSecretKeyPath, k8s.MLSecretKeyName))
		if err != nil {
			level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to read memberlist secret key file")
			os.Exit(1)
		}
		mlSecret = string(mlSecretBytes)
	}

	sList, err := speakerlist.New(logger, *myNode, *mlBindAddr, *mlBindPort, mlSecret, *namespace, *mlLabels, stopCh)
	if err != nil {
		os.Exit(1)
	}

	// Setup all clients and speakers, config decides what is being done runtime.
	ctrl, err := newController(controllerConfig{
		MyNode:   *myNode,
		Logger:   logger,
		LogLevel: logging.Level(*logLevel),
		SList:    sList,
		bgpType:  bgpImplementation(bgpType),
	})
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to create MetalLB controller")
		os.Exit(1)
	}

	var validateConfig metallbcfg.Validate
	if bgpType == "native" {
		validateConfig = metallbcfg.DiscardFRROnly
	} else {
		validateConfig = metallbcfg.DiscardNativeOnly
	}

	client, err := k8s.New(&k8s.Config{
		ProcessName:     "metallb-speaker",
		NodeName:        *myNode,
		Logger:          logger,
		DisableEpSlices: *disableEpSlices,

		MetricsHost:   *host,
		MetricsPort:   *port,
		EnablePprof:   *enablePprof,
		ReadEndpoints: true,
		Namespace:     *namespace,

		Listener: k8s.Listener{
			ServiceChanged: ctrl.SetBalancer,
			ConfigChanged:  ctrl.SetConfig,
			NodeChanged:    ctrl.SetNode,
		},
		ValidateConfig:    validateConfig,
		LoadBalancerClass: *loadBalancerClass,
	})
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to create k8s client")
		os.Exit(1)
	}
	ctrl.client = client

	sList.Start(client)
	defer sList.Stop()

	if err := client.Run(stopCh); err != nil {
		level.Error(logger).Log("op", "startup", "error", err, "msg", "failed to run k8s client")
		os.Exit(1)
	}
}

type controller struct {
	myNode  string
	bgpType bgpImplementation

	config *config.Config
	client service

	protocolHandlers map[config.Proto]Protocol
	announced        map[config.Proto]map[string]bool // for each protocol, says if we are advertising the given service
	svcIPs           map[string][]net.IP              // service name -> assigned IPs

	protocols []config.Proto
}

type controllerConfig struct {
	MyNode   string
	Logger   log.Logger
	LogLevel logging.Level
	SList    SpeakerList

	bgpType bgpImplementation

	// For testing only, and will be removed in a future release.
	// See: https://github.com/metallb/metallb/issues/152.
	DisableLayer2      bool
	SupportedProtocols []config.Proto
}

func newController(cfg controllerConfig) (*controller, error) {
	handlers := map[config.Proto]Protocol{
		config.BGP: &bgpController{
			logger:         cfg.Logger,
			myNode:         cfg.MyNode,
			svcAds:         make(map[string][]*bgp.Advertisement),
			bgpType:        cfg.bgpType,
			sessionManager: newBGP(cfg.bgpType, cfg.Logger, cfg.LogLevel),
		},
	}
	protocols := []config.Proto{config.BGP}

	if !cfg.DisableLayer2 {
		a, err := layer2.New(cfg.Logger)
		if err != nil {
			return nil, fmt.Errorf("making layer2 announcer: %s", err)
		}
		handlers[config.Layer2] = &layer2Controller{
			announcer: a,
			myNode:    cfg.MyNode,
			sList:     cfg.SList,
		}
		protocols = append(protocols, config.Layer2)
	}

	ret := &controller{
		myNode:           cfg.MyNode,
		bgpType:          cfg.bgpType,
		protocolHandlers: handlers,
		announced:        map[config.Proto]map[string]bool{},
		svcIPs:           map[string][]net.IP{},
		protocols:        protocols,
	}
	ret.announced[config.BGP] = map[string]bool{}
	ret.announced[config.Layer2] = map[string]bool{}

	return ret, nil
}

func (c *controller) SetBalancer(l log.Logger, name string, svc *v1.Service, eps epslices.EpsOrSlices) controllers.SyncState {
	if svc == nil {
		return c.deleteBalancer(l, name, "serviceDeleted")
	}

	if svc.Spec.Type != "LoadBalancer" {
		return c.deleteBalancer(l, name, "notLoadBalancer")
	}

	level.Debug(l).Log("event", "startUpdate", "msg", "start of service update")
	defer level.Debug(l).Log("event", "endUpdate", "msg", "end of service update")

	if c.config == nil {
		level.Debug(l).Log("event", "noConfig", "msg", "not processing, still waiting for config")
		return controllers.SyncStateSuccess
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return c.deleteBalancer(l, name, "noIPAllocated")
	}

	lbIPs := []net.IP{}
	for i := range svc.Status.LoadBalancer.Ingress {
		lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[i].IP)
		if lbIP == nil {
			level.Error(l).Log("op", "setBalancer", "error", fmt.Sprintf("invalid LoadBalancer IP %q", svc.Status.LoadBalancer.Ingress[i].IP), "msg", "invalid IP allocated by controller")
			return c.deleteBalancer(l, name, "invalidIP")
		}
		lbIPs = append(lbIPs, lbIP)
	}

	l = log.With(l, "ips", lbIPs)

	poolName := poolFor(c.config.Pools, lbIPs)
	if poolName == "" {
		level.Error(l).Log("op", "setBalancer", "error", "assigned IP not allowed by config", "msg", "IP allocated by controller not allowed by config")
		return c.deleteBalancer(l, name, "ipNotAllowed")
	}

	l = log.With(l, "pool", poolName)
	if c.config.Pools == nil || c.config.Pools.ByName[poolName] == nil {
		level.Error(l).Log("bug", "true", "msg", "internal error: allocated IP has no matching address pool")
		return c.deleteBalancer(l, name, "internalError")
	}
	pool := c.config.Pools.ByName[poolName]

	if svcIPs, ok := c.svcIPs[name]; ok && !compareIPs(lbIPs, svcIPs) {
		if st := c.deleteBalancer(l, name, "loadBalancerIPChanged"); st == controllers.SyncStateError {
			return st
		}
	}

	for _, protocol := range c.protocols {
		if st := c.handleService(l, name, lbIPs, svc, pool, eps, protocol); st == controllers.SyncStateError {
			return st
		}
	}

	return controllers.SyncStateSuccess
}

func (c *controller) handleService(l log.Logger,
	name string,
	lbIPs []net.IP,
	svc *v1.Service, pool *config.Pool,
	eps epslices.EpsOrSlices,
	protocol config.Proto) controllers.SyncState {

	l = log.With(l, "protocol", protocol)
	handler := c.protocolHandlers[protocol]
	if handler == nil {
		level.Error(l).Log("bug", "true", "msg", "internal error: unknown balancer protocol!")
		return c.deleteBalancerProtocol(l, protocol, name, "internalError")
	}

	if deleteReason := handler.ShouldAnnounce(l, name, lbIPs, pool, svc, eps); deleteReason != "" {
		return c.deleteBalancerProtocol(l, protocol, name, deleteReason)
	}

	if err := handler.SetBalancer(l, name, lbIPs, pool, c.client, svc); err != nil {
		level.Error(l).Log("op", "setBalancer", "error", err, "msg", "failed to announce service")
		return controllers.SyncStateError
	}

	if !c.announced[protocol][name] {
		c.announced[protocol][name] = true
		c.svcIPs[name] = lbIPs
	}

	for _, ip := range lbIPs {
		announcing.With(prometheus.Labels{
			"protocol": string(protocol),
			"service":  name,
			"node":     c.myNode,
			"ip":       ip.String(),
		}).Set(1)
	}
	level.Info(l).Log("event", "serviceAnnounced", "msg", "service has IP, announcing", "protocol", protocol)
	c.client.Infof(svc, "nodeAssigned", "announcing from node %q with protocol %q", c.myNode, protocol)
	return controllers.SyncStateSuccess
}

func (c *controller) deleteBalancer(l log.Logger, name, reason string) controllers.SyncState {
	for _, protocol := range c.protocols {
		if st := c.deleteBalancerProtocol(l, protocol, name, reason); st == controllers.SyncStateError {
			return st
		}
	}
	return controllers.SyncStateSuccess
}

func (c *controller) deleteBalancerProtocol(l log.Logger, protocol config.Proto, name, reason string) controllers.SyncState {
	announced := c.announced[protocol][name]
	if !announced {
		return controllers.SyncStateSuccess
	}

	if err := c.protocolHandlers[protocol].DeleteBalancer(l, name, reason); err != nil {
		level.Error(l).Log("op", "deleteBalancer", "error", err, "msg", "failed to clear balancer state", "protocol", protocol)
		return controllers.SyncStateError
	}

	for _, ip := range c.svcIPs[name] {
		ok := announcing.Delete(prometheus.Labels{
			"protocol": string(protocol),
			"service":  name,
			"node":     c.myNode,
			"ip":       ip.String(),
		})
		if !ok {
			level.Error(l).Log("op", "deleteBalancer", "error", "failed to delete service metric", "service", name, "protocol", protocol, "ip", ip.String())
		}
	}
	delete(c.announced[protocol], name)

	// we withdraw the service only if we are removing it from the last protocol
	for _, p := range c.protocols {
		if c.announced[p][name] {
			return controllers.SyncStateSuccess
		}
	}
	delete(c.svcIPs, name)
	level.Info(l).Log("event", "serviceWithdrawn", "ip", c.svcIPs[name], "reason", reason, "msg", "withdrawing service announcement")

	return controllers.SyncStateSuccess
}

func poolFor(pools *config.Pools, ips []net.IP) string {
	if pools == nil {
		return ""
	}
	for pname, p := range pools.ByName {
		cnt := 0
		for _, ip := range ips {
			for _, cidr := range p.CIDR {
				if cidr.Contains(ip) {
					cnt++
					break
				}
			}
			if cnt == len(ips) {
				return pname
			}
		}
	}
	return ""
}

func compareIPs(ips1, ips2 []net.IP) bool {
	if len(ips1) != len(ips2) {
		return false
	}

	for _, ip1 := range ips1 {
		found := false
		for _, ip2 := range ips2 {
			if ip1.Equal(ip2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c *controller) SetConfig(l log.Logger, cfg *config.Config) controllers.SyncState {

	level.Debug(l).Log("event", "startUpdate", "msg", "start of config update")
	defer level.Debug(l).Log("event", "endUpdate", "msg", "end of config update")

	if cfg == nil {
		level.Error(l).Log("op", "setConfig", "error", "no MetalLB configuration in cluster", "msg", "configuration is missing, MetalLB will not function")
		return controllers.SyncStateErrorNoRetry
	}

	for svc, ip := range c.svcIPs {
		if pool := poolFor(cfg.Pools, ip); pool == "" {
			level.Error(l).Log("op", "setConfig", "service", svc, "ip", ip, "error", "service has no configuration under new config", "msg", "new configuration rejected")
			return controllers.SyncStateError
		}
	}

	for proto, handler := range c.protocolHandlers {
		if err := handler.SetConfig(l, cfg); err != nil {
			level.Error(l).Log("op", "setConfig", "protocol", proto, "error", err, "msg", "applying new configuration to protocol handler failed")
			return controllers.SyncStateErrorNoRetry
		}
	}

	c.config = cfg

	return controllers.SyncStateReprocessAll
}

func (c *controller) SetNode(l log.Logger, node *v1.Node) controllers.SyncState {
	for proto, handler := range c.protocolHandlers {
		if err := handler.SetNode(l, node); err != nil {
			level.Error(l).Log("op", "setNode", "error", err, "protocol", proto, "msg", "failed to propagate node info to protocol handler")
			return controllers.SyncStateError
		}
	}
	return controllers.SyncStateSuccess
}

// A Protocol can advertise an IP address.
type Protocol interface {
	SetConfig(log.Logger, *config.Config) error
	ShouldAnnounce(log.Logger, string, []net.IP, *config.Pool, *v1.Service, epslices.EpsOrSlices) string
	SetBalancer(log.Logger, string, []net.IP, *config.Pool, service, *v1.Service) error
	DeleteBalancer(log.Logger, string, string) error
	SetNode(log.Logger, *v1.Node) error
}

// Speakerlist represents a list of healthy speakers.
type SpeakerList interface {
	UsableSpeakers() map[string]bool
	Rejoin()
}
