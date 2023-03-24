// SPDX-License-Identifier:Apache-2.0

package v1beta2

import (
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockValidator struct {
	bgpPeers   *BGPPeerList
	forceError bool
}

func (m *mockValidator) Validate(objects ...client.ObjectList) error {
	for _, obj := range objects { // assuming one object per type
		switch list := obj.(type) {
		case *BGPPeerList:
			m.bgpPeers = list
		default:
			panic("unexpected type")
		}
	}

	if m.forceError {
		return errors.New("error!")
	}
	return nil
}
