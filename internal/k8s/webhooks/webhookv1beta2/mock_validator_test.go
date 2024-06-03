// SPDX-License-Identifier:Apache-2.0

package webhookv1beta2

import (
	"errors"

	"go.universe.tf/metallb/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockValidator struct {
	bgpPeers   *v1beta2.BGPPeerList
	forceError bool
}

func (m *mockValidator) Validate(objects ...client.ObjectList) error {
	for _, obj := range objects { // assuming one object per type
		switch list := obj.(type) {
		case *v1beta2.BGPPeerList:
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
