// SPDX-License-Identifier:Apache-2.0

package config

import (
	"strings"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	apivalidate "go.universe.tf/metallb/internal/k8s/webhooks/validate"

	v1 "k8s.io/api/core/v1"
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
		Pools:       make([]metallbv1beta1.IPAddressPool, 0),
		Peers:       make([]metallbv1beta2.BGPPeer, 0),
		BFDProfiles: make([]metallbv1beta1.BFDProfile, 0),
		BGPAdvs:     make([]metallbv1beta1.BGPAdvertisement, 0),
		L2Advs:      make([]metallbv1beta1.L2Advertisement, 0),
		Communities: make([]metallbv1beta1.Community, 0),
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
		case *metallbv1beta1.CommunityList:
			clusterResources.Communities = append(clusterResources.Communities, list.Items...)
		case *v1.NodeList:
			clusterResources.Nodes = append(clusterResources.Nodes, list.Items...)
		}
	}
	clusterResources = resetTransientErrorsFields(clusterResources)
	_, err := For(clusterResources, v.validate)
	return err
}

func NewValidator(validate Validate) apivalidate.ClusterObjects {
	return &validator{validate: validate}
}

// Returns the given ClusterResources without the fields that can cause a TransientError.
// We use this so the webhooks do not make assumptions based on the ordering of objects.
func resetTransientErrorsFields(clusterResources ClusterResources) ClusterResources {
	for i := range clusterResources.Peers {
		clusterResources.Peers[i].Spec.BFDProfile = ""
		clusterResources.Peers[i].Spec.PasswordSecret = v1.SecretReference{}
	}
	for i, bgpAdv := range clusterResources.BGPAdvs {
		var communities []string
		for _, community := range bgpAdv.Spec.Communities {
			// We want to pass only communities that are potentially explicit values.
			if strings.Contains(community, ":") {
				communities = append(communities, community)
			}
		}
		clusterResources.BGPAdvs[i].Spec.Communities = communities
	}
	return clusterResources
}
