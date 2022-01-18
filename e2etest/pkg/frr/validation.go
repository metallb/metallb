// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"go.universe.tf/metallb/e2etest/pkg/k8s"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	"go.universe.tf/metallb/internal/ipfamily"
)

// NeighborsMatchNodes tells if ALL the given nodes are peered with the
// frr instance. We only care about established connections, as the
// frr instance may be configured with more nodes than are currently
// paired.
func NeighborsMatchNodes(nodes []v1.Node, neighbors []*bgpfrr.Neighbor, ipFamily ipfamily.Family) error {
	nodesIPs := map[string]struct{}{}

	ips := k8s.NodeIPsForFamily(nodes, ipFamily)
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
	}
	for _, n := range neighbors {
		if _, ok := nodesIPs[n.Ip.String()]; !ok { // skipping neighbors that are not nodes
			continue
		}
		if !n.Connected {
			return fmt.Errorf("node %s BGP session not established", n.Ip.String())
		}
		delete(nodesIPs, n.Ip.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than neighbors
		return fmt.Errorf("IP %v found in nodes but not in neighbors", nodesIPs)
	}
	return nil
}

// RoutesMatchNodes tells if ALL the given nodes are exposed as
// destinations for the given address.
func RoutesMatchNodes(nodes []v1.Node, route bgpfrr.Route, ipFamily ipfamily.Family) error {
	nodesIPs := map[string]struct{}{}

	ips := k8s.NodeIPsForFamily(nodes, ipFamily)
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
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

func BFDPeersMatchNodes(nodes []v1.Node, peers map[string]bgpfrr.BFDPeer, ipFamily ipfamily.Family) error {
	nodesIPs := map[string]struct{}{}
	ips := k8s.NodeIPsForFamily(nodes, ipFamily)
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
		if _, ok := peers[ip]; !ok {
			return fmt.Errorf("address %s not found in peers", ip)
		}
	}

	for k := range peers {
		if _, ok := nodesIPs[k]; !ok { // skipping neighbors that are not nodes
			return fmt.Errorf("%s not found in nodes ips", k)
		}
		delete(nodesIPs, k)
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in bfd peers", nodesIPs)
	}
	return nil
}
