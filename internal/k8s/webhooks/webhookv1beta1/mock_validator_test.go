// SPDX-License-Identifier:Apache-2.0

package webhookv1beta1

import (
	"errors"

	"go.universe.tf/metallb/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockValidator struct {
	ipAddressPools *v1beta1.IPAddressPoolList
	bgpAdvs        *v1beta1.BGPAdvertisementList
	l2Advs         *v1beta1.L2AdvertisementList
	communities    *v1beta1.CommunityList
	nodes          *v1.NodeList
	forceError     bool
}

func (m *mockValidator) Validate(objects ...client.ObjectList) error {
	for _, obj := range objects { // assuming one object per type
		switch list := obj.(type) {
		case *v1beta1.BGPAdvertisementList:
			m.bgpAdvs = list
		case *v1beta1.L2AdvertisementList:
			m.l2Advs = list
		case *v1beta1.IPAddressPoolList:
			m.ipAddressPools = list
		case *v1beta1.CommunityList:
			m.communities = list
		case *v1.NodeList:
			m.nodes = list
		default:
			panic("unexpected type")
		}
	}

	if m.forceError {
		return errors.New("error!")
	}
	return nil
}
