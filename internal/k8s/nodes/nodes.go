// SPDX-License-Identifier:Apache-2.0

package nodes

import (
	corev1 "k8s.io/api/core/v1"
)

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
