// SPDX-License-Identifier:Apache-2.0

package service

import (
	"fmt"
	"net"
	"strconv"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/wget"
	corev1 "k8s.io/api/core/v1"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

func ValidateL2(svc *corev1.Service) error {
	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
	ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
	hostport := net.JoinHostPort(ingressIP, port)
	address := fmt.Sprintf("http://%s/", hostport)

	err := wget.Do(address, executor.Host)
	if err != nil {
		return err
	}
	return nil
}
