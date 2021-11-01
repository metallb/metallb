// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"

	"go.universe.tf/metallb/internal/k8s/epslices"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func epsOrSlicesForServices(ctx context.Context, cl client.Reader, serviceName types.NamespacedName, endpoints NeedEndPoints) (epslices.EpsOrSlices, error) {
	res := epslices.EpsOrSlices{}
	switch endpoints {
	case EndpointSlices:
		var slices discovery.EndpointSliceList
		if err := cl.List(ctx, &slices, client.MatchingFields{epslices.SlicesServiceIndexName: serviceName.String()}); err != nil {
			return res, err
		}
		res = epslices.EpsOrSlices{SlicesVal: slices.Items, Type: epslices.Slices}

	case Endpoints:
		var endpoints v1.Endpoints
		if err := cl.Get(ctx, serviceName, &endpoints); err != nil && !apierrors.IsNotFound(err) {
			return res, err
		}
		res = epslices.EpsOrSlices{EpVal: &endpoints, Type: epslices.Eps}
	}
	return res, nil
}
