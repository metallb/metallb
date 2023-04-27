// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"net"

	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/netdev"
	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
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
	framework.ExpectNoError(err)

	nodeObject.Labels[key] = value
	_, err = cs.CoreV1().Nodes().Update(context.Background(), nodeObject, metav1.UpdateOptions{})
	framework.ExpectNoError(err)
}

func RemoveLabelFromNode(nodeName, key string, cs clientset.Interface) {
	nodeObject, err := cs.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	framework.ExpectNoError(err)

	delete(nodeObject.Labels, key)
	_, err = cs.CoreV1().Nodes().Update(context.Background(), nodeObject, metav1.UpdateOptions{})
	framework.ExpectNoError(err)
}
