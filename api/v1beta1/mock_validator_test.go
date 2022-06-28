// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockValidator struct {
	pools          *AddressPoolList
	ipAddressPools *IPAddressPoolList
	bgpAdvs        *BGPAdvertisementList
	l2Advs         *L2AdvertisementList
	communities    *CommunityList
	forceError     bool
}

func (m *mockValidator) Validate(objects ...client.ObjectList) error {
	for _, obj := range objects { // assuming one object per type
		switch list := obj.(type) {
		case *AddressPoolList:
			m.pools = list
		case *BGPAdvertisementList:
			m.bgpAdvs = list
		case *L2AdvertisementList:
			m.l2Advs = list
		case *IPAddressPoolList:
			m.ipAddressPools = list
		case *CommunityList:
			m.communities = list
		default:
			panic("unexpected type")
		}
	}

	if m.forceError {
		return errors.New("Error!")
	}
	return nil
}
