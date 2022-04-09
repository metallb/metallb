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
			// TODO CRD Add communities
			//			bgp-communities:
			//  bar: 64512:1234
			desc: "config using all features",
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
					{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
						Spec: v1beta1.IPPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool3",
						},
						Spec: v1beta1.IPPoolSpec{
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
						Spec: v1beta1.IPPoolSpec{
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
							Communities:       []string{ /* TODO CRD Add communities"bar", */ "1234:2345"},
							IPPools:           []string{"pool1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength:   pointer.Int32Ptr(24),
							AggregationLengthV6: pointer.Int32Ptr(64),
							IPPools:             []string{"pool1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "adv3",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPPools: []string{"pool2"},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPPools: []string{"pool3"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "l2adv2",
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
					{
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
									//0xfc0004d2: true,
									0x04D20929: true,
								},
							},
							{
								AggregationLength:   24,
								AggregationLengthV6: 64,
								Communities:         map[uint32]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{}},
					},
					"pool2": {
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[uint32]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{}},
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
						L2Advertisements: []*L2Advertisement{&L2Advertisement{}},
						AutoAssign:       true,
					},
					"pool4": {
						CIDR:             []*net.IPNet{ipnet("2001:db8::/64")},
						L2Advertisements: []*L2Advertisement{&L2Advertisement{}},
						AutoAssign:       true,
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
				Pools: []v1beta1.IPPool{
					{},
				},
			},
		},
		{
			desc: "address pool with no address",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPPoolSpec{},
					},
				},
			},
		},
		{
			desc: "address pool with no protocol",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
					},
				},
			},
		},
		{
			desc: "invalid pool CIDR",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
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
			desc: "bad community literal (wrong format)",
			crs: ClusterResources{
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities: []string{
								"1234",
							},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (asn part doesn't fit)",
			crs: ClusterResources{
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities: []string{
								"99999999:1",
							},
						},
					},
				},
			},
		},
		{
			desc: "bad community literal (community# part doesn't fit)",
			crs: ClusterResources{
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities: []string{
								"1:99999999",
							},
						},
					},
				},
			},
		},
		{
			desc: "duplicate pool definition",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPPoolSpec{},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec:       v1beta1.IPPoolSpec{},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPPoolSpec{},
					},
				},
			},
		},
		{
			desc: "duplicate CIDRs",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool2"},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
							Protocol:      string(Layer2),
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
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
							Protocol:      string(BGP),
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
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
							IPPools:           []string{"pool1"},
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
							},
						},
					},
					"legacybgppool1": {
						CIDR:          []*net.IPNet{ipnet("10.40.0.0/16"), ipnet("10.60.0.0/24")},
						AvoidBuggyIPs: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: map[uint32]bool{
									0x04D20929: true,
								},
							},
						},
					},
					"legacyl2pool1": {
						CIDR:             []*net.IPNet{ipnet("10.21.0.0/16"), ipnet("10.51.0.0/24")},
						AvoidBuggyIPs:    true,
						L2Advertisements: []*L2Advertisement{{}},
					},
				},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "config mixing legacy pools with IP pools with overlapping ips",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPPoolSpec{
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
							Protocol:      string(Layer2),
							AvoidBuggyIPs: true,
							AutoAssign:    pointer.BoolPtr(false),
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
		/* TODO Add communities CRD
		{
			desc: "Duplicate communities definition",
			crs: ClusterResources{
				Pools: []v1beta1.IPPool{
					{
						ObjectMeta: v1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPPoolSpec{
							Protocol: "bgp",
							Addresses: []string{
								"10.20.0.0/16",
							},
							BGPAdvertisements: []v1beta1.BgpAdvertisement{
								{
									AggregationLength: pointer.Int32Ptr(26),
								},
								{
									AggregationLength: pointer.Int32Ptr(26),
								},
							},
						},
					},
				},
			},
		},*/
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
