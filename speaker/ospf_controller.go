// SPDX-License-Identifier:Apache-2.0

package main

import (
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/epslices"
	k8snodes "go.universe.tf/metallb/internal/k8s/nodes"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ospfController implements the Protocol interface for OSPF. It tracks which
// service IPs should be redistributed into OSPF and keeps the FRR session
// manager in sync.
type ospfController struct {
	logger          log.Logger
	myNode          string
	nodeLabels      labels.Set
	sessionManager  bgp.SessionManager
	ignoreExcludeLB bool

	// activeInstances is the set of OSPFInstance configs that apply to this node.
	activeInstances []*config.OSPFInstance

	// svcPrefixesV4/V6 maps service name → announced prefixes per family.
	svcPrefixesV4 map[string][]string
	svcPrefixesV6 map[string][]string
}

// SetConfig receives the parsed cluster config. It selects the OSPFInstances
// that match this node, then pushes the updated OSPF state to the FRR layer.
func (c *ospfController) SetConfig(l log.Logger, cfg *config.Config) error {
	c.activeInstances = nil
	for _, inst := range cfg.OSPFInstances {
		if c.instanceMatchesNode(inst) {
			c.activeInstances = append(c.activeInstances, inst)
		}
	}
	return c.syncOSPF(l)
}

// ShouldAnnounce returns "" if the service IP should be redistributed into
// OSPF from this node, or a non-empty reason string if not.
func (c *ospfController) ShouldAnnounce(l log.Logger, name string, _ []net.IP, pool *config.Pool, svc *v1.Service, epSlices []discovery.EndpointSlice, nodes map[string]*v1.Node) string {
	if len(c.activeInstances) == 0 {
		return "noOSPFInstance"
	}
	if !poolMatchesNodeOSPF(pool, c.myNode) {
		level.Debug(l).Log("event", "skipping ospf announce", "service", name, "reason", "pool not matching node")
		return "notOwner"
	}
	if len(ospfAdsForService(pool.OSPFAdvertisements, c.myNode, svc)) == 0 {
		level.Debug(l).Log("event", "skipping ospf announce", "service", name, "reason", "no matching OSPFAdvertisement")
		return "noMatchingAdvertisement"
	}
	if k8snodes.IsNetworkUnavailable(nodes[c.myNode]) {
		return "nodeNetworkUnavailable"
	}
	if !c.ignoreExcludeLB && k8snodes.IsNodeExcludedFromBalancers(nodes[c.myNode]) {
		return "nodeLabeledExcludeBalancers"
	}
	filterNode := func(toFilter *string) bool {
		return toFilter == nil || *toFilter != c.myNode
	}
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal && !hasHealthyEndpoint(epSlices, filterNode) {
		return "noLocalEndpoints"
	} else if !hasHealthyEndpoint(epSlices, func(*string) bool { return false }) {
		return "noEndpoints"
	}
	return ""
}

// SetBalancer adds or updates the announced prefixes for a service.
func (c *ospfController) SetBalancer(l log.Logger, name string, lbIPs []net.IP, pool *config.Pool, _ service, svc *v1.Service) error {
	v4, v6 := splitIPsByFamily(lbIPs)
	c.svcPrefixesV4[name] = v4
	c.svcPrefixesV6[name] = v6
	return c.syncOSPF(l)
}

// DeleteBalancer removes the service from the redistribution set.
func (c *ospfController) DeleteBalancer(l log.Logger, name, reason string) error {
	if _, ok := c.svcPrefixesV4[name]; !ok {
		if _, ok2 := c.svcPrefixesV6[name]; !ok2 {
			return nil
		}
	}
	delete(c.svcPrefixesV4, name)
	delete(c.svcPrefixesV6, name)
	return c.syncOSPF(l)
}

// SetNode updates the node labels used for selector matching.
func (c *ospfController) SetNode(l log.Logger, node *v1.Node) error {
	if node.Name != c.myNode {
		return nil
	}
	c.nodeLabels = labels.Set(node.Labels)
	return nil
}

// SetEventCallback is a no-op for OSPF (no async events from FRR layer yet).
func (c *ospfController) SetEventCallback(func(interface{})) {}

// ── helpers ──────────────────────────────────────────────────────────────────

// syncOSPF pushes the current redistribution prefix sets to the FRR session
// manager, which rebuilds the FRR config and triggers a reload.
func (c *ospfController) syncOSPF(l log.Logger) error {
	allV4 := dedupSortedPrefixes(c.svcPrefixesV4)
	allV6 := dedupSortedPrefixes(c.svcPrefixesV6)

	params := make([]bgp.OSPFInstanceParams, 0, len(c.activeInstances))
	for _, inst := range c.activeInstances {
		p := bgp.OSPFInstanceParams{
			RouterID:   inst.RouterID,
			VRF:        inst.VRF,
			PrefixesV4: allV4,
			PrefixesV6: allV6,
		}
		for _, a := range inst.Areas {
			p.Areas = append(p.Areas, bgp.OSPFAreaParams{ID: a.ID, Type: a.Type})
		}
		for _, iface := range inst.Interfaces {
			ip := bgp.OSPFInterfaceParams{
				Name:    iface.Name,
				AreaID:  iface.AreaID,
				Passive: iface.Passive,
				Cost:    iface.Cost,
			}
			if iface.HelloInterval != nil {
				secs := int64(*iface.HelloInterval / time.Second)
				ip.HelloInterval = &secs
			}
			if iface.DeadInterval != nil {
				secs := int64(*iface.DeadInterval / time.Second)
				ip.DeadInterval = &secs
			}
			p.Interfaces = append(p.Interfaces, ip)
		}
		params = append(params, p)
	}

	if err := c.sessionManager.SyncOSPFInstances(params); err != nil {
		level.Error(l).Log("op", "syncOSPF", "error", err, "msg", "failed to sync OSPF instances")
		return fmt.Errorf("syncing OSPF instances: %w", err)
	}
	return nil
}

func (c *ospfController) instanceMatchesNode(inst *config.OSPFInstance) bool {
	for _, sel := range inst.NodeSelectors {
		if sel.Matches(c.nodeLabels) {
			return true
		}
	}
	return false
}

// poolMatchesNodeOSPF returns true if the pool has at least one
// OSPFAdvertisement that permits the given node.
func poolMatchesNodeOSPF(pool *config.Pool, nodeName string) bool {
	for _, adv := range pool.OSPFAdvertisements {
		if len(adv.Nodes) == 0 || adv.Nodes[nodeName] {
			return true
		}
	}
	return false
}

// ospfAdsForService returns the OSPFAdvertisements from the pool that match
// both the given node and the service labels.
func ospfAdsForService(ads []*config.OSPFAdvertisement, nodeName string, svc *v1.Service) []*config.OSPFAdvertisement {
	var matched []*config.OSPFAdvertisement
	svcLabels := labels.Set(svc.Labels)
	for _, adv := range ads {
		if len(adv.Nodes) > 0 && !adv.Nodes[nodeName] {
			continue
		}
		if len(adv.ServiceSelectors) > 0 {
			anyMatch := false
			for _, sel := range adv.ServiceSelectors {
				if sel.Matches(svcLabels) {
					anyMatch = true
					break
				}
			}
			if !anyMatch {
				continue
			}
		}
		matched = append(matched, adv)
	}
	return matched
}

// splitIPsByFamily converts a slice of net.IP into sorted /32 (IPv4) and /128
// (IPv6) CIDR strings for use in prefix-lists.
func splitIPsByFamily(ips []net.IP) (v4, v6 []string) {
	for _, ip := range ips {
		if ip.To4() != nil {
			v4 = append(v4, ip.String()+"/32")
		} else {
			v6 = append(v6, ip.String()+"/128")
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)
	return
}

// dedupSortedPrefixes merges all per-service prefix slices into a single
// sorted, deduplicated slice.
func dedupSortedPrefixes(svcPrefixes map[string][]string) []string {
	seen := map[string]bool{}
	for _, pfxs := range svcPrefixes {
		for _, p := range pfxs {
			seen[p] = true
		}
	}
	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// epslices is referenced via the shared hasHealthyEndpoint in bgp_controller.go.
var _ = epslices.EndpointCanServe
