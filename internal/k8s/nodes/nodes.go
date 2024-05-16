// SPDX-License-Identifier:Apache-2.0

package nodes

import (
	"errors"
	"net"

	"go.universe.tf/metallb/internal/ipfamily"
	corev1 "k8s.io/api/core/v1"
)

func IsNodeAvailable(n *corev1.Node) error {
	if IsNodeUnschedulable(n) {
		return errors.New("nodeUnschedulable")
	}

	if IsNetworkUnavailable(n) {
		return errors.New("nodeNetworkUnavailable")
	}

	return nil
}

func IsNodeUnschedulable(n *corev1.Node) bool {
	if n == nil {
		return false
	}

	return n.Spec.Unschedulable
}

// IsNetworkUnavailable returns true if the given node NodeNetworkUnavailable condition status is true.
func IsNetworkUnavailable(n *corev1.Node) bool {
	return conditionStatus(n, corev1.NodeNetworkUnavailable) == corev1.ConditionTrue
}

// conditionStatus returns the status of the condition for a given node.
func conditionStatus(n *corev1.Node, ct corev1.NodeConditionType) corev1.ConditionStatus {
	if n == nil {
		return corev1.ConditionUnknown
	}

	for _, c := range n.Status.Conditions {
		if c.Type == ct {
			return c.Status
		}
	}

	return corev1.ConditionUnknown
}

// IsNodeExcludedFromBalancers returns true if the given node has labeld node.kubernetes.io/exclude-from-external-load-balancers".
func IsNodeExcludedFromBalancers(n *corev1.Node) bool {
	if n == nil {
		return false
	}

	if _, ok := n.Labels[corev1.LabelNodeExcludeBalancers]; ok {
		return true
	}
	return false
}

// NodeIPsForFamily returns all input node nodeIP based on ipfamily.
func NodeIPsForFamily(nodes []corev1.Node, family ipfamily.Family) []net.IP {
	var nodeIPs []net.IP
	for _, n := range nodes {
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				nodeIP := net.ParseIP(a.Address)
				if family != ipfamily.DualStack && ipfamily.ForAddress(nodeIP) != family {
					continue
				}
				nodeIPs = append(nodeIPs, nodeIP)
			}
		}
	}
	return nodeIPs
}
