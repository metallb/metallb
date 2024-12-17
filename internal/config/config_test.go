// SPDX-License-Identifier:Apache-2.0

package config

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/internal/bgp/community"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
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
		ObjectMeta: metav1.ObjectMeta{Name: testPoolName},
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
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},

		{
			desc: "config using all features",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:                 42,
							ASN:                   142,
							Address:               "1.2.3.4",
							Port:                  1179,
							HoldTime:              ptr.To(metav1.Duration{Duration: 180 * time.Second}),
							ConnectTime:           ptr.To(metav1.Duration{Duration: time.Second}),
							RouterID:              "10.20.30.40",
							SrcAddress:            "10.20.30.40",
							EnableGracefulRestart: true,
							EBGPMultiHop:          true,
							VRFName:               "foo",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer2",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:                 100,
							ASN:                   200,
							Address:               "2.3.4.5",
							EnableGracefulRestart: false,
							EBGPMultiHop:          false,
							ConnectTime:           ptr.To(metav1.Duration{Duration: time.Second}),
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []metav1.LabelSelectorRequirement{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    ptr.To(false),
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
							LocalPref:         uint32(100),
							Communities:       []string{"bar"},
							IPAddressPools:    []string{"pool1"},
							Peers:             []string{"peer1"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength:   ptr.To[int32](24),
							AggregationLengthV6: ptr.To[int32](64),
							IPAddressPools:      []string{"pool1"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool2"},
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2adv1",
						},
						Spec: v1beta1.L2AdvertisementSpec{
							IPAddressPools: []string{"pool3"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2adv2",
						},
					},
				},
				Communities: []v1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
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
				Peers: map[string]*Peer{
					"peer1": {
						Name:                  "peer1",
						MyASN:                 42,
						ASN:                   142,
						Addr:                  net.ParseIP("1.2.3.4"),
						SrcAddr:               net.ParseIP("10.20.30.40"),
						Port:                  1179,
						HoldTime:              ptr.To(180 * time.Second),
						KeepaliveTime:         ptr.To(60 * time.Second),
						ConnectTime:           ptr.To(time.Second),
						RouterID:              net.ParseIP("10.20.30.40"),
						NodeSelectors:         []labels.Selector{labels.Everything()},
						EnableGracefulRestart: true,
						EBGPMultiHop:          true,
						VRF:                   "foo",
					},
					"peer2": {
						Name:                  "peer2",
						MyASN:                 100,
						ASN:                   200,
						Addr:                  net.ParseIP("2.3.4.5"),
						ConnectTime:           ptr.To(time.Second),
						NodeSelectors:         []labels.Selector{selector("bar in (quux),foo=bar")},
						EnableGracefulRestart: false,
						EBGPMultiHop:          false,
					},
				},
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:          "pool1",
						CIDR:          []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs: true,
						AutoAssign:    false,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: func() map[community.BGPCommunity]bool {
									c, _ := community.New("64512:1234")
									return map[community.BGPCommunity]bool{
										c: true,
									}
								}(),
								Nodes: map[string]bool{},
								Peers: []string{"peer1"},
							},
							{
								Name:                "adv2",
								AggregationLength:   24,
								AggregationLengthV6: 64,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: true,
						}},
					},
					"pool2": {
						Name:       "pool2",
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv3",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: true,
						}},
					},
					"pool3": {
						Name: "pool3",
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
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: true,
						}},
						AutoAssign: true,
					},
					"pool4": {
						Name: "pool4",
						CIDR: []*net.IPNet{ipnet("2001:db8::/64")},
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: true,
						}},
						AutoAssign: true,
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},

		{
			desc: "ip address pool with namespace selection",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    ptr.To(false),
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 1,
								Namespaces: []string{"test-ns1"}},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 2,
								NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"team": "metallb"}}}},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool3",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"40.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 3,
								NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"team": "red"}}}},
						},
					},
				},
				Namespaces: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "test-ns2",
							Labels: map[string]string{"team": "metallb"},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:               "pool1",
						CIDR:               []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs:      true,
						AutoAssign:         false,
						ServiceAllocations: &ServiceAllocation{Priority: 1, Namespaces: sets.Set[string]{"test-ns1": {}}},
					},
					"pool2": {
						Name:               "pool2",
						CIDR:               []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign:         true,
						ServiceAllocations: &ServiceAllocation{Priority: 2, Namespaces: sets.Set[string]{"test-ns2": {}}},
					},
					"pool3": {
						Name:               "pool3",
						CIDR:               []*net.IPNet{ipnet("40.0.0.0/8")},
						AutoAssign:         true,
						ServiceAllocations: &ServiceAllocation{Priority: 3, Namespaces: sets.Set[string]{}},
					},
				},
					ByNamespace: map[string][]string{"test-ns1": {"pool1"}, "test-ns2": {"pool2"}},
				},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},

		{
			desc: "ip address pool with duplicate namespace selection",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    ptr.To(false),
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 1,
								Namespaces: []string{"test-ns1", "test-ns1"}},
						},
					},
				},
				Namespaces: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns1",
						},
					},
				},
			},
		},

		{
			desc: "ip address pool with duplicate namespace selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    ptr.To(false),
							AllocateTo: &v1beta1.ServiceAllocation{
								Priority: 1,
								NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"foo": "bar"}},
									{MatchLabels: map[string]string{"foo": "bar"}}},
							},
						},
					},
				},
				Namespaces: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "test-ns1",
							Labels: map[string]string{"foo": "bar"},
						},
					},
				},
			},
		},

		{
			desc: "ip address pool with service selection",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 2,
								ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"team": "metallb"}}}},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"40.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 3,
								ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"team": "red"}}}},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:               "pool1",
						CIDR:               []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign:         true,
						ServiceAllocations: &ServiceAllocation{Priority: 2, ServiceSelectors: []labels.Selector{selector("team=metallb")}},
					},
					"pool2": {
						Name:               "pool2",
						CIDR:               []*net.IPNet{ipnet("40.0.0.0/8")},
						AutoAssign:         true,
						ServiceAllocations: &ServiceAllocation{Priority: 3, ServiceSelectors: []labels.Selector{selector("team=red")}},
					},
				},
					ByServiceSelector: []string{"pool1", "pool2"}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},

		{
			desc: "ip address pool with duplicate service selection",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 2,
								ServiceSelectors: []metav1.LabelSelector{{MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "foo",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"bar", "bar"},
									},
								}}}},
						},
					},
				},
			},
		},

		{
			desc: "ip address pool with namespace and service selection",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 2,
								Namespaces:       []string{"test-ns1"},
								ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"testsvc-1": "1"}}}},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool2",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"40.0.0.0/8",
							},
							AllocateTo: &v1beta1.ServiceAllocation{Priority: 3,
								NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"team": "metallb"}}},
								ServiceSelectors:   []metav1.LabelSelector{{MatchLabels: map[string]string{"testsvc-2": "2"}}}},
						},
					},
				},
				Namespaces: []corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ns1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "test-ns2",
							Labels: map[string]string{"team": "metallb"},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/8")},
						AutoAssign: true,
						ServiceAllocations: &ServiceAllocation{Priority: 2, Namespaces: sets.New("test-ns1"),
							ServiceSelectors: []labels.Selector{selector("testsvc-1=1")}},
					},
					"pool2": {
						Name:       "pool2",
						CIDR:       []*net.IPNet{ipnet("40.0.0.0/8")},
						AutoAssign: true,
						ServiceAllocations: &ServiceAllocation{Priority: 3, Namespaces: sets.New("test-ns2"),
							ServiceSelectors: []labels.Selector{selector("testsvc-2=2")}},
					},
				},
					ByNamespace:       map[string][]string{"test-ns1": {"pool1"}, "test-ns2": {"pool2"}},
					ByServiceSelector: []string{"pool1", "pool2"}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},

		{
			desc: "peer-only",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
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
			desc: "invalid keepalivetime larger than holdtime",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:         42,
							ASN:           42,
							Address:       "1.2.3.4",
							HoldTime:      ptr.To(metav1.Duration{Duration: 30 * time.Second}),
							KeepaliveTime: ptr.To(metav1.Duration{Duration: 90 * time.Second}),
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
							HoldTime: ptr.To(metav1.Duration{Duration: time.Second}),
						},
					},
				},
			},
		},
		{
			desc: "peer with holdtime only",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:    42,
							ASN:      42,
							Address:  "1.2.3.4",
							HoldTime: ptr.To(metav1.Duration{Duration: 180 * time.Second}),
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						HoldTime:      ptr.To(180 * time.Second),
						KeepaliveTime: ptr.To(60 * time.Second),
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "peer with keepalive only",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:         42,
							ASN:           42,
							Address:       "1.2.3.4",
							KeepaliveTime: ptr.To(metav1.Duration{Duration: 60 * time.Second}),
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						HoldTime:      ptr.To(180 * time.Second),
						KeepaliveTime: ptr.To(60 * time.Second),
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "peer with zero hold/keepalive timers",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:         42,
							ASN:           42,
							Address:       "1.2.3.4",
							HoldTime:      ptr.To(metav1.Duration{Duration: 0 * time.Second}),
							KeepaliveTime: ptr.To(metav1.Duration{Duration: 0 * time.Second}),
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						HoldTime:      ptr.To(0 * time.Second),
						KeepaliveTime: ptr.To(0 * time.Second),
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "peer without hold/keepalive timers",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:   42,
							ASN:     42,
							Address: "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
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
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []metav1.LabelSelectorRequirement{
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
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []metav1.LabelSelectorRequirement{
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
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []metav1.LabelSelectorRequirement{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        100,
							ASN:          200,
							Address:      "2.3.4.5",
							EBGPMultiHop: false,
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
									MatchExpressions: []metav1.LabelSelectorRequirement{
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
					},
				},
			},
		},
		{
			desc: "invalid pool CIDR",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv3",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "advertisement with default BGP settings",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"1.2.3.0/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv3",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "bad aggregation length (too long)",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
							AggregationLength: ptr.To[int32](34),
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
							AggregationLength: ptr.To[int32](26),
						},
					},
				},
			},
		},
		{
			desc: "different local pref - same peers and nodes",
			crs: ClusterResources{
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 100,
						},
					},
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 200,
						},
					},
				},
			},
		},
		{
			desc: "different local pref - different ipv4 aggregation length",
			crs: ClusterResources{
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref:         100,
							AggregationLength: ptr.To[int32](24),
						},
					},
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 200,
						},
					},
				},
			},
		},
		{
			desc: "different local pref - different aggregation lengths",
			crs: ClusterResources{
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref:           100,
							AggregationLength:   ptr.To[int32](24),
							AggregationLengthV6: ptr.To[int32](120),
						},
					},
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 200,
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR: []*net.IPNet{
							ipnet("10.20.30.40/24"),
						},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   24,
								AggregationLengthV6: 120,
								LocalPref:           100,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node1": true, "node2": true},
							},
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           200,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node1": true, "node2": true},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "different local pref - same peers & different nodes",
			crs: ClusterResources{
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node1",
							Labels: map[string]string{"node": "node1"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node2",
							Labels: map[string]string{"node": "node2"},
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 100,
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"node": "node1"},
								},
							},
						},
					},
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 200,
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"node": "node2"},
								},
							},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR: []*net.IPNet{
							ipnet("10.20.30.40/24"),
						},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node1": true},
							},
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           200,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node2": true},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "different local pref - different peers & same nodes",
			crs: ClusterResources{
				Nodes: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.30.40/24",
							},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 100,
							Peers:     []string{"peer1"},
						},
					},
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							LocalPref: 200,
							Peers:     []string{"peer2"},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR: []*net.IPNet{
							ipnet("10.20.30.40/24"),
						},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Peers:               []string{"peer1"},
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node1": true, "node2": true},
							},
							{
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           200,
								Peers:               []string{"peer2"},
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"node1": true, "node2": true},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "aggregation length by range",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
							AggregationLength: ptr.To[int32](26),
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
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
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "aggregation length by range, too wide",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
							AggregationLength: ptr.To[int32](24),
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{Name: testAdvName},
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool2"},
						Spec:       v1beta1.IPAddressPoolSpec{},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool2"},
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
						ObjectMeta: metav1.ObjectMeta{Name: "pool1"},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								" 10.0.0.0/8",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pool2"},
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "default",
					},
				},
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv3",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
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
					"bgpsecret": {Type: corev1.SecretTypeOpaque, ObjectMeta: metav1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"}},
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
					"bgpsecret": {Type: corev1.SecretTypeBasicAuth, ObjectMeta: metav1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"}},
				},
			},
		},
		{
			desc: "BGP Peer with a valid secret",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
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
					"bgpsecret": {Type: corev1.SecretTypeBasicAuth, ObjectMeta: metav1.ObjectMeta{Name: "bgpsecret", Namespace: "metallb-system"},
						Data: map[string][]byte{"password": []byte("nopass")}},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:           "peer1",
						MyASN:          42,
						ASN:            42,
						Addr:           net.ParseIP("1.2.3.4"),
						Port:           179,
						NodeSelectors:  []labels.Selector{labels.Everything()},
						BFDProfile:     "",
						SecretPassword: "nopass",
						PasswordRef: corev1.SecretReference{
							Name:      "bgpsecret",
							Namespace: "metallb-system",
						},
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "nondefault",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval:  ptr.To(uint32(50)),
							TransmitInterval: ptr.To(uint32(51)),
							DetectMultiplier: ptr.To(uint32(52)),
							EchoInterval:     ptr.To(uint32(54)),
							EchoMode:         ptr.To(true),
							PassiveMode:      ptr.To(true),
							MinimumTTL:       ptr.To(uint32(55)),
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv3",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{
					"nondefault": {
						Name:             "nondefault",
						ReceiveInterval:  ptr.To(uint32(50)),
						DetectMultiplier: ptr.To(uint32(52)),
						TransmitInterval: ptr.To(uint32(51)),
						EchoInterval:     ptr.To(uint32(54)),
						MinimumTTL:       ptr.To(uint32(55)),
						EchoMode:         true,
						PassiveMode:      true,
					},
				},
				Peers: map[string]*Peer{},
			},
		},
		{
			desc: "BFD Profile with too low receive interval",
			crs: ClusterResources{

				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval: ptr.To(uint32(2)),
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "default",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval: ptr.To(uint32(90000)),
						},
					},
				},
			},
		},
		{
			desc: "Session with ipv6 and bfd echo mode",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"fc00:f853:0ccd:e793::/64",
							},
						},
					},
				},
				BFDProfiles: []v1beta1.BFDProfile{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nondefault",
						},
						Spec: v1beta1.BFDProfileSpec{
							ReceiveInterval:  ptr.To(uint32(50)),
							TransmitInterval: ptr.To(uint32(51)),
							DetectMultiplier: ptr.To(uint32(52)),
							EchoInterval:     ptr.To(uint32(54)),
							EchoMode:         ptr.To(true),
							PassiveMode:      ptr.To(true),
							MinimumTTL:       ptr.To(uint32(55)),
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
					},
				},
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        42,
							ASN:          142,
							Address:      "1.2.3.4",
							Port:         1179,
							HoldTime:     ptr.To(metav1.Duration{Duration: 180 * time.Second}),
							RouterID:     "10.20.30.40",
							SrcAddress:   "10.20.30.40",
							EBGPMultiHop: true,
							BFDProfile:   "nondefault",
						},
					},
				},
			},
		},
		{
			desc: "Session with ipv4 and bfd echo mode",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "with-echo",
						},
						Spec: v1beta1.BFDProfileSpec{
							EchoMode: ptr.To(true),
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv",
						},
					},
				},
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							ASN:        142,
							Address:    "1.2.3.4",
							Port:       1179,
							BFDProfile: "with-echo",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           142,
						Addr:          net.ParseIP("1.2.3.4"),
						Port:          1179,
						NodeSelectors: []labels.Selector{labels.Everything()},
						BFDProfile:    "with-echo"},
				},
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("1.2.3.4/24")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{
					"with-echo": {
						Name:     "with-echo",
						EchoMode: true,
					},
				},
			},
		},
		{
			desc: "config IPAddressPool with large communities CR",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:        42,
							ASN:          142,
							Address:      "1.2.3.4",
							Port:         1179,
							HoldTime:     ptr.To(metav1.Duration{Duration: 180 * time.Second}),
							RouterID:     "10.20.30.40",
							SrcAddress:   "10.20.30.40",
							EBGPMultiHop: true,
							VRFName:      "foo",
						},
					},
				},
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
								"10.50.0.0/24",
							},
							AvoidBuggyIPs: true,
							AutoAssign:    ptr.To(false),
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
							LocalPref:         uint32(100),
							Communities:       []string{"bar"},
							IPAddressPools:    []string{"pool1"},
							Peers:             []string{"peer1"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength:   ptr.To[int32](24),
							AggregationLengthV6: ptr.To[int32](64),
							IPAddressPools:      []string{"pool1"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv3",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"pool2"},
						},
					},
				},
				Communities: []v1beta1.Community{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "community",
						},
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Name:  "bar",
									Value: "large:123:64512:1234",
								},
							},
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           142,
						Addr:          net.ParseIP("1.2.3.4"),
						SrcAddr:       net.ParseIP("10.20.30.40"),
						Port:          1179,
						HoldTime:      ptr.To(180 * time.Second),
						KeepaliveTime: ptr.To(60 * time.Second),
						RouterID:      net.ParseIP("10.20.30.40"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  true,
						VRF:           "foo",
					},
				},
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:          "pool1",
						CIDR:          []*net.IPNet{ipnet("10.20.0.0/16"), ipnet("10.50.0.0/24")},
						AvoidBuggyIPs: true,
						AutoAssign:    false,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities: func() map[community.BGPCommunity]bool {
									c, _ := community.New("large:123:64512:1234")
									return map[community.BGPCommunity]bool{
										c: true,
									}
								}(),
								Nodes: map[string]bool{},
								Peers: []string{"peer1"},
							},
							{
								Name:                "adv2",
								AggregationLength:   24,
								AggregationLengthV6: 64,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "use ip pool selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: true,
						}},
					},
					"pool2": {
						Name:       "pool2",
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv2",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           200,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
							},
						},
						L2Advertisements: nil,
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "specify interfaces",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
							Interfaces: []string{"eth0"},
						},
					},
				},
			},
			want: &Config{
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						L2Advertisements: []*L2Advertisement{{
							Nodes:         map[string]bool{},
							AllInterfaces: false,
							Interfaces:    []string{"eth0"},
						}},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "use duplicate match labels in ip pool selectors - in BGP adv",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv2",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
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
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:              "pool1",
						CIDR:              []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign:        true,
						BGPAdvertisements: nil,
						L2Advertisements:  nil,
					},
					"pool2": {
						Name:              "pool2",
						CIDR:              []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign:        true,
						BGPAdvertisements: nil,
						L2Advertisements:  nil,
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "use node selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool1",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"10.20.0.0/16",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"second": true},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes: map[string]bool{
								"first": true,
							},
							AllInterfaces: true,
						}},
					},
					"pool2": {
						Name:       "pool2",
						CIDR:       []*net.IPNet{ipnet("30.0.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv2",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{"first": true},
							},
						},
						L2Advertisements: nil,
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "use duplicate match labels in node selectors",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
					},
				},
				L2Advs: []v1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
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
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						CIDR:       []*net.IPNet{ipnet("10.20.0.0/16")},
						AutoAssign: true,
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes: map[string]bool{
									"first":  true,
									"second": true,
								},
							},
						},
						L2Advertisements: []*L2Advertisement{{
							Nodes: map[string]bool{
								"first":  true,
								"second": true,
							},
							AllInterfaces: true,
						}},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
				Peers:       map[string]*Peer{},
			},
		},
		{
			desc: "advertisement with peer selector",
			crs: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
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
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv1",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							AggregationLength: ptr.To[int32](32),
							LocalPref:         uint32(100),
							IPAddressPools:    []string{"pool1"},
							Peers:             []string{"peer1"},
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						ASN:           42,
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools: &Pools{ByName: map[string]*Pool{
					"pool1": {
						Name:       "pool1",
						AutoAssign: true,
						CIDR:       []*net.IPNet{ipnet("1.2.3.0/24")},
						BGPAdvertisements: []*BGPAdvertisement{
							{
								Name:                "adv1",
								AggregationLength:   32,
								AggregationLengthV6: 128,
								LocalPref:           100,
								Communities:         map[community.BGPCommunity]bool{},
								Nodes:               map[string]bool{},
								Peers:               []string{"peer1"},
							},
						},
					},
				}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "peer with dynamic asn",
			crs: ClusterResources{

				Peers: []v1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "peer1",
						},
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							DynamicASN: v1beta2.InternalASNMode,
							Address:    "1.2.3.4",
						},
					},
				},
			},
			want: &Config{
				Peers: map[string]*Peer{
					"peer1": {
						Name:          "peer1",
						MyASN:         42,
						DynamicASN:    "internal",
						Addr:          net.ParseIP("1.2.3.4"),
						NodeSelectors: []labels.Selector{labels.Everything()},
						EBGPMultiHop:  false,
					},
				},
				Pools:       &Pools{ByName: map[string]*Pool{}},
				BFDProfiles: map[string]*BFDProfile{},
			},
		},
		{
			desc: "peer without asn or dynamic asn",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							ASN:        0,
							DynamicASN: "",
						},
					},
				},
			},
		},
		{
			desc: "peer with both asn and dynamic asn",
			crs: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							MyASN:      42,
							ASN:        42,
							DynamicASN: v1beta2.InternalASNMode,
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

func TestContainsAdvertisement(t *testing.T) {
	tests := []struct {
		desc    string
		advs    []*L2Advertisement
		toCheck *L2Advertisement
		expect  bool
	}{
		{
			desc: "Contain",
			advs: []*L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeA": true,
						"nodeB": true,
					},
					Interfaces:    []string{"eth0", "eth1"},
					AllInterfaces: false,
				},
				{
					Nodes: map[string]bool{
						"nodeB": true,
					},
					Interfaces:    []string{},
					AllInterfaces: true,
				},
			},
			toCheck: &L2Advertisement{
				Nodes: map[string]bool{
					"nodeB": true,
					"nodeA": true,
				},
				Interfaces:    []string{"eth1", "eth0"},
				AllInterfaces: false,
			},
			expect: true,
		},
		{
			desc: "Not contain: Nodes don't equal",
			advs: []*L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeA": true,
						"nodeB": true,
					},
					Interfaces:    []string{"eth0", "eth1"},
					AllInterfaces: false,
				},
			},
			toCheck: &L2Advertisement{
				Nodes: map[string]bool{
					"nodeB": true,
					"nodeC": true,
				},
				Interfaces:    []string{"eth1", "eth0"},
				AllInterfaces: false,
			},
			expect: false,
		},
		{
			desc: "Not contain: Interfaces don't equal",
			advs: []*L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeA": true,
						"nodeB": true,
					},
					Interfaces:    []string{"eth0", "eth1"},
					AllInterfaces: false,
				},
			},
			toCheck: &L2Advertisement{
				Nodes: map[string]bool{
					"nodeA": true,
					"nodeB": true,
				},
				Interfaces:    []string{"eth1"},
				AllInterfaces: false,
			},
			expect: false,
		},
		{
			desc: "Not contain: AllInterfaces doesn't equal",
			advs: []*L2Advertisement{
				{
					Nodes: map[string]bool{
						"nodeA": true,
						"nodeB": true,
					},
					Interfaces:    []string{"eth1"},
					AllInterfaces: false,
				},
			},
			toCheck: &L2Advertisement{
				Nodes: map[string]bool{
					"nodeA": true,
					"nodeB": true,
				},
				Interfaces:    []string{"eth1"},
				AllInterfaces: true,
			},
			expect: false,
		},
	}
	for _, test := range tests {
		result := containsAdvertisement(test.advs, test.toCheck)
		if result != test.expect {
			t.Errorf("%s: expect is %v, but result is %v", test.desc, test.expect, result)
		}
	}
}

func FuzzParseCIDR(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseCIDR(input)
	})
}
