// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"fmt"
	"net/netip"
	"strings"

	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

// ParseIPToken parses an IP address from the first whitespace-separated token
// of s. Zone identifiers (e.g. fe80::1%eth0) are stripped before parsing.
func ParseIPToken(s string) netip.Addr {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return netip.Addr{}
	}
	ipStr := fields[0]
	if i := strings.Index(ipStr, "%"); i >= 0 {
		ipStr = ipStr[:i]
	}
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

// For IP returns the list of routes in the given container
// (or in the current host) to reach the service ip.
func ForIP(target string, exec executor.Executor) []netip.Addr {
	dst, err := netip.ParseAddr(target)
	Expect(err).NotTo(HaveOccurred(), "failed to convert", target, "to ip")
	dst = dst.Unmap()

	args := []string{"route", "show", dst.String()}
	if !dst.Is4() { // assuming it's an ipv6 address
		args = []string{"-6", "route", "show", dst.String()}
	}
	res, err := exec.Exec("ip", args...)
	Expect(err).NotTo(HaveOccurred())

	routes := make([]netip.Addr, 0)

	rows := strings.Split(res, "\n")
	// The output for a route with a single nexthop looks like: x.x.x.x via x.x.x.x dev x proto bgp metric x
	// Route with multiple nexthops:
	/*
		x.x.x.x proto bgp metric x
		    nexthop via x.x.x.x dev x weight 1
		    nexthop via x.x.x.x dev x weight 1
	*/
	for _, r := range rows {
		if !strings.Contains(r, "via") {
			continue
		}
		via := strings.Split(r, "via")[1] // The IP should be after via
		netIP := ParseIPToken(via)
		if !netIP.IsValid() {
			continue
		}

		routes = append(routes, netIP)
	}

	return routes
}

// MatchNodes tells whether the given list of destination ips
// matches the expected list of nodes.
func MatchNodes(nodes []v1.Node, ips []netip.Addr, ipFamily ipfamily.Family, vrfName string) error {
	nodesIPs := map[netip.Addr]struct{}{}

	ii, err := k8s.NodeIPsForFamily(nodes, ipFamily, vrfName)
	if err != nil {
		return err
	}
	for _, ip := range ii {
		nodesIPs[ip.Unmap()] = struct{}{}
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		if _, ok := nodesIPs[ip]; !ok {
			return fmt.Errorf("IP %s found in routes but not in nodes", ip)
		}
		delete(nodesIPs, ip)
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in routes. Routes %v", nodesIPs, ips)
	}
	return nil
}
