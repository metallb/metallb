// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"fmt"
	"net"
	"strconv"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/wget"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

func nodeForService(svc *corev1.Service, nodes []corev1.Node) (string, error) {
	ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])

	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
	hostport := net.JoinHostPort(ingressIP, port)
	address := fmt.Sprintf("http://%s/", hostport)
	err := mac.RequestAddressResolution(ingressIP, executor.Host)
	if err != nil {
		return "", err
	}
	err = wget.Do(address, executor.Host)
	framework.ExpectNoError(err)
	advNode, err := advertisingNodeFromMAC(nodes, ingressIP, executor.Host)
	if err != nil {
		return "", err
	}

	return advNode.Name, nil
}
