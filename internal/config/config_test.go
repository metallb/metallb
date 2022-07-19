// SPDX-License-Identifier:Apache-2.0

package config

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func selector(s string) labels.Selector {
	ret, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return ret
}

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestParse(t *testing.T) {
	testAdvName := "testAdv"
	testPoolName := "testPool"
	testPool := v1beta1.IPAddressPool{
		ObjectMeta: v1.ObjectMeta{Name: testPoolName},
		Spec: v1beta1.IPAddressPoolSpec{
			Addresses: []string{
				"10.20.0.0/16",
			},
		},
	}
	tests := []struct {
		desc string
		crs  ClusterResources
		want *Config
	}{
		{
			desc: "empty config",
			crs:  ClusterResources{},
			want: &Config{
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "config using all features",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        42,
							ASN:          142,
							Address:      "1.2.3.4",
							Port:         1179,
							HoldTime:     v1.Duration{Duration: 180 * time.Second},
							RouterID:     "10.20.30.40",
							SrcAddress:   "10.20.30.40",
							EBGPMultiHop: true,
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "peer2",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        100,
							ASN:          200,
							Address:      "2.3.4.5",
							EBGPMultiHop: false,
							NodeSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []v1.LabelSelectorRequirement{
										{
											Key:      "bar",
											Operator: "In",
											Values:   []string{"quux"},
										},
									},
								},
							},
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool3",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"40.0.0.0/25",
								"40.0.0.150-40.0.0.200",
								"40.0.0.210 - 40.0.0.240",
								"40.0.0.250 - 40.0.0.250",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool4",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"2001:db8::/64",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							Communities:       []string{"bar"},
							IPAddressPools:    []string{"pool1"},
							Peers:             []string{"peer1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength:   pointer.Int32Ptr(24),
							AggregationLengthV6: pointer.Int32Ptr(64),
							IPAddressPools:      []string{"pool1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool2"},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPools: []string{"pool3"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv2",
						},
					},
				},
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "64512:1234",
								},
							},
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						Name:          "peer1",
						MyASN:         42,
						ASN:           142,
						Addr:          net.ParseIP("1.2.3.4"),
						SrcAddr:       net.ParseIP("10.20.30.40"),
						Port:          1179,
						HoldTime:      180 * time.Second,
						KeepaliveTime: 60 * time.Second,
						RouterID:      net.ParseIP("10.20.30.40"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  true,
					},
					{
						Name:          "peer2",
						MyASN:         100,
						ASN:           200,
						Addr:          net.ParseIP("2.3.4.5"),
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{selector("bar in (quux),foo=bar")},
						EBGPMultiHop:  false,
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						CIDR:          []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs: true,
						AutoAssign:    false,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0xfc0004d2: true,
								},
								Nodes: map[string]bool{},
								Peers: []string{"peer1"},
							},
							{
								AggregationLength:   24,
								AggregationLengthV6: 64,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{},
						}},
					},
					"pool2": {
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{},
						}},
					},
					"pool3": {
						CIDR: []*net.IPNet{
							ipnet("40.0.0.0/25"),
							ipnet("40.0.0.150/31"),
							ipnet("40.0.0.152/29"),
							ipnet("40.0.0.160/27"),
							ipnet("40.0.0.192/29"),
							ipnet("40.0.0.200/32"),
							ipnet("40.0.0.210/31"),
							ipnet("40.0.0.212/30"),
							ipnet("40.0.0.216/29"),
							ipnet("40.0.0.224/28"),
							ipnet("40.0.0.240/32"),
							ipnet("40.0.0.250/32"),
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{},
						}},
						AutoAssign: true,
					},
					"pool4": {
						CIDR: []*net.IPNet{ipnet("2001:db8::/64")},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{},
						}},
						AutoAssign: true,
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "peer-only",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "invalid peer-address",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.400",
						},
					},
				},
			},
		},

		{
			desc: "invalid my-asn",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
		},
		{
			desc: "invalid peer-asn",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							Address: "1.2.3.4",
						},
					},
				},
			},
		},
		{
			desc: "invalid ebgp-multihop",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        42,
							ASN:          42,
							Address:      "1.2.3.4",
							EBGPMultiHop: true,
						},
					},
				},
			},
		},
		{
			desc: "invalid hold time (too short)",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:    42,
							ASN:      42,
							Address:  "1.2.3.4",
							HoldTime: v1.Duration{Duration: time.Second},
						},
					},
				},
			},
		},
		{
			desc: "invalid RouterID",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:    42,
							ASN:      42,
							Address:  "1.2.3.4",
							RouterID: "oh god how do I BGP",
						},
					},
				},
			},
		},
		{
			desc: "empty node selector (select everything)",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "invalid expression node selector (missing key)",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							NodeSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []v1.LabelSelectorRequirement{
										{
											Operator: "In",
											Values:   []string{"quux"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "invalid expression node selector (missing operator)",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							NodeSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []v1.LabelSelectorRequirement{
										{
											Key:    "bar",
											Values: []string{"quux"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "invalid expression node selector (invalid operator)",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							NodeSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []v1.LabelSelectorRequirement{
										{
											Key:      "bar",
											Operator: "surrounds",
											Values:   []string{"quux"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate values node selector match expression",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "peer",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        100,
							ASN:          200,
							Address:      "2.3.4.5",
							EBGPMultiHop: false,
							NodeSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []v1.LabelSelectorRequirement{
										{
											Key:      "bar",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"quux", "quux"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate peers",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
		},
		{
			desc: "no pool name",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{},
				},
			},
		},
		{
			desc: "address pool with no address",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
				},
			},
		},
		{
			desc: "address pool with no protocol",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
					},
				},
			},
		},
		{
			desc: "invalid pool CIDR",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"100.200.300.400/24",
							},
						},
					},
				},
			},
		},
		{
			desc: "invalid pool CIDR prefix length",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/33",
							},
						},
					},
				},
			},
		},
		{
			desc: "invalid pool CIDR, first address of the range is after the second",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.10-1.2.3.1",
							},
						},
					},
				},
			},
		},
		{
			desc: "simple advertisement",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "advertisement with default BGP settings",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "bad aggregation length (too long)",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.10-1.2.3.1",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(34),
						},
					},
				},
			},
		},
		{
			desc: "bad aggregation length (incompatible with CIDR)",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
								"1.2.3.0/28",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(26),
						},
					},
				},
			},
		},
		{
			desc: "aggregation length by range",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"3.3.3.2-3.3.3.254",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(26),
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR: []*net.IPNet{
							ipnet("3.3.3.2/31"),
							ipnet("3.3.3.4/30"),
							ipnet("3.3.3.8/29"),
							ipnet("3.3.3.16/28"),
							ipnet("3.3.3.32/27"),
							ipnet("3.3.3.64/26"),
							ipnet("3.3.3.128/26"),
							ipnet("3.3.3.192/27"),
							ipnet("3.3.3.224/28"),
							ipnet("3.3.3.240/29"),
							ipnet("3.3.3.248/30"),
							ipnet("3.3.3.252/31"),
							ipnet("3.3.3.254/32"),
						},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   26,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "aggregation length by range, too wide",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"3.3.3.2-3.3.3.254",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(24),
						},
					},
				},
			},
		},
		{
			desc: "duplicate ip address pools - in L2 adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPools: []string{testPoolName, testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "duplicate ip address pools - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{testPoolName, testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "duplicate peers - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{testPoolName},
							Peers:          []string{"peer1", "peer1"},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (wrong format) - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities:    []string{"1234"},
							IPAddressPools: []string{testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (asn part doesn't fit) - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities:    []string{"99999999:1"},
							IPAddressPools: []string{testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (community# part doesn't fit) - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities:    []string{"1:99999999"},
							IPAddressPools: []string{testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "bad community ref (unknown ref) - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities:    []string{"community"},
							IPAddressPools: []string{testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "duplicate community literal - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{testPool},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{Name: testAdvName},
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities:    []string{"1234:5678", "1234:5678"},
							IPAddressPools: []string{testPoolName},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (wrong format) - in the community CR",
			crs: ClusterResources{
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "1234",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (asn part doesn't fit) - in the community CR",
			crs: ClusterResources{
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "99999999:1",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (community# part doesn't fit) - in the community CR",
			crs: ClusterResources{
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "1:99999999",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate communities definition (in 2 different crs)",
			crs: ClusterResources{
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community1",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "1234:5678",
								},
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community2",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "1234:5678",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate communities definition (in the same cr)",
			crs: ClusterResources{
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "1234:5678",
								},
								{
									Name:  "bar",
									Value: "1234:5678",
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate pool definition",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
				},
			},
		},
		{
			desc: "duplicate CIDRs",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
				},
			},
		},
		{
			desc: "overlapping CIDRs",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.0.0.0/16",
							},
						},
					},
				},
			},
		},
		{
			desc: "Session with default BFD Profile",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							ASN:        42,
							Address:    "1.2.3.4",
							BFDProfile: "default",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "default",
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "default",
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{
					"default": {
						Name: "default",
					},
				},
			},
		},
		{
			desc: "BGP Peer with both password and secret ref set",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:    42,
							ASN:      42,
							Address:  "1.2.3.4",
							Password: "nopass",
							PasswordSecret: corev1.SecretReference{Name: "nosecret",
								Namespace: "metallb-system"},
						},
					},
				},
			},
		},
		{
			desc: "BGP Peer with invalid secret type",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							PasswordSecret: corev1.SecretReference{Name: "bgpsecret",
								Namespace: "metallb-system"},
						},
					},
				},
				PasswordSecrets: map[string]corev1.Secret{
					"bgpsecret": {Type: corev1.SecretTypeOpaque, ObjectMeta: v1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"}},
				},
			},
		},
		{
			desc: "BGP Peer without password set in the secret",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							PasswordSecret: corev1.SecretReference{Name: "bgpsecret",
								Namespace: "metallb-system"},
						},
					},
				},
				PasswordSecrets: map[string]corev1.Secret{
					"bgpsecret": {Type: corev1.SecretTypeBasicAuth, ObjectMeta: v1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"}},
				},
			},
		},
		{
			desc: "BGP Peer with a valid secret",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Port:    179,
							Address: "1.2.3.4",
							PasswordSecret: corev1.SecretReference{Name: "bgpsecret",
								Namespace: "metallb-system"},
						},
					},
				},
				PasswordSecrets: map[string]corev1.Secret{
					"bgpsecret": {Type: corev1.SecretTypeBasicAuth, ObjectMeta: v1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"},
						Data: map[string][]byte{"password": []byte([]byte("nopass"))}},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						Port:          179,
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "",
						Password:      "nopass",
					},
				},
				Pools:       map[string]*Pool{},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "BGP Peer with unavailable secret ref",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
							PasswordSecret: corev1.SecretReference{Name: "nosecret",
								Namespace: "metallb-system"},
						},
					},
				},
			},
		},
		{
			desc: "Peer with non existing BFD Profile",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							ASN:        42,
							Address:    "1.2.3.4",
							BFDProfile: "default",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
		},
		{
			desc: "Multiple BFD Profiles with the same name",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "default",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
					},
				},
			},
		},
		{
			desc: "Session with nondefault BFD Profile",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "nondefault",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval:  pointer.Uint32Ptr(50),
							TransmitInterval: pointer.Uint32Ptr(51),
							DetectMultiplier: pointer.Uint32Ptr(52),
							EchoInterval:     pointer.Uint32Ptr(54),
							EchoMode:         pointer.BoolPtr(true),
							PassiveMode:      pointer.BoolPtr(true),
							MinimumTTL:       pointer.Uint32Ptr(55),
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{
					"nondefault": {
						Name:             "nondefault",
						ReceiveInterval:  pointer.Uint32Ptr(50),
						DetectMultiplier: pointer.Uint32Ptr(52),
						TransmitInterval: pointer.Uint32Ptr(51),
						EchoInterval:     pointer.Uint32Ptr(54),
						MinimumTTL:       pointer.Uint32Ptr(55),
						EchoMode:         true,
						PassiveMode:      true,
					},
				},
			},
		},
		{
			desc: "BFD Profile with too low receive interval",
			crs: ClusterResources{

				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "default",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval: pointer.Uint32Ptr(2),
						},
					},
				},
			},
		},
		{
			desc: "BFD Profile with too high receive interval",
			crs: ClusterResources{

				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "default",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval: pointer.Uint32Ptr(90000),
						},
					},
				},
			},
		},
		{
			desc: "config mixing legacy pools with IP pools",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        42,
							ASN:          142,
							Address:      "1.2.3.4",
							Port:         1179,
							HoldTime:     v1.Duration{Duration: 180 * time.Second},
							RouterID:     "10.20.30.40",
							SrcAddress:   "10.20.30.40",
							EBGPMultiHop: true,
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
						},
					},
				},
				LegacyAddressPools: []v1beta1.AddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacyl2pool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.21.0.0/16",
								"10.51.0.0/24",
							},
							Protocol:   string(Layer2),
							AutoAssign: pointer.BoolPtr(false),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacybgppool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.40.0.0/16",
								"10.60.0.0/24",
							},
							Protocol:   string(BGP),
							AutoAssign: pointer.BoolPtr(false),
							BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
								{
									AggregationLength: pointer.Int32Ptr(32),
									LocalPref:         uint32(100),
									Communities:       []string{"1234:2345"},
								},
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							Communities:       []string{"1234:2345"},
							IPAddressPools:    []string{"pool1"},
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						MyASN:         42,
						ASN:           142,
						Addr:          net.ParseIP("1.2.3.4"),
						SrcAddr:       net.ParseIP("10.20.30.40"),
						Port:          1179,
						HoldTime:      180 * time.Second,
						KeepaliveTime: 60 * time.Second,
						RouterID:      net.ParseIP("10.20.30.40"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  true,
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						CIDR:          []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs: true,
						AutoAssign:    false,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0x04D20929: true,
								},
								Nodes: map[string]bool{},
							},
						},
					},
					"legacybgppool1": {
						CIDR: []*net.IPNet{ipnet("10.40.0.0/16"), ipnet("10.60.0.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0x04D20929: true,
								},
								Nodes: map[string]bool{},
							},
						},
					},
					"legacyl2pool1": {
						CIDR: []*net.IPNet{ipnet("10.21.0.0/16"), ipnet("10.51.0.0/24")},
						L2Advertisements: []*L2Advertisement{{
							Nodes: map[string]bool{},
						}},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "config legacy pool with bgp communities crd",
			crs: ClusterResources{
				LegacyAddressPools: []v1beta1.AddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacybgppool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.40.0.0/16",
								"10.60.0.0/24",
							},
							Protocol:   string(BGP),
							AutoAssign: pointer.BoolPtr(false),
							BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
								{
									AggregationLength: pointer.Int32Ptr(32),
									LocalPref:         uint32(100),
									Communities:       []string{"bar"},
								},
							},
						},
					},
				},
				Communities: []v1beta1.Community{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "64512:1234",
								},
							},
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"legacybgppool1": {
						CIDR: []*net.IPNet{ipnet("10.40.0.0/16"), ipnet("10.60.0.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0xfc0004d2: true,
								},
								Nodes: map[string]bool{},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "config mixing legacy pools with IP pools with overlapping ips",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
						},
					},
				},
				LegacyAddressPools: []v1beta1.AddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacyl2pool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.51.0.0/24",
							},
							Protocol:   string(Layer2),
							AutoAssign: pointer.BoolPtr(false),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacybgppool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.40.0.0/16",
								"10.60.0.0/24",
							},
							Protocol: string(BGP),
						},
					},
				},
			},
		},
		{
			desc: "use ip pool selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool1",
							Labels: map[string]string{"test": "pool1"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool2",
							Labels: map[string]string{"test": "pool2"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/16",
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(200),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "pool2",
									},
								},
							},
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes: map[string]bool{},
						}},
					},
					"pool2": {
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           200,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: nil,
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "use duplicate match labels in ip pool selectors - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool1",
							Labels: map[string]string{"test": "pool1"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "use duplicate match expression in ip pool selector - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool1",
							Labels: map[string]string{"test": "pool1"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "test",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"pool1", "pool1"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "use duplicate ip pool selectors - in L2 adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool1",
							Labels: map[string]string{"test": "pool1"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
								{
									MatchLabels: map[string]string{
										"test": "pool1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "use non existent label for ip pool selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool1",
							Labels: map[string]string{"test": "pool1"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "pool2",
							Labels: map[string]string{"test": "pool2"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/16",
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "do-not-select-pool",
									},
								},
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "do-not-select-pool",
									},
								},
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(200),
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"test": "do-not-select-pool",
									},
								},
							},
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						CIDR:              []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign:        true,
						BGPAdvertisements: nil,
						L2Advertisements:  nil,
					},
					"pool2": {
						CIDR:              []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign:        true,
						BGPAdvertisements: nil,
						L2Advertisements:  nil,
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "use node selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool1"},
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"second": "true",
									},
								},
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool2"},
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"first": "true",
									},
								},
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"first": "true",
									},
								},
							},
							IPAddressPools: []string{"pool1"},
						},
					},
				},
				LegacyAddressPools: []v1beta1.AddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacyl2pool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.21.0.0/16",
								"10.51.0.0/24",
							},
							Protocol:   string(Layer2),
							AutoAssign: pointer.BoolPtr(false),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacybgppool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.40.0.0/16",
								"10.60.0.0/24",
							},
							Protocol:   string(BGP),
							AutoAssign: pointer.BoolPtr(false),
						},
					},
				},
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first",
							Labels: map[string]string{
								"first": "true",
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second",
							Labels: map[string]string{
								"second": "true",
							},
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"legacybgppool1": {
						CIDR: []*net.IPNet{ipnet("10.40.0.0/16"), ipnet("10.60.0.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes: map[string]bool{
									"first":  true,
									"second": true,
								},
							},
						},
					},

					"legacyl2pool1": {
						CIDR: []*net.IPNet{ipnet("10.21.0.0/16"), ipnet("10.51.0.0/24")},
						L2Advertisements: []*L2Advertisement{{
							Nodes: map[string]bool{
								"first":  true,
								"second": true,
							},
						}},
					},

					"pool1": {
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{"second": true},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{
								"first": true,
							},
						}},
					},
					"pool2": {
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{"first": true},
							},
						},
						L2Advertisements: nil,
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "use duplicate match labels in node selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool1"},
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"second": "true",
									},
								},
								{
									MatchLabels: map[string]string{
										"second": "true",
									},
								},
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"first": "true",
									},
								},
								{
									MatchLabels: map[string]string{
										"first": "true",
									},
								},
							},
							IPAddressPools: []string{"pool1"},
						},
					},
				},
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first",
							Labels: map[string]string{
								"first": "true",
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second",
							Labels: map[string]string{
								"second": "true",
							},
						},
					},
				},
			},
		},
		{
			desc: "use duplicate match expression values in node selector",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool1"},
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "kubernetes.io/hostname",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"first", "second"},
										},
										{
											Key:      "kubernetes.io/hostname",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"second", "first"},
										},
									},
								},
							},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "kubernetes.io/hostname",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"first"},
										},
									},
								},
							},
							IPAddressPools: []string{"pool1"},
						},
					},
				},
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first",
							Labels: map[string]string{
								"first": "true",
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second",
							Labels: map[string]string{
								"second": "true",
							},
						},
					},
				},
			},
		},
		{
			desc: "no nodes means all nodes",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
					},
				},
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first",
							Labels: map[string]string{
								"first": "true",
							},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second",
							Labels: map[string]string{
								"second": "true",
							},
						},
					},
				},
			},
			want: &Config{
				Pools: map[string]*Pool{
					"pool1": {
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
								Nodes: map[string]bool{
									"first":  true,
									"second": true,
								},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{
							Nodes: map[string]bool{
								"first":  true,
								"second": true,
							},
						}},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "advertisement with peer selector",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: pointer.Int32Ptr(32),
							LocalPref:         uint32(100),
							IPAddressPools:    []string{"pool1"},
							Peers:             []string{"peer1"},
						},
					},
				},
			},
			want: &Config{
				Peers: []*Peer{
					{
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						HoldTime:      90 * time.Second,
						KeepaliveTime: 30 * time.Second,
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools: map[string]*Pool{
					"pool1": {
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities:         map[uint32]bool{},
								Nodes:               map[string]bool{},
								Peers:               []string{"peer1"},
							},
						},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "config legacy pool with invalid community",
			crs: ClusterResources{
				LegacyAddressPools: []v1beta1.AddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "legacybgppool1",
						},
						Spec: v1beta1.AddressPoolSpec{
							Addresses: []string{
								"10.40.0.0/16",
								"10.60.0.0/24",
							},
							Protocol:   string(BGP),
							AutoAssign: pointer.BoolPtr(false),
							BGPAdvertisements: []v1beta1.LegacyBgpAdvertisement{
								{
									AggregationLength: pointer.Int32Ptr(32),
									LocalPref:         uint32(100),
									Communities:       []string{"1234"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := For(test.crs, DontValidate)
			if err != nil && test.want != nil {
				t.Errorf("%q: parse failed: %s", test.desc, err)
				return
			}
			if test.want == nil && err == nil {
				t.Errorf("%q: parse unexpectedly succeeded", test.desc)
				return
			}
			selectorComparer := cmp.Comparer(func(x, y labels.Selector) bool {
				if x == nil {
					return y == nil
				}
				if y == nil {
					return x == nil
				}
				// Nothing() and Everything() have the same string
				// representation, stupidly. So, compare explicitly for
				// Nothing.
				if x == labels.Nothing() {
					return y == labels.Nothing()
				}
				if y == labels.Nothing() {
					return x == labels.Nothing()
				}
				return x.String() == y.String()
			})
			// We don't care about comparing cidrPerAddress as it's calculated
			cidrPerAddressComparer := cmp.Comparer(func(x, y map[string][]*net.IPNet) bool {
				return true
			})

			if diff := cmp.Diff(test.want, got, selectorComparer, cidrPerAddressComparer, cmp.AllowUnexported(Pool{})); diff != "" {
				t.Errorf("%q: parse returned wrong result (-want, +got)\n%s", test.desc, diff)
			}
		})
	}
}
