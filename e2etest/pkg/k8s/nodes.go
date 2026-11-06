// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/netdev"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func NodeIPsForFamily(nodes []v1.Node, family ipfamily.Family, vrfName string) ([]string, error) {
	res := []string{}
	for _, n := range nodes {
		// If the peer is supposed to be connected via a VRF, taking the node address is not enough.
		// We need to find the ip associated to the interface inside the VRF
		if vrfName != "" {
			exec := executor.ForContainer(n.Name)
			dev, err := netdev.WithMaster(exec, vrfName)
			if err != nil {
				return nil, err
			}
			addr, err := netdev.AddressesForDevice(exec, dev)
			if err != nil {
				return nil, err
			}
			switch family {
			case ipfamily.IPv4:
				res = append(res, addr.IPV4Address)
			case ipfamily.IPv6:
				res = append(res, addr.IPV6Address)
			case ipfamily.DualStack:
				res = append(res, addr.IPV4Address)
				res = append(res, addr.IPV6Address)
			}
			continue
		}
		for _, a := range n.Status.Addresses {
			if a.Type == v1.NodeInternalIP {
				if family != ipfamily.DualStack && ipfamily.ForAddress(net.ParseIP(a.Address)) != family {
					continue
				}
				res = append(res, a.Address)
			}
		}
	}
	return res, nil
}

func SelectorsForNodes(nodes []v1.Node) []metav1.LabelSelector {
	selectors := []metav1.LabelSelector{}
	if len(nodes) == 0 {
		return []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					"non": "existent",
				},
			},
		}
	}
	for _, node := range nodes {
		selectors = append(selectors, metav1.LabelSelector{
			MatchLabels: map[string]string{
				"kubernetes.io/hostname": node.GetLabels()["kubernetes.io/hostname"],
			},
		})
	}
	return selectors
}

func AddLabelToNode(nodeName, key, value string, cs clientset.Interface) {
	nodeObject, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	nodeObject.Labels[key] = value
	_, err = cs.CoreV1().Nodes().Update(context.Background(), nodeObject, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func RemoveLabelFromNode(nodeName, key string, cs clientset.Interface) {
	nodeObject, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	delete(nodeObject.Labels, key)
	_, err = cs.CoreV1().Nodes().Update(context.Background(), nodeObject, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

// AddTaintToNode adds the given taint to the node, doing nothing if an identical taint is already there.
func AddTaintToNode(cs clientset.Interface, nodeName string, taint v1.Taint) (bool, error) {
	ginkgo.By(fmt.Sprintf("adding the taint %s=%s:%s to node %s", taint.Key, taint.Value, taint.Effect, nodeName))

	added := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, t := range node.Spec.Taints {
			if t.Key == taint.Key && t.Value == taint.Value && t.Effect == taint.Effect {
				return nil
			}
		}
		node.Spec.Taints = append(append([]v1.Taint(nil), node.Spec.Taints...), taint)
		_, err = cs.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
		if err == nil {
			added = true
		}
		return err
	})
	return added, err
}

// RemoveTaintFromNode removes the given taint from the node.

func RemoveTaintFromNode(cs clientset.Interface, nodeName string, taint v1.Taint) error {
	ginkgo.By(fmt.Sprintf("removing the taint %s=%s:%s from node %s", taint.Key, taint.Value, taint.Effect, nodeName))

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		remaining := []v1.Taint{}
		for _, t := range node.Spec.Taints {
			if t.Key == taint.Key && t.Value == taint.Value && t.Effect == taint.Effect {
				continue
			}
			remaining = append(remaining, t)
		}
		if len(remaining) == len(node.Spec.Taints) {
			return nil
		}
		node.Spec.Taints = remaining
		_, err = cs.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
		return err
	})
}

// SetNodeCondition sets the node's condition to the desired status and validates that the change is applied.
func SetNodeCondition(cs clientset.Interface, nodeName string, conditionType v1.NodeConditionType, status v1.ConditionStatus) error {
	ginkgo.By(fmt.Sprintf("setting the %s condition to %s on node %s", conditionType, status, nodeName))

	condition := v1.NodeCondition{
		Type:               conditionType,
		Status:             status,
		Reason:             "Testing",
		Message:            fmt.Sprintf("This condition is %s for testing", status),
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
	}

	raw, err := json.Marshal(&[]v1.NodeCondition{condition})
	if err != nil {
		return fmt.Errorf("failed to set condition %s on node %s: %s", conditionType, nodeName, err)
	}

	Eventually(func() error {
		patch := []byte(fmt.Sprintf(`{"status":{"conditions":%s}}`, raw))
		_, err = cs.CoreV1().Nodes().PatchStatus(context.Background(), nodeName, patch)
		if err != nil {
			return fmt.Errorf("failed to set condition %s on node %s: %s", conditionType, nodeName, err)
		}

		n, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node %s: %s", nodeName, err)
		}

		gotStatus := conditionStatus(n, conditionType)
		if status != gotStatus {
			return fmt.Errorf("failed: got unexpected %s status on node %s", conditionType, nodeName)
		}

		return nil
	}, time.Minute, 3*time.Second).ShouldNot(HaveOccurred())

	return nil
}

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
