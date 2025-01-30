// SPDX-License-Identifier:Apache-2.0

package k8salloc

import (
	"go.universe.tf/metallb/internal/allocator"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Ports turns a service definition into a set of allocator ports.
func Ports(svc *v1.Service) []allocator.Port {
	var ret []allocator.Port
	for _, port := range svc.Spec.Ports {
		ret = append(ret, allocator.Port{
			Proto: string(port.Protocol),
			Port:  int(port.Port),
		})
	}
	return ret
}

// BackendKey extracts the backend key for a service.
func BackendKey(svc *v1.Service) string {
	/*
	Monkeypatch: remove this check to support sharing an ip between externalTrafficPolicy=Local services
 	Only works for single-node k8s clusters
	if svc.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		return labels.Set(svc.Spec.Selector).String()
	}
	*/
	// Cluster traffic policy can share services regardless of backends.
	return ""
}
