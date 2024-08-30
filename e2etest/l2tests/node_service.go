// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"fmt"
	"net"
	"strconv"

	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/executor"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/wget"
	corev1 "k8s.io/api/core/v1"
)

func nodeForService(svc *corev1.Service, nodes []corev1.Node) (*corev1.Node, error) {
	ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])

	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
	hostport := net.JoinHostPort(ingressIP, port)
	address := fmt.Sprintf("http://%s/", hostport)
	err := mac.RequestAddressResolution(ingressIP, executor.Host)
	if err != nil {
		return nil, err
	}
	err = wget.Do(address, executor.Host)
	Expect(err).NotTo(HaveOccurred())
	advNode, err := advertisingNodeFromMAC(nodes, ingressIP, executor.Host)
	if err != nil {
		return nil, err
	}

	return advNode, nil
}
