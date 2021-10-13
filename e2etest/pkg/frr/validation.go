// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// NeighborsMatchNodes tells if ALL the given nodes are peered with the
// frr instance. We only care about established connections, as the
// frr instance may be configured with more nodes than are currently
// paired.
func NeighborsMatchNodes(nodes []v1.Node, neighbors []*Neighbor) error {
	nodesIPs := map[string]struct{}{}

	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				nodesIPs[a.Address] = struct{}{}
			}
		}
	}
	for _, n := range neighbors {
		if _, ok := nodesIPs[n.ip.String()]; !ok { // skipping neighbors that are not nodes
			continue
		}
		if !n.connected {
			return fmt.Errorf("node %s BGP session not established", n.ip.String())
		}
		delete(nodesIPs, n.ip.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than neighbors
		return fmt.Errorf("IP %v found in nodes but not in neighbors", nodesIPs)
	}
	return nil
}

// RoutesMatchNodes tells if ALL the given nodes are exposed as
// destinations for the given address.
func RoutesMatchNodes(nodes []v1.Node, route Route) error {
	nodesIPs := map[string]struct{}{}

	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				nodesIPs[a.Address] = struct{}{}
			}
		}
	}
	for _, h := range route.NextHops {
		if _, ok := nodesIPs[h.String()]; !ok { // skipping neighbors that are not nodes
			return fmt.Errorf("%s not found in nodes ips", h.String())
		}

		delete(nodesIPs, h.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in next hops", nodesIPs)
	}
	return nil
}
