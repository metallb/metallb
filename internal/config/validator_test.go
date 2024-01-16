// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	v1 "k8s.io/api/core/v1"
)

func TestValidator(t *testing.T) {
	v := validator{DontValidate}

	bgpPeerList := metallbv1beta2.BGPPeerList{
		Items: []metallbv1beta2.BGPPeer{
			{
				Spec: metallbv1beta2.BGPPeerSpec{
					MyASN:      42,
					ASN:        42,
					Address:    "1.2.3.4",
					BFDProfile: "default",
				},
			},
		},
	}
	err := v.Validate(&bgpPeerList)
	if err != nil {
		t.Error("The validator should not fail for non existing bfd profile")
	}
}

func TestResetTransientErrorsFields(t *testing.T) {
	tests := []struct {
		desc             string
		clusterResources ClusterResources
		expected         ClusterResources
	}{
		{
			desc:             "empty resource",
			clusterResources: ClusterResources{},
			expected:         ClusterResources{},
		},
		{
			desc: "BFDProfiles, PasswordSecrets and Community references are reset",
			clusterResources: ClusterResources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						Spec: metallbv1beta2.BGPPeerSpec{
							BFDProfile: "myBFDProfile",
							MyASN:      64512,
							ASN:        64512,
							Address:    "172.30.0.3",
							PasswordSecret: v1.SecretReference{
								Name:      "myName",
								Namespace: "myNamespace",
							},
						},
					},
				},
				Communities: []metallbv1beta1.Community{
					{
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "myCommunityAlias",
									Value: "65000:100",
								},
							},
						},
					},
				},
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							Communities: []string{
								"11111:11aaaa",
								"larg:12345:12345:12345",
								"alias",
								"myCommunityAlias",
							},
						},
					},
				},
			},
			expected: ClusterResources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						Spec: metallbv1beta2.BGPPeerSpec{
							BFDProfile:     "",
							MyASN:          64512,
							ASN:            64512,
							Address:        "172.30.0.3",
							PasswordSecret: v1.SecretReference{},
						},
					},
				},
				Communities: []metallbv1beta1.Community{
					{
						Spec: metallbv1beta1.CommunitySpec{
							Communities: []metallbv1beta1.CommunityAlias{
								{
									Name:  "myCommunityAlias",
									Value: "65000:100",
								},
							},
						},
					},
				},
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							Communities: []string{
								"11111:11aaaa",
								"larg:12345:12345:12345",
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		result := resetTransientErrorsFields(test.clusterResources)
		if !cmp.Equal(result, test.expected) {
			t.Fatalf("test %s failed, unexpected clusterResources (-want +got)\n%s", test.desc, cmp.Diff(result, test.expected))
		}
	}
}
