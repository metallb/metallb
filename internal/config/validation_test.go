// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"
	"time"

	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// peerOnNodes returns a BGPPeer for address "1.2.3.4" selecting nodes by hostname (OR semantics).
func peerOnNodes(hostnames ...string) v1beta2.BGPPeer {
	sels := make([]v1.LabelSelector, len(hostnames))
	for i, h := range hostnames {
		sels[i] = v1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/hostname": h}}
	}
	return v1beta2.BGPPeer{Spec: v1beta2.BGPPeerSpec{Address: "1.2.3.4", NodeSelectors: sels}}
}

// hostNode returns a Node with the given name and kubernetes.io/hostname label.
func hostNode(hostname string) corev1.Node {
	return corev1.Node{ObjectMeta: v1.ObjectMeta{
		Name:   hostname,
		Labels: map[string]string{"kubernetes.io/hostname": hostname},
	}}
}

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
				Pools: []v1beta1.IPAddressPool{
					{
						Spec: v1beta1.IPAddressPoolSpec{
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
			desc: "v6 address but pool not selected",
			config: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{"2001:db8::/64"},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"bar"},
						},
					},
				},
			},
		},
		{
			desc: "v6 address and selected",
			config: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "foo",
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{"2001:db8::/64"},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "bar",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"foo1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "bar",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"foo"},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "v6 address and selected by labels",
			config: ClusterResources{
				Pools: []v1beta1.IPAddressPool{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:   "foo",
							Labels: map[string]string{"key": "value"},
						},
						Spec: v1beta1.IPAddressPoolSpec{
							Addresses: []string{"2001:db8::/64"},
						},
					},
				},
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "bar",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"foo1"},
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "bar",
						},
						Spec: v1beta1.BGPAdvertisementSpec{
							IPAddressPoolSelectors: []v1.LabelSelector{
								{
									MatchLabels: map[string]string{"key": "value"},
								},
							},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "enable BGP GracefulRestart",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							EnableGracefulRestart: true,
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "disable BGP MP",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							DisableMP: true,
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "dynamic ASN",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							DynamicASN: "internal",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "interface",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Interface: "eth0",
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
							KeepaliveTime: &v1.Duration{Duration: time.Second},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "connect time",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							ConnectTime: ptr.To(v1.Duration{Duration: time.Second}),
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "large BGP community inside BGP Advertisement",
			config: ClusterResources{
				BGPAdvs: []v1beta1.BGPAdvertisement{
					{
						Spec: v1beta1.BGPAdvertisementSpec{
							Communities: []string{"large:123:456:789"},
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "large BGP community inside Community CR",
			config: ClusterResources{
				Communities: []v1beta1.Community{
					{
						Spec: v1beta1.CommunitySpec{
							Communities: []v1beta1.CommunityAlias{
								{
									Value: "large:123:456:789",
								},
							},
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
			desc: "myAsn set, one different but with different vrf",
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
							VRFName: "red",
						},
					},
				},
			},
		},
		{
			desc: "myAsn set, two different but with different vrf",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
							MyASN:   123,
							VRFName: "red",
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.5",
							MyASN:   123,
							VRFName: "red",
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
		}, {
			desc: "duplicate bgp address, different vrfs",
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
							VRFName: "red",
						},
					}, {
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
							VRFName: "green",
						},
					},
				},
			},
			mustFail: false,
		}, {
			desc: "duplicate bgp address, same vrf",
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
							VRFName: "red",
						},
					}, {
						Spec: v1beta2.BGPPeerSpec{
							Address: "1.2.3.4",
							VRFName: "red",
						},
					},
				},
			},
			mustFail: true,
		},
		{
			desc: "two peers with interface set different",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Interface: "eth0",
							MyASN:     123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Interface: "eth1",
							MyASN:     123,
						},
					},
				},
			},
			mustFail: false,
		},
		{
			desc: "two peers with interface set same",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{
						Spec: v1beta2.BGPPeerSpec{
							Interface: "eth0",
							MyASN:     123,
						},
					},
					{
						Spec: v1beta2.BGPPeerSpec{
							Interface: "eth0",
							MyASN:     123,
						},
					},
				},
			},
			mustFail: true,
		},
		// Node-selector-aware duplicate detection: same address is only a real
		// duplicate when both peers can be scheduled on the same node.
		{
			desc: "duplicate address, disjoint hostname selectors",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{peerOnNodes("node1"), peerOnNodes("node2")},
				Nodes: []corev1.Node{hostNode("node1"), hostNode("node2")},
			},
		},
		{
			desc: "duplicate address, overlapping hostname selectors",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{peerOnNodes("node1"), peerOnNodes("node1")},
				Nodes: []corev1.Node{hostNode("node1")},
			},
			mustFail: true,
		},
		{
			desc: "duplicate address, one peer without nodeSelector matches all nodes",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					{Spec: v1beta2.BGPPeerSpec{Address: "1.2.3.4"}}, // no selector = all nodes
					peerOnNodes("node1"),
				},
				Nodes: []corev1.Node{hostNode("node1")},
			},
			mustFail: true,
		},
		{
			desc: "duplicate address, disjoint selectors but no nodes known (conservative)",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{peerOnNodes("node1"), peerOnNodes("node2")},
				// No Nodes: conservative fallback must reject.
			},
			mustFail: true,
		},
		{
			desc: "three peers same address, all on distinct nodes",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					peerOnNodes("node1"),
					peerOnNodes("node2"),
					peerOnNodes("node3"),
				},
				Nodes: []corev1.Node{hostNode("node1"), hostNode("node2"), hostNode("node3")},
			},
		},
		{
			desc: "three peers same address, third overlaps with first",
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					peerOnNodes("node1"),
					peerOnNodes("node2"),
					peerOnNodes("node1"), // collides with first peer
				},
				Nodes: []corev1.Node{hostNode("node1"), hostNode("node2")},
			},
			mustFail: true,
		},
		{
			desc: "duplicate address, multiple nodeSelectors (OR), disjoint",
			// peer1: node1 OR node2; peer2: node3 OR node4 → no overlap
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					peerOnNodes("node1", "node2"),
					peerOnNodes("node3", "node4"),
				},
				Nodes: []corev1.Node{hostNode("node1"), hostNode("node2"), hostNode("node3"), hostNode("node4")},
			},
		},
		{
			desc: "duplicate address, multiple nodeSelectors (OR), overlap via second selector",
			// peer1: node1 OR node2; peer2: node3 OR node2 → node2 overlaps
			config: ClusterResources{
				Peers: []v1beta2.BGPPeer{
					peerOnNodes("node1", "node2"),
					peerOnNodes("node3", "node2"),
				},
				Nodes: []corev1.Node{hostNode("node1"), hostNode("node2"), hostNode("node3")},
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
