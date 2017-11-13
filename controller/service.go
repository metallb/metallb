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

	"github.com/golang/glog"
	"go.universe.tf/metallb/internal"
	"go.universe.tf/metallb/internal/config"
	"k8s.io/api/core/v1"
)

func (c *controller) convergeService(key string, svc *v1.Service) {
	var lbIP net.IP

	// The assigned LB IP is the end state of convergence. If there's
	// none or a malformed one, nuke all controlled state so that we
	// start converging from a clean slate.
	if len(svc.Status.LoadBalancer.Ingress) == 1 {
		lbIP = net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
	}
	if lbIP == nil {
		glog.Infof("%q has no valid ingress IP currently", key)
		c.clearServiceState(key, svc)
	}

	// It's possible the config mutated and the IP we have no longer
	// makes sense. If so, clear it out and give the rest of the logic
	// a chance to allocate again.
	if lbIP != nil && !c.ipIsValid(lbIP) {
		glog.Infof("%q has assigned IP %q, but that IP is no longer valid per config", key, lbIP)
		c.clearServiceState(key, svc)
		lbIP = nil
	}

	// If there's an LB IP, but we think it's currently assigned to
	// someone else, clear it! Something needs fixing.
	if lbIP != nil {
		conflict := false
		if s, ok := c.ipToSvc[lbIP.String()]; ok && s != key {
			conflict = true
		} else if i, ok := c.svcToIP[key]; ok && i != lbIP.String() {
			conflict = true
		}
		if conflict {
			glog.Infof("%q has assigned IP %q, in conflict with another service", key, lbIP)
			c.clearServiceState(key, svc)
			lbIP = nil
		}
	}

	// User set or changed the desired LB IP, nuke the
	// state. allocateIP will pay attention to LoadBalancerIP and try
	// to meet the user's demands.
	if svc.Spec.LoadBalancerIP != "" && svc.Spec.LoadBalancerIP != lbIP.String() {
		glog.Infof("%q assigned %q, user requested %q", key, lbIP, svc.Spec.LoadBalancerIP)
		c.clearServiceState(key, svc)
		lbIP = nil
	}

	// If lbIP is still nil at this point, try to allocate.
	if lbIP == nil {
		glog.Infof("%q allocating IP", key)
		ip, err := c.allocateIP(key, svc)
		if err != nil {
			c.events.Eventf(svc, v1.EventTypeWarning, "AllocationFailed", "Failed to allocate IP for %q: %s", key, err)
			// TODO: should retry on pool exhaustion allocation
			// failures, once we keep track of when pools become
			// non-full.
			return
		}
		lbIP = ip
		glog.Infof("%q has been allocated IP %q", key, lbIP)
	}

	if lbIP == nil {
		glog.Infof("%q failed to allocate an IP, but did not exit convergeService early", key)
		c.events.Eventf(svc, v1.EventTypeWarning, "InternalError", "didn't allocate an IP but also did not fail")
		return
	}

	// At this point, we have an IP selected somehow, all that remains
	// is to program the data plane...
	svc.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	svc.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{IP: lbIP.String()}}

	// ... and record that we allocated the IP.
	c.ipToSvc[lbIP.String()] = key
	c.svcToIP[key] = lbIP.String()
}

// clearServiceState clears all fields that are actively managed by
// this controller.
func (c *controller) clearServiceState(key string, svc *v1.Service) {
	delete(c.ipToSvc, c.svcToIP[key])
	delete(c.svcToIP, key)
	svc.Status.LoadBalancer = v1.LoadBalancerStatus{}
}

// ipIsValid checks that ip is part of a configured pool.
func (c *controller) ipIsValid(ip net.IP) bool {
	for _, p := range c.config.Pools {
		for _, c := range p.CIDR {
			if c.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func (c *controller) allocateIP(key string, svc *v1.Service) (net.IP, error) {
	// If the user asked for a specific IP, try that.
	if svc.Spec.LoadBalancerIP != "" {
		ip := net.ParseIP(svc.Spec.LoadBalancerIP).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid spec.loadBalancerIP %q", svc.Spec.LoadBalancerIP)
		}
		if err := c.assignIP(key, svc, ip); err != nil {
			return nil, err
		}
		return ip, nil
	}

	// Otherwise, did the user ask for a specific pool?
	desiredPool := svc.Annotations[internal.AnnotationAddressPool]
	if desiredPool != "" {
		if p, ok := c.config.Pools[desiredPool]; ok {
			return c.allocateIPFromPool(key, svc, p)
		}
		return nil, fmt.Errorf("pool %q does not exist", desiredPool)
	}

	// Okay, in that case just bruteforce across all pools.
	for _, p := range c.config.Pools {
		ip, err := c.allocateIPFromPool(key, svc, p)
		if err != nil {
			return nil, err
		}
		if ip != nil {
			return ip, nil
		}
	}
	return nil, errors.New("no addresses available in any pool")
}

func (c *controller) allocateIPFromPool(key string, svc *v1.Service, pool *config.Pool) (net.IP, error) {
	for _, cidr := range pool.CIDR {
		for ip := cidr.IP; cidr.Contains(ip); ip = nextIP(ip) {
			if _, ok := c.ipToSvc[ip.String()]; !ok {
				// Minor inefficiency here, assignIP will
				// retraverse the pools to check that ip is
				// contained within a pool. TODO: refactor to
				// avoid.
				err := c.assignIP(key, svc, ip)
				if err != nil {
					return nil, err
				}
				return ip, nil
			}
		}
	}
	return nil, nil
}

func (c *controller) assignIP(key string, svc *v1.Service, ip net.IP) error {
	if s, ok := c.ipToSvc[ip.String()]; ok && s != key {
		return fmt.Errorf("address already belongs to other service %q", s)
	}

	if !c.ipIsValid(ip) {
		return errors.New("address is not part of any known pool")
	}

	c.events.Eventf(svc, v1.EventTypeNormal, "IPAllocated", "Assigned IP %q", ip)
	return nil
}

func nextIP(prev net.IP) net.IP {
	var ip net.IP
	ip = append(ip, prev...)
	if ip.To4() != nil {
		ip = ip.To4()
	}
	for o := 0; o < len(ip); o++ {
		if ip[len(ip)-o-1] != 255 {
			ip[len(ip)-o-1]++
			return ip
		}
		ip[len(ip)-o-1] = 0
	}
	return ip
}
