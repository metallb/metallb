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
	"fmt"
	"net"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	v1 "k8s.io/api/core/v1"

	"go.universe.tf/metallb/internal/allocator/k8salloc"
	"go.universe.tf/metallb/internal/ipfamily"
)

const (
	annotationAddressPool = "metallb.universe.tf/address-pool"
)

func (c *controller) convergeBalancer(l log.Logger, key string, svc *v1.Service) bool {
	lbIPs := []net.IP{}
	var err error
	// Not a LoadBalancer, early exit. It might have been a balancer
	// in the past, so we still need to clear LB state.
	if svc.Spec.Type != "LoadBalancer" {
		level.Debug(l).Log("event", "clearAssignment", "reason", "notLoadBalancer", "msg", "not a LoadBalancer")
		c.clearServiceState(key, svc)
		// Early return, we explicitly do *not* want to reallocate
		// an IP.
		return true
	}

	// If the ClusterIPs is malformed or not set we can't determine the
	// ipFamily to use.
	if len(svc.Spec.ClusterIPs) == 0 {
		level.Info(l).Log("event", "clearAssignment", "reason", "noClusterIPs", "msg", "No ClusterIPs")
		c.clearServiceState(key, svc)
		return true
	}

	// The assigned LB IP(s) is the end state of convergence. If there's
	// none or a malformed one, nuke all controlled state so that we
	// start converging from a clean slate.
	for i := range svc.Status.LoadBalancer.Ingress {
		ip := svc.Status.LoadBalancer.Ingress[i].IP
		if len(ip) != 0 {
			lbIPs = append(lbIPs, net.ParseIP(ip))
		}
	}

	lbIPsIPFamily := ipfamily.Unknown
	if len(lbIPs) == 0 {
		c.clearServiceState(key, svc)
	} else {
		lbIPsIPFamily, err = ipfamily.ForAddressesIPs(lbIPs)
		if err != nil {
			level.Error(l).Log("event", "clearAssignment", "reason", "nolbIPsIPFamily", "msg", "Failed to retrieve lbIPs family")
			c.client.Errorf(svc, "nolbIPsIPFamily", "Failed to retrieve LBIPs IPFamily for %q: %s", lbIPs, err)
			return true
		}
		clusterIPsIPFamily, err := ipfamily.ForAddresses(svc.Spec.ClusterIPs)
		if err != nil {
			level.Error(l).Log("event", "clearAssignment", "reason", "noclusterIPsIPFamily", "msg", "Failed to retrieve clusterIPs family")
			c.client.Errorf(svc, "noclusterIPsIPFamily", "Failed to retrieve ClusterIPs IPFamily for %q: %s", svc.Spec.ClusterIPs, err)
			return true
		}
		// Clear the lbIP if it has a different ipFamily compared to the clusterIP.
		// (this should not happen since the "ipFamily" of a service is immutable)
		if lbIPsIPFamily != clusterIPsIPFamily {
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}
	}

	// It's possible the config mutated and the IP we have no longer
	// makes sense. If so, clear it out and give the rest of the logic
	// a chance to allocate again.
	if len(lbIPs) != 0 {
		// This assign is idempotent if the config is consistent,
		// otherwise it'll fail and tell us why.
		if err = c.ips.Assign(key, lbIPs, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			level.Info(l).Log("event", "clearAssignment", "error", err, "msg", "current IP not allowed by config, clearing")
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}

		// The user might also have changed the pool annotation, and
		// requested a different pool than the one that is currently
		// allocated.
		desiredPool := svc.Annotations[annotationAddressPool]
		if len(lbIPs) != 0 && desiredPool != "" && c.ips.Pool(key) != desiredPool {
			level.Info(l).Log("event", "clearAssignment", "reason", "differentPoolRequested", "msg", "user requested a different pool than the one currently assigned")
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}
		// User set or changed the desired LB IP, nuke the
		// state. allocateIP will pay attention to LoadBalancerIP and try
		// to meet the user's demands.
		if svc.Spec.LoadBalancerIP != "" {
			if lbIPsIPFamily == ipfamily.DualStack {
				level.Error(l).Log("event", "loadbalancerIP", "error", "DualStack", "msg", "user requested loadbalancer IP for dualstack address family")
				c.client.Errorf(svc, "LoadBalancerFailed", "Failed loadbalancer IP for dual-stack")
				return true
			}
			if svc.Spec.LoadBalancerIP != lbIPs[0].String() {
				level.Info(l).Log("event", "clearAssignment", "reason", "differentIPRequested", "msg", "user requested a different IP than the one currently assigned")
				c.clearServiceState(key, svc)
				lbIPs = []net.IP{}
			}
		}
	}

	// If lbIP is still nil at this point, try to allocate.
	if len(lbIPs) == 0 {
		if !c.synced {
			level.Error(l).Log("op", "allocateIPs", "error", "controller not synced", "msg", "controller not synced yet, cannot allocate IP; will retry after sync")
			return false
		}
		lbIPs, err = c.allocateIPs(key, svc)
		if err != nil {
			level.Error(l).Log("op", "allocateIPs", "error", err, "msg", "IP allocation failed")
			c.client.Errorf(svc, "AllocationFailed", "Failed to allocate IP for %q: %s", key, err)
			// The outer controller loop will retry converging this
			// service when another service gets deleted, so there's
			// nothing to do here but wait to get called again later.
			return true
		}
		level.Info(l).Log("event", "ipAllocated", "ip", lbIPs, "msg", "IP address assigned by controller")
		c.client.Infof(svc, "IPAllocated", "Assigned IP %q", lbIPs)
	}

	if len(lbIPs) == 0 {
		level.Error(l).Log("bug", "true", "msg", "internal error: failed to allocate an IP, but did not exit convergeService early!")
		c.client.Errorf(svc, "InternalError", "didn't allocate an IP but also did not fail")
		c.clearServiceState(key, svc)
		return true
	}

	pool := c.ips.Pool(key)
	if pool == "" || c.config.Pools[pool] == nil {
		level.Error(l).Log("bug", "true", "ip", lbIPs, "msg", "internal error: allocated IP has no matching address pool")
		c.client.Errorf(svc, "InternalError", "allocated an IP that has no pool")
		c.clearServiceState(key, svc)
		return true
	}

	// At this point, we have an IP selected somehow, all that remains
	// is to program the data plane.
	lbIngressIPs := []v1.LoadBalancerIngress{}
	for _, lbIP := range lbIPs {
		lbIngressIPs = append(lbIngressIPs, v1.LoadBalancerIngress{IP: lbIP.String()})
	}
	svc.Status.LoadBalancer.Ingress = lbIngressIPs
	return true
}

// clearServiceState clears all fields that are actively managed by
// this controller.
func (c *controller) clearServiceState(key string, svc *v1.Service) {
	c.ips.Unassign(key)
	svc.Status.LoadBalancer = v1.LoadBalancerStatus{}
}

func (c *controller) allocateIPs(key string, svc *v1.Service) ([]net.IP, error) {
	if len(svc.Spec.ClusterIPs) == 0 {
		// (we should never get here because the caller ensured that Spec.ClusterIP != nil)
		return nil, fmt.Errorf("invalid ClusterIP [%s], can't determine family", svc.Spec.ClusterIP)
	}

	serviceIPFamily, err := ipfamily.ForAddresses(svc.Spec.ClusterIPs)
	if err != nil {
		return nil, err
	}

	// If the user asked for a specific IP, try that.
	if svc.Spec.LoadBalancerIP != "" {
		ip := net.ParseIP(svc.Spec.LoadBalancerIP)
		if ip == nil {
			return nil, fmt.Errorf("invalid spec.loadBalancerIP %q", svc.Spec.LoadBalancerIP)
		}
		lbipFamily := ipfamily.ForAddress(ip)
		if serviceIPFamily != lbipFamily {
			return nil, fmt.Errorf("requested spec.loadBalancerIP %q does not match the ipFamily of the service", svc.Spec.LoadBalancerIP)
		}
		lbIPs := []net.IP{ip}
		if err := c.ips.Assign(key, lbIPs, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			return nil, err
		}
		return lbIPs, nil
	}
	// Otherwise, did the user ask for a specific pool?
	desiredPool := svc.Annotations[annotationAddressPool]
	if desiredPool != "" {
		ips, err := c.ips.AllocateFromPool(key, serviceIPFamily, desiredPool, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
		if err != nil {
			return nil, err
		}
		return ips, nil
	}

	// Okay, in that case just bruteforce across all pools.
	return c.ips.Allocate(key, serviceIPFamily, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
}
