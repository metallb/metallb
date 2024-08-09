// SPDX-License-Identifier:Apache-2.0

package service

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type Tweak func(svc *corev1.Service)

func TrafficPolicyLocal(svc *corev1.Service) {
	svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
}

func ForceV4(svc *corev1.Service) {
	f := corev1.IPFamilyPolicySingleStack
	svc.Spec.IPFamilyPolicy = &f
	svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
}

func ForceV6(svc *corev1.Service) {
	f := corev1.IPFamilyPolicySingleStack
	svc.Spec.IPFamilyPolicy = &f
	svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv6Protocol}
}

func DualStack(svc *corev1.Service) {
	f := corev1.IPFamilyPolicyRequireDualStack
	svc.Spec.IPFamilyPolicy = &f
	svc.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
}

func TrafficPolicyCluster(svc *corev1.Service) {
	svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
}

func WithSpecificIPs(svc *corev1.Service, ips ...string) {
	if len(ips) == 0 {
		return
	}
	svc.Annotations = map[string]string{
		"metallb.io/loadBalancerIPs": strings.Join(ips, ","),
	}
}

func WithSpecificPool(poolName string) func(*corev1.Service) {
	return func(svc *corev1.Service) {
		svc.Annotations = map[string]string{
			"metallb.io/address-pool": poolName,
		}
	}
}

func WithLoadbalancerClass(loadBalancerClass string) func(*corev1.Service) {
	return func(svc *corev1.Service) {
		svc.Spec.LoadBalancerClass = ptr.To[string](loadBalancerClass)
	}
}
