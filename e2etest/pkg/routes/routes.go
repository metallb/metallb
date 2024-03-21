// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

var (
	Ipv4Re *regexp.Regexp
	Ipv6Re *regexp.Regexp
)

func init() {
	Ipv4Re = regexp.MustCompile(`(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.)){3})(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)
	Ipv6Re = regexp.MustCompile(`(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])).`)
}

// For IP returns the list of routes in the given container
// (or in the current host) to reach the service ip.
func ForIP(target string, exec executor.Executor) []net.IP {
	dst := net.ParseIP(target)
	Expect(dst).NotTo(Equal(nil), "failed to convert", target, "to ip")

	re := Ipv4Re
	res, err := exec.Exec("ip", []string{"route", "show", target}...)
	Expect(err).NotTo(HaveOccurred())

	if dst.To4() == nil { // assuming it's an ipv6 address
		re = Ipv6Re
		res, err = exec.Exec("ip", []string{"-6", "route", "show", target}...)
	}
	Expect(err).NotTo(HaveOccurred())

	routes := make([]net.IP, 0)

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
		ip := re.FindString(via)
		if ip == "" {
			continue
		}
		netIP := net.ParseIP(ip)
		Expect(netIP).NotTo(Equal(nil), "failed to convert", target, "to ip")

		routes = append(routes, netIP)
	}

	return routes
}

// MatchNodes tells whether the given list of destination ips
// matches the expected list of nodes.
func MatchNodes(nodes []v1.Node, ips []net.IP, ipFamily ipfamily.Family, vrfName string) error {
	nodesIPs := map[string]struct{}{}

	ii, err := k8s.NodeIPsForFamily(nodes, ipFamily, vrfName)
	if err != nil {
		return err
	}
	for _, ip := range ii {
		nodesIPs[ip] = struct{}{}
	}
	for _, ip := range ips {
		if _, ok := nodesIPs[ip.String()]; !ok {
			return fmt.Errorf("IP %s found in routes but not in nodes", ip.String())
		}
		delete(nodesIPs, ip.String())
	}
	if len(nodesIPs) != 0 { // some leftover, meaning more nodes than routes
		return fmt.Errorf("IP %v found in nodes but not in routes. Routes %v", nodesIPs, ips)
	}
	return nil
}
