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
	"reflect"

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
	client.HandleLeadership(*myNode, ctrl.arpAnn.SetLeader)

	glog.Fatal(client.Run(*port))
}

type controller struct {
	myIP   net.IP
	myNode string

	config *config.Config
	ips    *allocator.Allocator

	arpAnn *arp.Announce

	bgpPeers  []*peer
	bgpSvcAds map[string][]*bgp.Advertisement
}

func newController(myIP net.IP, myNode string, noARP bool) (*controller, error) {
	var (
		arpAnn *arp.Announce
		err    error
	)
	if !noARP {
		arpAnn, err = arp.New(myIP)
		if err != nil {
			return nil, fmt.Errorf("making ARP announcer: %s", err)
		}
	}

	ret := &controller{
		myIP:   myIP,
		myNode: myNode,

		ips: allocator.New(),

		arpAnn: arpAnn,

		bgpSvcAds: map[string][]*bgp.Advertisement{},
	}

	return ret, nil
}

func (c *controller) SetBalancer(name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc == nil {
		return c.deleteBalancerBGP(name, "service deleted")
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
		return c.deleteBalancerBGP(name, "no IP allocated by controller")
	}

	// Should we advertise? Yes, if externalTrafficPolicy is Cluster,
	// or Local && there's a ready local endpoint.
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal && !k8s.NodeHasHealthyEndpoint(eps, c.myNode) {
		glog.Infof("%s: externalTrafficPolicy is Local, and no healthy local endpoints", name)
		return c.deleteBalancerBGP(name, "no healthy local endpoints")
	}

	lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP).To4()
	if lbIP == nil {
		glog.Errorf("%s: invalid LoadBalancer IP %q", name, svc.Status.LoadBalancer.Ingress[0].IP)
		return c.deleteBalancerBGP(name, "invalid IP allocated by controller")
	}

	if err := c.ips.Assign(name, lbIP); err != nil {
		glog.Errorf("%s: IP %q assigned by controller is not allowed by config", name, lbIP)
		return c.deleteBalancerBGP(name, "invalid IP allocated by controller")
	}

	poolName := c.ips.Pool(name)
	pool := c.config.Pools[c.ips.Pool(name)]
	if pool == nil {
		glog.Errorf("%s: could not find pool %q that definitely should exist!", name, poolName)
		return c.deleteBalancerBGP(name, "can't find pool")
	}

	switch pool.Protocol {
	case config.ARP:
		if err := c.SetBalancerARP(name, lbIP, pool); err != nil {
			return err
		}
	case config.BGP:
		if err := c.SetBalancerBGP(name, lbIP, pool); err != nil {
			return err
		}
	default:
		glog.Errorf("%s: unknown balancer protocol %q. This should not happen, please file a bug!", name, pool.Protocol)
		return c.deleteBalancer(name, "internal error (unknown balancer protocol)")
	}

	announcing.With(prometheus.Labels{
		"protocol": string(pool.Protocol),
		"service":  name,
		"node":     c.myNode,
		"ip":       lbIP.String(),
	}).Set(1)

	return nil
}

func (c *controller) deleteBalancer(name, reason string) error {
	if err := c.deleteBalancerARP(name, reason); err != nil {
		return err
	}
	if err := c.deleteBalancerBGP(name, reason); err != nil {
		return err
	}

	// TODO: put the log about stopping announcements here, a few
	// refactoring steps down the road.

	c.ips.Unassign(name)

	for _, proto := range []config.Proto{config.ARP, config.BGP} {
		announcing.Delete(prometheus.Labels{
			"protocol": string(proto),
			"service":  name,
			"node":     c.myNode,
			"ip":       c.ips.IP(name).String(),
		})
	}
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

	newPeers := make([]*peer, 0, len(cfg.Peers))
newPeers:
	for _, p := range cfg.Peers {
		for i, ep := range c.bgpPeers {
			if ep == nil {
				continue
			}
			if reflect.DeepEqual(p, ep.cfg) {
				newPeers = append(newPeers, ep)
				c.bgpPeers[i] = nil
				continue newPeers
			}
		}
		// No existing peers match, create a new one.
		newPeers = append(newPeers, &peer{
			cfg: p,
		})
	}

	c.config = cfg
	oldPeers := c.bgpPeers
	c.bgpPeers = newPeers

	for _, p := range oldPeers {
		if p == nil {
			continue
		}
		glog.Infof("Peer %q deconfigured, closing BGP session", p.cfg.Addr)
		if err := p.bgp.Close(); err != nil {
			glog.Warningf("Shutting down BGP session to %q: %s", p.cfg.Addr, err)
		}
	}

	var errs []error
	for _, p := range c.bgpPeers {
		if p.bgp != nil {
			continue
		}

		glog.Infof("Peer %q configured, starting BGP session", p.cfg.Addr)
		routerID := c.myIP
		if p.cfg.RouterID != nil {
			routerID = p.cfg.RouterID
		}
		s, err := newBGP(fmt.Sprintf("%s:%d", p.cfg.Addr, p.cfg.Port), p.cfg.MyASN, routerID, p.cfg.ASN, p.cfg.HoldTime)
		if err != nil {
			errs = append(errs, fmt.Errorf("Creating BGP session to %q: %s", p.cfg.Addr, err))
		} else {
			p.bgp = s
		}
	}
	if len(errs) != 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		return fmt.Errorf("%d new BGP sessions failed to start", len(errs))
	}

	return nil
}
