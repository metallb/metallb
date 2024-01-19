// SPDX-License-Identifier:Apache-2.0

package epslices

import (
	"fmt"

	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
)

const SlicesServiceIndexName = "ServiceName"

// IsConditionServing tells if the conditions represent a serving state, deferring
// to ready state if serving == nil.
func IsConditionServing(conditions discovery.EndpointConditions) bool {
	if conditions.Serving == nil {
		if conditions.Ready == nil {
			return true
		}
		return *conditions.Ready
	}
	return *conditions.Serving
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
