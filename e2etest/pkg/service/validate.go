// SPDX-License-Identifier:Apache-2.0

package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/wget"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
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

func ValidateDesiredLB(svc *corev1.Service) {
	desiredLbIPs := svc.Annotations["metallb.universe.tf/loadBalancerIPs"]
	if desiredLbIPs == "" {
		return
	}
	framework.ExpectEqual(desiredLbIPs, strings.Join(getIngressIPs(svc.Status.LoadBalancer.Ingress), ","))
}

// ValidateAssignedWith validates that the service is assigned with the given ip.
func ValidateAssignedWith(svc *corev1.Service, ip string) error {
	if ip == "" {
		return nil
	}

	ingressIPs := getIngressIPs(svc.Status.LoadBalancer.Ingress)
	for _, ingressIP := range ingressIPs {
		if ingressIP == ip {
			return nil
		}
	}

	return fmt.Errorf("validation failed: ip %s is not assigned to service %s", ip, svc.Name)
}

func getIngressIPs(ingresses []corev1.LoadBalancerIngress) []string {
	var ips []string
	for _, ingress := range ingresses {
		ips = append(ips, ingress.IP)
	}
	return ips
}
