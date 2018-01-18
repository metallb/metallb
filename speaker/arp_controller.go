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
	"fmt"
	"net"

	"go.universe.tf/metallb/internal/config"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
)

func (c *controller) SetBalancerARP(name string, svc *v1.Service, eps *v1.Endpoints) error {
	if svc == nil {
		return c.deleteBalancerARP(name, "service deleted")
	}

	if svc.Spec.Type != "LoadBalancer" {
		return nil
	}

	glog.Infof("%s: start update", name)
	defer glog.Infof("%s: end update", name)

	if c.arpConfig == nil {
		glog.Infof("%s: skipped, waiting for config", name)
		return nil
	}

	if len(svc.Status.LoadBalancer.Ingress) != 1 {
		glog.Infof("%s: no IP allocated by controller", name)
		return c.deleteBalancerARP(name, "no IP allocated by controller")
	}

	lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP).To4()
	if lbIP == nil {
		glog.Errorf("%s: invalid LoadBalancer IP %q", name, svc.Status.LoadBalancer.Ingress[0].IP)
		return c.deleteBalancerARP(name, "invalid IP allocated by controller")
	}

	if err := c.arpIPs.Assign(name, lbIP); err != nil {
		glog.Errorf("%s: IP %q assigned by controller is not allowed by config", name, lbIP)
		return c.deleteBalancerARP(name, "invalid IP allocated by controller")
	}

	poolName := c.arpIPs.Pool(name)
	pool := c.arpConfig.Pools[c.arpIPs.Pool(name)]
	if pool == nil {
		glog.Errorf("%s: could not find pool %q that definitely should exist!", name, poolName)
		return c.deleteBalancerARP(name, "can't find pool")
	}

	if pool.Protocol != config.ARP {
		glog.Errorf("%s: protocol in pool is not set to %s, got %s", name, string(config.ARP), pool.Protocol)
		return nil
	}

	glog.Infof("%s: announcable, making 1 advertisement using ARP", name)

	c.arpAnn.SetBalancer(name, lbIP)

	announcing.With(prometheus.Labels{
		"protocol": string(config.ARP),
		"service":  name,
		"node":     c.myNode,
		"ip":       lbIP.String(),
	}).Set(1)

	return nil
}

func (c *controller) deleteBalancerARP(name, reason string) error {
	if !c.arpAnn.AnnounceName(name) {
		return nil
	}

	glog.Infof("%s: stopping announcements, %s", name, reason)
	announcing.Delete(prometheus.Labels{
		"protocol": string(config.ARP),
		"service":  name,
		"node":     c.myNode,
		"ip":       c.arpIPs.IP(name).String(),
	})
	c.arpIPs.Unassign(name)
	c.arpAnn.DeleteBalancer(name)
	return nil
}

func (c *controller) SetConfigARP(cfg *config.Config) error {
	glog.Infof("Start config update")
	defer glog.Infof("End config update")

	if cfg == nil {
		glog.Errorf("No MetalLB configuration in cluster")
		return errors.New("configuration missing")
	}

	if err := c.arpIPs.SetPools(cfg.Pools); err != nil {
		glog.Errorf("Applying new configuration failed: %s", err)
		return fmt.Errorf("configuration rejected: %s", err)
	}

	c.arpConfig = cfg

	return nil
}
