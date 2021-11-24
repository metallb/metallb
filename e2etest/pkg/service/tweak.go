// SPDX-License-Identifier:Apache-2.0

package service

import corev1 "k8s.io/api/core/v1"

type Tweak func(svc *corev1.Service)

func TrafficPolicyLocal(svc *corev1.Service) {
	svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
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
