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
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	v1 "k8s.io/api/core/v1"

	"go.universe.tf/metallb/internal/allocator/k8salloc"
)

const (
	annotationAddressPool     = "metallb.universe.tf/address-pool"
	annotationLoadBalancerIPs = "metallb.universe.tf/load-balancer-ips"
)

func (c *controller) convergeBalancer(l log.Logger, key string, svc *v1.Service) bool {
	var lbIP net.IP

	// Not a LoadBalancer, early exit. It might have been a balancer
	// in the past, so we still need to clear LB state.
	if svc.Spec.Type != "LoadBalancer" {
		level.Debug(l).Log("event", "clearAssignment", "reason", "notLoadBalancer", "msg", "not a LoadBalancer")
		c.clearServiceState(key, svc)
		// Early return, we explicitly do *not* want to reallocate
		// an IP.
		return true
	}

	// If the ClusterIP is malformed or not set we can't determine the
	// ipFamily to use.
	clusterIP := net.ParseIP(svc.Spec.ClusterIP)
	if clusterIP == nil {
		level.Info(l).Log("event", "clearAssignment", "reason", "noClusterIP", "msg", "No ClusterIP")
		c.clearServiceState(key, svc)
		return true
	}

	if len(svc.Spec.ClusterIPs) > 1 && svc.Spec.LoadBalancerIP == "" {
		return c.convergeBalancerDual(l, key, svc)
	}

	// The assigned LB IP is the end state of convergence. If there's
	// none or a malformed one, nuke all controlled state so that we
	// start converging from a clean slate.
	if len(svc.Status.LoadBalancer.Ingress) == 1 {
		lbIP = net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
	}
	if lbIP == nil {
		c.clearServiceState(key, svc)
	}

	// Clear the lbIP if it has a different ipFamily compared to the clusterIP.
	// (this should not happen since the "ipFamily" of a service is immutable)
	if (clusterIP.To4() == nil) != (lbIP.To4() == nil) {
		c.clearServiceState(key, svc)
		lbIP = nil
	}

	// It's possible the config mutated and the IP we have no longer
	// makes sense. If so, clear it out and give the rest of the logic
	// a chance to allocate again.
	if lbIP != nil {
		// This assign is idempotent if the config is consistent,
		// otherwise it'll fail and tell us why.
		if err := c.ips.Assign(key, lbIP, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			level.Info(l).Log("event", "clearAssignment", "reason", "notAllowedByConfig", "msg", "current IP not allowed by config, clearing")
			c.clearServiceState(key, svc)
			lbIP = nil
		}

		// The user might also have changed the pool annotation, and
		// requested a different pool than the one that is currently
		// allocated.
		desiredPool := svc.Annotations[annotationAddressPool]
		if lbIP != nil && desiredPool != "" && c.ips.Pool(key) != desiredPool {
			level.Info(l).Log("event", "clearAssignment", "reason", "differentPoolRequested", "msg", "user requested a different pool than the one currently assigned")
			c.clearServiceState(key, svc)
			lbIP = nil
		}
	}

	// User set or changed the desired LB IP, nuke the
	// state. allocateIP will pay attention to LoadBalancerIP and try
	// to meet the user's demands.
	if svc.Spec.LoadBalancerIP != "" && svc.Spec.LoadBalancerIP != lbIP.String() {
		level.Info(l).Log("event", "clearAssignment", "reason", "differentIPRequested", "msg", "user requested a different IP than the one currently assigned")
		c.clearServiceState(key, svc)
		lbIP = nil
	}

	// If lbIP is still nil at this point, try to allocate.
	if lbIP == nil {
		if !c.synced {
			level.Error(l).Log("op", "allocateIP", "error", "controller not synced", "msg", "controller not synced yet, cannot allocate IP; will retry after sync")
			return false
		}
		ip, err := c.allocateIP(key, svc)
		if err != nil {
			level.Error(l).Log("op", "allocateIP", "error", err, "msg", "IP allocation failed")
			c.client.Errorf(svc, "AllocationFailed", "Failed to allocate IP for %q: %s", key, err)
			// The outer controller loop will retry converging this
			// service when another service gets deleted, so there's
			// nothing to do here but wait to get called again later.
			return true
		}
		lbIP = ip
		level.Info(l).Log("event", "ipAllocated", "ip", lbIP, "msg", "IP address assigned by controller")
		c.client.Infof(svc, "IPAllocated", "Assigned IP %q", lbIP)
	}

	if lbIP == nil {
		level.Error(l).Log("bug", "true", "msg", "internal error: failed to allocate an IP, but did not exit convergeService early!")
		c.client.Errorf(svc, "InternalError", "didn't allocate an IP but also did not fail")
		c.clearServiceState(key, svc)
		return true
	}

	pool := c.ips.Pool(key)
	if pool == "" || c.config.Pools[pool] == nil {
		level.Error(l).Log("bug", "true", "ip", lbIP, "msg", "internal error: allocated IP has no matching address pool")
		c.client.Errorf(svc, "InternalError", "allocated an IP that has no pool")
		c.clearServiceState(key, svc)
		return true
	}

	// At this point, we have an IP selected somehow, all that remains
	// is to program the data plane.
	svc.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{IP: lbIP.String()}}
	return true
}

// clearServiceState clears all fields that are actively managed by
// this controller.
func (c *controller) clearServiceState(key string, svc *v1.Service) {
	c.ips.Unassign(key)
	svc.Status.LoadBalancer = v1.LoadBalancerStatus{}
}

func (c *controller) allocateIP(key string, svc *v1.Service) (net.IP, error) {
	clusterIP := net.ParseIP(svc.Spec.ClusterIP)
	if clusterIP == nil {
		// (we should never get here because the caller ensured that Spec.ClusterIP != nil)
		return nil, fmt.Errorf("invalid ClusterIP [%s], can't determine family", svc.Spec.ClusterIP)
	}
	isIPv6 := clusterIP.To4() == nil

	// If the user asked for a specific IP, try that.
	if svc.Spec.LoadBalancerIP != "" {
		ip := net.ParseIP(svc.Spec.LoadBalancerIP)
		if ip == nil {
			return nil, fmt.Errorf("invalid spec.loadBalancerIP %q", svc.Spec.LoadBalancerIP)
		}
		if (ip.To4() == nil) != isIPv6 {
			return nil, fmt.Errorf("requested spec.loadBalancerIP %q does not match the ipFamily of the service", svc.Spec.LoadBalancerIP)
		}
		if err := c.ips.Assign(key, ip, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			return nil, err
		}
		return ip, nil
	}

	// Otherwise, did the user ask for a specific pool?
	desiredPool := svc.Annotations[annotationAddressPool]
	if desiredPool != "" {
		ip, err := c.ips.AllocateFromPool(key, isIPv6, desiredPool, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
		if err != nil {
			return nil, err
		}
		return ip, nil
	}

	// Okay, in that case just bruteforce across all pools.
	return c.ips.Allocate(key, isIPv6, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
}

// Dual-stack.

func (c *controller) convergeBalancerDual(l log.Logger, key string, svc *v1.Service) bool {
	var lbIP, lbIP2 net.IP

	// The assigned LB IP is the end state of convergence. If there's
	// none or a malformed one, nuke all controlled state so that we
	// start converging from a clean slate.
	if len(svc.Status.LoadBalancer.Ingress) > 1 {
		lbIP = net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
		lbIP2 = net.ParseIP(svc.Status.LoadBalancer.Ingress[1].IP)
	}

	// It's possible the config mutated and the IP we have no longer
	// makes sense. If so, clear it out and give the rest of the logic
	// a chance to allocate again.
	if lbIP != nil && lbIP2 != nil {
		// This assign is idempotent if the config is consistent,
		// otherwise it'll fail and tell us why.
		if err := c.ips.AssignDual(key, lbIP, lbIP2, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			l.Log("event", "clearAssignment", "reason", "notAllowedByConfig", "msg", "current IP not allowed by config, clearing")
			c.clearServiceState(key, svc)
			lbIP = nil
		}

		if lbIP != nil {
			// The user might also have changed the pool annotation, and
			// requested a different pool than the one that is currently
			// allocated.
			desiredPool := svc.Annotations[annotationAddressPool]
			if desiredPool != "" && c.ips.Pool(key) != desiredPool {
				l.Log("event", "clearAssignment", "reason", "differentPoolRequested", "msg", "user requested a different pool than the one currently assigned")
				c.clearServiceState(key, svc)
				lbIP = nil
			}
		}
	} else {
		c.clearServiceState(key, svc)
		lbIP = nil
	}

	lbIP, lbIP2, err := parseRequestedIPs(svc.Annotations[annotationLoadBalancerIPs])
	if err != nil {
		l.Log("op", "allocateIP", "error", err, "msg", "Can't parse requested IPs")
		return true
	} else if lbIP != nil {
		// Try to assign the requested IPs
		if err := c.ips.AssignDual(key, lbIP, lbIP2, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			l.Log("op", "allocateIP", "error", err, "msg", "Can't assign requested IPs")
			return true
		}
	}

	// If lbIP's is still nil at this point, try to allocate.
	if lbIP == nil {
		if !c.synced {
			l.Log("op", "allocateIP", "error", "controller not synced", "msg", "controller not synced yet, cannot allocate IP; will retry after sync")
			return false
		}
		ip, ip2, err := c.allocateIPDual(key, svc)
		if err != nil {
			l.Log("op", "allocateIP", "error", err, "msg", "IP allocation failed")
			c.client.Errorf(svc, "AllocationFailed", "Failed to allocate IP for %q: %s", key, err)
			// The outer controller loop will retry converging this
			// service when another service gets deleted, so there's
			// nothing to do here but wait to get called again later.
			return true
		}
		lbIP = ip
		lbIP2 = ip2
		l.Log("event", "ipAllocated", "ip", lbIP, "ip2", lbIP2, "msg", "IP address assigned by controller")
		c.client.Infof(svc, "IPAllocated", "Assigned IP %q %q", lbIP, lbIP2)
	}

	if lbIP == nil || lbIP2 == nil {
		l.Log("bug", "true", "msg", "internal error: failed to allocate an IP, but did not exit convergeService early!")
		c.client.Errorf(svc, "InternalError", "didn't allocate an IP but also did not fail")
		c.clearServiceState(key, svc)
		return true
	}

	pool := c.ips.Pool(key)
	if pool == "" || c.config.Pools[pool] == nil {
		l.Log("bug", "true", "ip", lbIP, "msg", "internal error: allocated IP has no matching address pool")
		c.client.Errorf(svc, "InternalError", "allocated an IP that has no pool")
		c.clearServiceState(key, svc)
		return true
	}

	// At this point, we have an IP selected somehow, all that remains
	// is to program the data plane.
	svc.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{IP: lbIP.String()}, {IP: lbIP2.String()}}
	return true
}

func (c *controller) allocateIPDual(key string, svc *v1.Service) (net.IP, net.IP, error) {
	desiredPool := svc.Annotations[annotationAddressPool]
	if desiredPool != "" {
		ip, ip2, err := c.ips.AllocateFromPoolDual(key, desiredPool, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
		if err != nil {
			return nil, nil, err
		}
		return ip, ip2, nil
	}

	// Okay, in that case just bruteforce across all pools.
	return c.ips.AllocateDual(key, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
}

func parseRequestedIPs(requestedIPs string) (net.IP, net.IP, error) {
	if requestedIPs == "" {
		return nil, nil, nil
	}
	var lbIP, lbIP2 net.IP
	// requestedIPs must be a comma-separated list of 2 addresses, one from each family.
	ips := strings.Split(requestedIPs, ",")
	if len(ips) != 2 {
		return nil, nil, fmt.Errorf("load-balancer-ips: Must be two addresses")
	}
	if lbIP = net.ParseIP(strings.TrimSpace(ips[0])); lbIP == nil {
		return nil, nil, fmt.Errorf("load-balancer-ips: Invalid address %s", ips[0])
	}
	if lbIP2 = net.ParseIP(strings.TrimSpace(ips[1])); lbIP2 == nil {
		return nil, nil, fmt.Errorf("load-balancer-ips: Invalid address %s", ips[1])
	}
	if (lbIP.To4() == nil) == (lbIP2.To4() == nil) {
		return nil, nil, fmt.Errorf("load-balancer-ips: Same family")
	}
	return lbIP, lbIP2, nil
}
