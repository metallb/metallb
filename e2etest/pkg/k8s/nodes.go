// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"net"

	v1 "k8s.io/api/core/v1"
)

func NodeIPsForFamily(nodes []v1.Node, family string) []string {
	res := []string{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				if family != "dual" && IPFamilyForAddress(a.Address) != family {
					continue
				}
				res = append(res, a.Address)
			}
		}
	}
	return res
}

func IPFamilyForAddress(ip string) string {
	ipNet := net.ParseIP(ip)
	if ipNet.To4() == nil {
		return "ipv6"
	}
	return "ipv4"
}
