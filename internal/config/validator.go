// SPDX-License-Identifier:Apache-2.0

package config

import (
	"errors"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	apivalidate "go.universe.tf/metallb/api/validate"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TransientError is an error that happens due to interdependencies
// between crds, such as referencing non-existing bfd profile.
type TransientError struct {
	Message string
}

func (e TransientError) Error() string { return e.Message }

// validator is an implementation of the resources validator
// that tries to parse the configuration and fails if the returned
// error is non transitient.
type validator struct {
	validate Validate
}

func (v *validator) Validate(resources ...client.ObjectList) error {
	clusterResources := ClusterResources{
		Pools:              make([]metallbv1beta1.IPAddressPool, 0),
		Peers:              make([]metallbv1beta2.BGPPeer, 0),
		BFDProfiles:        make([]metallbv1beta1.BFDProfile, 0),
		BGPAdvs:            make([]metallbv1beta1.BGPAdvertisement, 0),
		L2Advs:             make([]metallbv1beta1.L2Advertisement, 0),
		LegacyAddressPools: make([]metallbv1beta1.AddressPool, 0),
	}
	for _, list := range resources {
		switch list := list.(type) {
		case *metallbv1beta1.IPAddressPoolList:
			clusterResources.Pools = append(clusterResources.Pools, list.Items...)
		case *metallbv1beta2.BGPPeerList:
			clusterResources.Peers = append(clusterResources.Peers, list.Items...)
		case *metallbv1beta1.BFDProfileList:
			clusterResources.BFDProfiles = append(clusterResources.BFDProfiles, list.Items...)
		case *metallbv1beta1.BGPAdvertisementList:
			clusterResources.BGPAdvs = append(clusterResources.BGPAdvs, list.Items...)
		case *metallbv1beta1.L2AdvertisementList:
			clusterResources.L2Advs = append(clusterResources.L2Advs, list.Items...)
		case *metallbv1beta1.AddressPoolList:
			clusterResources.LegacyAddressPools = append(clusterResources.LegacyAddressPools, list.Items...)
		}
	}
	_, err := For(clusterResources, v.validate)
	if errors.As(err, &TransientError{}) { // we do not want to make assumption on ordering in webhooks.
		return nil
	}
	return err
}

func NewValidator(validate Validate) apivalidate.ClusterObjects {
	return &validator{validate: validate}
}
