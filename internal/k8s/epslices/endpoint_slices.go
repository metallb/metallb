// SPDX-License-Identifier:Apache-2.0

package epslices

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
)

type EpsOrSlices struct {
	Type      EpsOrSliceType
	EpVal     *v1.Endpoints
	SlicesVal []discovery.EndpointSlice
}

type EpsOrSliceType int

const (
	Unknown EpsOrSliceType = iota
	Eps
	Slices
)

const SlicesServiceIndexName = "ServiceName"

// IsConditionReady tells if the conditions represent a ready state, interpreting
// nil ready as ready.
func IsConditionReady(conditions discovery.EndpointConditions) bool {
	if conditions.Ready == nil {
		return true
	}
	return *conditions.Ready
}

func ServiceKeyForSlice(endpointSlice *discovery.EndpointSlice) (types.NamespacedName, error) {
	if endpointSlice == nil {
		return types.NamespacedName{}, fmt.Errorf("nil EndpointSlice")
	}
	serviceName, err := serviceNameForSlice(endpointSlice)
	if err != nil {
		return types.NamespacedName{}, err
	}

	return types.NamespacedName{Namespace: endpointSlice.Namespace, Name: serviceName}, nil
}

func SlicesServiceIndex(obj interface{}) ([]string, error) {
	endpointSlice, ok := obj.(*discovery.EndpointSlice)
	if !ok {
		return nil, fmt.Errorf("passed object is not a slice")
	}
	serviceKey, err := ServiceKeyForSlice(endpointSlice)
	if err != nil {
		return nil, err
	}
	return []string{serviceKey.String()}, nil
}

func serviceNameForSlice(endpointSlice *discovery.EndpointSlice) (string, error) {
	serviceName, ok := endpointSlice.Labels[discovery.LabelServiceName]
	if !ok || serviceName == "" {
		return "", fmt.Errorf("endpointSlice missing the %s label", discovery.LabelServiceName)
	}
	return serviceName, nil
}
