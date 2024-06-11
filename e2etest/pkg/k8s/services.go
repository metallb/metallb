// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"sort"
	"strings"

	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// GetSvcNode returns the node that the LB Service announcing from.
func GetSvcNode(cs clientset.Interface, svcNS string, svcName string, allNodes *corev1.NodeList) (*corev1.Node, error) {
	events, err := cs.CoreV1().Events(svcNS).List(context.Background(), metav1.ListOptions{FieldSelector: "reason=nodeAssigned"})
	if err != nil {
		return nil, err
	}

	svcEvents := make([]corev1.Event, 0)
	for _, e := range events.Items {
		if e.InvolvedObject.Name == svcName {
			svcEvents = append(svcEvents, e)
		}
	}
	if len(svcEvents) == 0 {
		return nil, errors.New("service doesn't be assigned node")
	}

	sort.Slice(svcEvents, func(i, j int) bool {
		return svcEvents[i].LastTimestamp.After(svcEvents[j].LastTimestamp.Time)
	})

	msg := svcEvents[0].Message

	for _, node := range allNodes.Items {
		if strings.Contains(msg, "\""+node.Name+"\"") {
			return &node, nil
		}
	}
	return nil, errors.New("can't find the node that service be assigned")
}
