// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"

	"go.universe.tf/metallb/internal/k8s/epslices"
	discovery "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func epSlicesForService(ctx context.Context, cl client.Reader, serviceName types.NamespacedName) ([]discovery.EndpointSlice, error) {
	var slices discovery.EndpointSliceList
	if err := cl.List(ctx, &slices, client.MatchingFields{epslices.SlicesServiceIndexName: serviceName.String()}); err != nil {
		return nil, err
	}
	return slices.Items, nil
}
