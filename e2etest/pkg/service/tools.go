// SPDX-License-Identifier:Apache-2.0

package service

import v1 "k8s.io/api/core/v1"

func GetIngressPoint(ing *v1.LoadBalancerIngress) string {
	host := ing.IP
	if host == "" {
		host = ing.Hostname
	}
	return host
}
