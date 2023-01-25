// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
)

// DescribeSvc logs the output of kubectl describe svc for the given namespace.
func DescribeSvc(ns string) {
	framework.Logf("\nOutput of kubectl describe svc:\n")
	desc, _ := kubectl.RunKubectl(
		ns, "describe", "svc", fmt.Sprintf("--namespace=%v", ns))
	framework.Logf(desc)
}

// GetSvcNode returns the node that the LB Service announcing from.
func GetSvcNode(cs clientset.Interface, svcNS string, svcName string, allNodes *corev1.NodeList) (*corev1.Node, error) {
	events, err := cs.CoreV1().Events(svcNS).List(context.Background(), metav1.ListOptions{FieldSelector: "reason=nodeAssigned"})
	if err != nil {
		return nil, err
	}

	msg := ""
	for _, e := range events.Items {
		if e.InvolvedObject.Name == svcName {
			msg = e.Message
			break
		}
	}
	if msg == "" {
		return nil, errors.New("service doesn't be assigned node")
	}

	for _, node := range allNodes.Items {
		if strings.Contains(msg, "\""+node.Name+"\"") {
			return &node, nil
		}
	}
	return nil, errors.New("can't find the node that service be assigned")
}
