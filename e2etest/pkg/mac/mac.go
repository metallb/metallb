// SPDX-License-Identifier:Apache-2.0

package mac

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/routes"

	corev1 "k8s.io/api/core/v1"
)

// ForIP returns the MAC address of a given IP.
func ForIP(ip string, exec executor.Executor) (net.HardwareAddr, error) {
	macRe := regexp.MustCompile("([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})")

	res, err := exec.Exec("ip", []string{"neigh", "show", ip}...)
	if err != nil {
		return nil, err
	}

	rows := strings.Split(res, "\n")
	for _, r := range rows {
		m := macRe.FindString(r)
		if m == "" {
			continue
		}
		mac, err := net.ParseMAC(m)
		if err != nil {
			return nil, err
		}
		return mac, nil
	}

	return nil, fmt.Errorf("could not resolve MAC for ip %s", ip)
}

// MatchNode returns the node with the given MAC address.
func MatchNode(nodes []corev1.Node, mac net.HardwareAddr, exec executor.Executor) (*corev1.Node, error) {
	res, err := exec.Exec("ip", []string{"neigh", "show"}...)
	if err != nil {
		return nil, err
	}

	nodesIPs := map[string]corev1.Node{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				nodesIPs[a.Address] = n
			}
		}
	}

	rows := strings.Split(res, "\n")
	// The output of ip neigh show looks like:
	/*
		...
		172.18.0.4 dev br-97bb56038aab lladdr 02:42:ac:12:00:04 REACHABLE
		fe80::42:acff:fe12:3 dev br-97bb56038aab lladdr 02:42:ac:12:00:03 router REACHABLE
		...
	*/
	for _, r := range rows {
		if !strings.Contains(r, mac.String()) {
			continue
		}

		ip := routes.Ipv4Re.FindString(r)
		if ip == "" {
			ip = routes.Ipv6Re.FindString(r)
		}

		if ip == "" {
			continue
		}

		netIP := net.ParseIP(ip)
		if netIP == nil {
			return nil, fmt.Errorf("failed to convert %s to net.IP", ip)
		}

		if n, ok := nodesIPs[netIP.String()]; ok {
			return &n, nil
		}
	}

	return nil, fmt.Errorf("no node found for MAC: %s", mac)
}

// UpdateNodeCache pings all the nodes to update the address resolution cache.
func UpdateNodeCache(nodes []corev1.Node, exec executor.Executor) error {
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				_, err := exec.Exec("ping", []string{a.Address, "-c", "1"}...)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
