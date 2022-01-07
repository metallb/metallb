// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"net"

	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
)

func NodeIPsForFamily(nodes []v1.Node, family ipfamily.Family) []string {
	res := []string{}
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				if family != ipfamily.DualStack && ipfamily.ForAddress(net.ParseIP(a.Address)) != family {
					continue
				}
				res = append(res, a.Address)
			}
		}
	}
	return res
}
