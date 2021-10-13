// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var (
	ipv4Re *regexp.Regexp
	ipv6Re *regexp.Regexp
)

func init() {
	ipv4Re = regexp.MustCompile(`(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|)){4})`)
	ipv6Re = regexp.MustCompile(`(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`)
}

// For IP returns the list of routes in the given container
// (or in the current host) to reach the service ip.
func ForIP(target string, exec executor.Executor) []net.IP {
	res, err := exec.Exec("ip", []string{"route", "show", target}...)
	framework.ExpectNoError(err)

	routes := make([]net.IP, 0)

	dst := net.ParseIP(target)
	framework.ExpectNotEqual(dst, nil, "Failed to convert", target, "to ip")

	re := ipv4Re
	if dst.To4() == nil { // assuming it's an ipv6 address
		re = ipv6Re
	}
	rows := strings.Split(res, "\n")
	for _, r := range rows {
		if !strings.Contains(r, "nexthop via") {
			continue
		}
		ip := re.FindString(r)
		if ip == "" {
			continue
		}
		netIP := net.ParseIP(ip)
		framework.ExpectNotEqual(netIP, nil, "Failed to convert", ip, "to ip")
		routes = append(routes, netIP)
	}

	return routes
}

// MatchNodes tells whether the given list of destination ips
// matches the expected list of nodes.
func MatchNodes(nodes []v1.Node, ips []net.IP) error {
	nodesIPs := map[string]struct{}{}

	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				nodesIPs[a.Address] = struct{}{}
			}
		}
	}
	for _, ip := range ips {
		if _, ok := nodesIPs[ip.String()]; !ok {
			return fmt.Errorf("IP %s found in routes but not in nodes", ip.String())
		}
		delete(nodesIPs, ip.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in routes", nodesIPs)
	}
	return nil
}
