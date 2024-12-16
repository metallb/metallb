// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
)

// NeighborsMatchNodes tells if ALL the given nodes are peered with the
// frr instance. We only care about established connections, as the
// frr instance may be configured with more nodes than are currently
// paired.
func NeighborsMatchNodes(nodes []v1.Node, neighbors NeighborsMap, ipFamily ipfamily.Family, vrfName string) error {
	nodesIPs := map[string]struct{}{}

	ips, err := k8s.NodeIPsForFamily(nodes, ipFamily, vrfName)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
	}
	for _, n := range neighbors {
		if _, ok := nodesIPs[n.ID]; !ok { // skipping neighbors that are not nodes
			continue
		}
		if !n.Connected {
			return fmt.Errorf("node %s BGP session not established", n.ID)
		}
		delete(nodesIPs, n.ID)
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than neighbors
		return fmt.Errorf("vrfName=%s ,nodeIPS=%v vs nextHops=%v\n", vrfName, ips, neighbors)
	}
	return nil
}

// RoutesMatchNodes tells if ALL the given nodes are exposed as
// destinations for the given address.
func RoutesMatchNodes(nodes []v1.Node, route Route, ipFamily ipfamily.Family, vrfName string) error {
	nodesIPs := map[string]struct{}{}

	ips, err := k8s.NodeIPsForFamily(nodes, ipFamily, vrfName)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
	}

	for _, h := range route.NextHops {
		if _, ok := nodesIPs[h.String()]; !ok { // skipping neighbors that are not nodes
			return fmt.Errorf("%s not found in nodes ips, %v", h.String(), nodesIPs)
		}

		delete(nodesIPs, h.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("vrfName=%s ,nodeIPS=%v vs nextHops=%v\n", vrfName, ips, route.NextHops)
	}
	return nil
}

func BFDPeersMatchNodes(nodes []v1.Node, peers map[string]BFDPeer, ipFamily ipfamily.Family, vrfName string) error {
	nodesIPs := map[string]struct{}{}
	ips, err := k8s.NodeIPsForFamily(nodes, ipFamily, vrfName)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		nodesIPs[ip] = struct{}{}
		if _, ok := peers[ip]; !ok {
			return fmt.Errorf("address %s not found in peers", ip)
		}
	}

	for k := range peers {
		if _, ok := nodesIPs[k]; !ok { // skipping neighbors that are not nodes
			return fmt.Errorf("%s not found in nodes ips %v", k, nodesIPs)
		}
		delete(nodesIPs, k)
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in bfd peers", nodesIPs)
	}
	return nil
}
