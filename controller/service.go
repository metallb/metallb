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
	"reflect"
	"sort"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	v1 "k8s.io/api/core/v1"

	"go.universe.tf/metallb/internal/allocator/k8salloc"
	"go.universe.tf/metallb/internal/ipfamily"
)

const (
	AnnotationPrefix             = "metallb.universe.tf"
	AnnotationAddressPool        = AnnotationPrefix + "/" + "address-pool"
	AnnotationLoadBalancerIPs    = AnnotationPrefix + "/" + "loadBalancerIPs"
	AnnotationIPAllocateFromPool = AnnotationPrefix + "/" + "ip-allocated-from-pool"
)

var ErrConverge = fmt.Errorf("failed to converge")

func (c *controller) convergeBalancer(l log.Logger, key string, svc *v1.Service) error {
	lbIPs := []net.IP{}
	var err error
	// Not a LoadBalancer, early exit. It might have been a balancer
	// in the past, so we still need to clear LB state.
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		level.Debug(l).Log("event", "clearAssignment", "reason", "notLoadBalancer", "msg", "not a LoadBalancer")
		c.clearServiceState(key, svc)
		// Early return, we explicitly do *not* want to reallocate
		// an IP.
		return nil
	}

	// Return if pools are empty.
	if len(c.pools.ByName) == 0 {
		level.Debug(l).Log("event", "clearAssignment", "reason", "noConfig", "msg", "pools are empty")
		c.clearServiceState(key, svc)
		return ErrConverge
	}

	// If the ClusterIPs is malformed or not set we can't determine the
	// ipFamily to use.
	if len(svc.Spec.ClusterIPs) == 0 && svc.Spec.ClusterIP == "" {
		level.Info(l).Log("event", "clearAssignment", "reason", "noClusterIPs", "msg", "No ClusterIPs")
		c.clearServiceState(key, svc)
		return ErrConverge
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
	if len(lbIPs) == 0 {
		c.clearServiceState(key, svc)
	} else {
		lbIPsIPFamily, err := ipfamily.ForAddressesIPs(lbIPs)
		if err != nil {
			level.Error(l).Log("event", "clearAssignment", "reason", "nolbIPsIPFamily", "msg", "Failed to retrieve lbIPs family")
			c.client.Errorf(svc, "nolbIPsIPFamily", "Failed to retrieve LBIPs IPFamily for %q: %s", lbIPs, err)
		}
		clusterIPsIPFamily, err := ipfamily.ForService(svc)
		if err != nil {
			level.Error(l).Log("event", "clearAssignment", "reason", "noclusterIPsIPFamily", "msg", "Failed to retrieve clusterIPs family")
			c.client.Errorf(svc, "noclusterIPsIPFamily", "Failed to retrieve ClusterIPs IPFamily for %q %s: %s", svc.Spec.ClusterIPs, svc.Spec.ClusterIP, err)
			return ErrConverge
		}
		// Clear the lbIP if it has a different ipFamily compared to the clusterIP.
		// (this should not happen since the "ipFamily" of a service is immutable)
		if lbIPsIPFamily != clusterIPsIPFamily ||
			lbIPsIPFamily == ipfamily.Unknown {
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
		if err = c.ips.Assign(key, svc, lbIPs, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			level.Info(l).Log("event", "clearAssignment", "error", err, "msg", "current IP not allowed by config, clearing")
			c.client.Infof(svc, "ClearAssignment", "current IP for %q not allowed by config, will attempt for new IP assignment: %s", key, err)
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}

		// The user might also have changed the pool annotation, and
		// requested a different pool than the one that is currently
		// allocated.
		desiredPool := svc.Annotations[AnnotationAddressPool]
		if len(lbIPs) != 0 && desiredPool != "" && c.ips.Pool(key) != desiredPool {
			level.Info(l).Log("event", "clearAssignment", "reason", "differentPoolRequested", "msg", "user requested a different pool than the one currently assigned")
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}
		// User set or changed the desired LB IP(s), nuke the
		// state. allocateIP will pay attention to LoadBalancerIP(s) and try
		// to meet the user's demands.
		desiredLbIPs, _, err := getDesiredLbIPs(svc)
		if err != nil {
			level.Error(l).Log("event", "loadbalancerIP", "error", err, "msg", "invalid requested loadbalancer IPs")
			c.client.Errorf(svc, "LoadBalancerFailed", "invalid requested loadbalancer IPs: %s", err)
			return ErrConverge
		}
		if len(desiredLbIPs) > 0 && !isEqualIPs(lbIPs, desiredLbIPs) {
			level.Info(l).Log("event", "clearAssignment", "reason", "differentIPRequested", "msg", "user requested a different IP than the one currently assigned")
			c.clearServiceState(key, svc)
			lbIPs = []net.IP{}
		}
	}

	// If lbIP is still nil at this point, try to allocate.
	if len(lbIPs) == 0 {
		lbIPs, err = c.allocateIPs(key, svc)
		if err != nil {
			level.Error(l).Log("op", "allocateIPs", "error", err, "msg", "IP allocation failed")
			c.client.Errorf(svc, "AllocationFailed", "Failed to allocate IP for %q: %s", key, err)
			// The outer controller loop will retry converging this
			// service when another service gets deleted, so there's
			// nothing to do here but wait to get called again later.
			return ErrConverge
		}
		level.Info(l).Log("event", "ipAllocated", "ip", lbIPs, "msg", "IP address assigned by controller")
		c.client.Infof(svc, "IPAllocated", "Assigned IP %q", lbIPs)
	}

	if len(lbIPs) == 0 {
		level.Error(l).Log("bug", "true", "msg", "internal error: failed to allocate an IP, but did not exit convergeService early!")
		c.client.Errorf(svc, "InternalError", "didn't allocate an IP but also did not fail")
		c.clearServiceState(key, svc)
		return ErrConverge
	}

	pool := c.ips.Pool(key)
	if pool == "" || c.pools == nil || c.pools.IsEmpty(pool) {
		level.Error(l).Log("bug", "true", "ip", lbIPs, "msg", "internal error: allocated IP has no matching address pool")
		c.client.Errorf(svc, "InternalError", "allocated an IP that has no pool")
		c.clearServiceState(key, svc)
		return ErrConverge
	}

	// At this point, we have an IP selected somehow, all that remains
	// is to program the data plane.
	lbIngressIPs := []v1.LoadBalancerIngress{}
	for _, lbIP := range lbIPs {
		lbIngressIPs = append(lbIngressIPs, v1.LoadBalancerIngress{IP: lbIP.String()})
	}
	svc.Status.LoadBalancer.Ingress = lbIngressIPs
	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}
	svc.Annotations[AnnotationIPAllocateFromPool] = pool

	return nil
}

// clearServiceState clears all fields that are actively managed by
// this controller.
func (c *controller) clearServiceState(key string, svc *v1.Service) {
	c.ips.Unassign(key)
	delete(svc.Annotations, AnnotationIPAllocateFromPool)
	svc.Status.LoadBalancer = v1.LoadBalancerStatus{}
}

func (c *controller) allocateIPs(key string, svc *v1.Service) ([]net.IP, error) {
	if len(svc.Spec.ClusterIPs) == 0 && svc.Spec.ClusterIP == "" {
		// (we should never get here because the caller ensured that Spec.ClusterIP != nil)
		return nil, fmt.Errorf("invalid ClusterIPs [%v] [%s], can't determine family", svc.Spec.ClusterIPs, svc.Spec.ClusterIP)
	}

	serviceIPFamily, err := ipfamily.ForService(svc)
	if err != nil {
		return nil, err
	}

	desiredLbIPs, desiredLbIPFamily, err := getDesiredLbIPs(svc)
	if err != nil {
		return nil, err
	}

	desiredPool := svc.Annotations[AnnotationAddressPool]

	// If the user asked for a specific IPs, try that.
	if len(desiredLbIPs) > 0 {
		if serviceIPFamily != desiredLbIPFamily {
			return nil, fmt.Errorf("requested loadBalancer IP(s) %q does not match the ipFamily of the service", desiredLbIPs)
		}
		if err := c.ips.Assign(key, svc, desiredLbIPs, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc)); err != nil {
			return nil, err
		}

		// Verify that ip and address pool annotations are compatible.
		if desiredPool != "" && c.ips.Pool(key) != desiredPool {
			c.ips.Unassign(key)
			return nil, fmt.Errorf("requested loadBalancer IP(s) %q is not compatible with requested address pool %s", desiredLbIPs, desiredPool)
		}

		return desiredLbIPs, nil
	}

	// Assign ip from requested address pool.
	if desiredPool != "" {
		ips, err := c.ips.AllocateFromPool(key, svc, serviceIPFamily, desiredPool, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
		if err != nil {
			return nil, err
		}
		return ips, nil
	}

	// Okay, in that case just bruteforce across all pools.
	return c.ips.Allocate(key, svc, serviceIPFamily, k8salloc.Ports(svc), k8salloc.SharingKey(svc), k8salloc.BackendKey(svc))
}

func (c *controller) isServiceAllocated(key string) bool {
	return c.ips.Pool(key) != ""
}

func getDesiredLbIPs(svc *v1.Service) ([]net.IP, ipfamily.Family, error) {
	var desiredLbIPs []net.IP
	desiredLbIPsStr := svc.Annotations[AnnotationLoadBalancerIPs]

	if desiredLbIPsStr == "" && svc.Spec.LoadBalancerIP == "" {
		return nil, "", nil
	} else if desiredLbIPsStr != "" && svc.Spec.LoadBalancerIP != "" {
		return nil, "", fmt.Errorf("service can not have both %s and svc.Spec.LoadBalancerIP", AnnotationLoadBalancerIPs)
	}

	if desiredLbIPsStr != "" {
		desiredLbIPsSlice := strings.Split(desiredLbIPsStr, ",")
		for _, desiredLbIPStr := range desiredLbIPsSlice {
			desiredLbIP := net.ParseIP(strings.TrimSpace(desiredLbIPStr))
			if desiredLbIP == nil {
				return nil, "", fmt.Errorf("invalid %s: %q", AnnotationLoadBalancerIPs, desiredLbIPsStr)
			}
			desiredLbIPs = append(desiredLbIPs, desiredLbIP)
		}
		desiredLbIPFamily, err := ipfamily.ForAddressesIPs(desiredLbIPs)
		if err != nil {
			return nil, "", err
		}
		return desiredLbIPs, desiredLbIPFamily, nil
	}

	desiredLbIP := net.ParseIP(svc.Spec.LoadBalancerIP)
	if desiredLbIP == nil {
		return nil, "", fmt.Errorf("invalid spec.loadBalancerIP %q", svc.Spec.LoadBalancerIP)
	}
	desiredLbIPs = append(desiredLbIPs, desiredLbIP)
	desiredLbIPFamily := ipfamily.ForAddress(desiredLbIP)

	return desiredLbIPs, desiredLbIPFamily, nil
}

func isEqualIPs(ipsA, ipsB []net.IP) bool {
	sort.Slice(ipsA, func(i, j int) bool {
		return ipsA[i].String() < ipsA[j].String()
	})
	sort.Slice(ipsB, func(i, j int) bool {
		return ipsB[i].String() < ipsB[j].String()
	})
	return reflect.DeepEqual(ipsA, ipsB)
}
