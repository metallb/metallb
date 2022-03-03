// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"
	"time"

	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		desc     string
		config   ClusterResources
		mustFail bool
	}{
		{
			desc: "peer with bfd profile",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:    "1.2.3.4",
							BFDProfile: "foo",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "bfd profile set",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{Name: "foo"},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "v6 address",
			config: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						Spec: v1beta1.IPPoolSpec{
							Addresses: []string{"2001:db8::/64"},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "keepalive time",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							KeepaliveTime: v1.Duration{Duration: time.Second},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "should pass",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := DiscardFRROnly(test.config)
			if test.mustFail && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.mustFail && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
		})
	}
}

func TestValidateFRR(t *testing.T) {
	tests := []struct {
		desc     string
		config   ClusterResources
		mustFail bool
	}{
		{
			desc: "peer with routerid",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.4",
							RouterID: "1.2.3.4",
						},
					},
				},
			},
		},
		{
			desc: "routerid set, one different",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.4",
							RouterID: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.5",
							RouterID: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.6",
							RouterID: "1.2.3.5",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "routerid set, one not set",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.4",
							RouterID: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.5",
							RouterID: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.6",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "bfd profile set",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address:  "1.2.3.4",
							RouterID: "1.2.3.4",
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
					},
				},
			},
		},
		{
			desc: "myAsn set, all equals",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
							MyASN:   123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.5",
							MyASN:   123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.6",
							MyASN:   123,
						},
					},
				},
			},
		},
		{
			desc: "myAsn set, one different",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
							MyASN:   123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.5",
							MyASN:   123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.6",
							MyASN:   124,
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "duplicate bgp address",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
						},
					},
				},
			},
			mustFail: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := DiscardNativeOnly(test.config)
			if test.mustFail && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.mustFail && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
		})
	}
}
