// SPDX-License-Identifier:Apache-2.0

package nodes

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
)

func IsNodeAvailable(n *corev1.Node) error {
	// if IsNodeUnschedulable(n) {
	// 	return errors.New("nodeUnschedulable")
	// }

	if IsNetworkUnavailable(n) {
		return errors.New("nodeNetworkUnavailable")
	}

	return nil
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

func IsNodeExcludedFromBalancers2(n, pattern *corev1.Node) bool {
	if n == nil || pattern == nil {
		return false
	}

	// DISCUSS: exclude on Unschedulable

	// DISCUSS: should do full match all labels exists or once a label exists
	// exclude on Labels
	for k, v := range pattern.Labels {
		if nv, exists := n.Labels[k]; exists && nv == v {
			return true
		}
	}

	// DISCUSS: should do full match
	// if annotation of pattern is subset of annotation of node, return true
	for k, v := range pattern.Annotations {
		if nv, exists := n.Annotations[k]; exists && nv == v {
			return true
		}
	}

	// DISCUSS exclude on taints? (no)
	// DISCUSS exclude in conditions? (yes)

	return false
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
