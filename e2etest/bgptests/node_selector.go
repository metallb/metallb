// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"

	"fmt"
	"strings"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/ipfamily"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"

	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = ginkgo.Describe("BGP Node Selector", func() {
	var cs clientset.Interface
	var f *framework.Framework
	var nodeToLabel *v1.Node

	ginkgo.AfterEach(func() {
		if nodeToLabel != nil {
			k8s.RemoveLabelFromNode(nodeToLabel.Name, "bgp-node-selector-test", cs)
		}

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			dumpBGPInfo(cs, f)
		}
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			framework.ExpectNoError(err)
		}
	})

	f = framework.NewDefaultFramework("bgp")

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
	})

	table.DescribeTable("Two services, two distinct advertisements with different node selectors",
		func(pairingIPFamily ipfamily.Family, addresses []string, nodesForFirstPool, nodesForSecondPool []int) {
			var allNodes *corev1.NodeList
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			expectedNodesForFirstPool := nodesForSelection(allNodes.Items, nodesForFirstPool)
			expectedNodesForSecondPool := nodesForSelection(allNodes.Items, nodesForSecondPool)

			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{addresses[0]},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{addresses[1]},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "first-adv",
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							NodeSelectors:  k8s.SelectorsForNodes(expectedNodesForFirstPool),
							IPAddressPools: []string{"first-pool"},
						},
					}, {
						ObjectMeta: metav1.ObjectMeta{
							Name: "second-adv",
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							NodeSelectors:  k8s.SelectorsForNodes(expectedNodesForSecondPool),
							IPAddressPools: []string{"second-pool"},
						},
					},
				},
			}
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
				framework.ExpectNoError(err)
			}

			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			firstService, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "first-lb", testservice.WithSpecificPool("first-pool"))
			defer testservice.Delete(cs, firstService)
			secondService, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "second-lb", testservice.WithSpecificPool("second-pool"))
			defer testservice.Delete(cs, secondService)

			checkServiceOnlyOnNodes(cs, firstService, expectedNodesForFirstPool, pairingIPFamily)
			checkServiceOnlyOnNodes(cs, secondService, expectedNodesForSecondPool, pairingIPFamily)
		},
		table.Entry("IPV4 - two on first, two on second", ipfamily.IPv4, []string{"192.168.10.0/24", "192.168.16.0/24"}, []int{0, 1}, []int{0, 1}),
		table.Entry("IPV4 - one on first, two on second", ipfamily.IPv4, []string{"192.168.10.0/24", "192.168.16.0/24"}, []int{0}, []int{0, 1}),
		table.Entry("IPV4 - zero on first, two on second", ipfamily.IPv4, []string{"192.168.10.0/24", "192.168.16.0/24"}, []int{}, []int{0, 1}),
		table.Entry("IPV6 - one on first, two on second", ipfamily.IPv6, []string{"fc00:f853:0ccd:e799::/116", "fc00:f853:0ccd:e800::/116"}, []int{0}, []int{1, 2}),
	)

	// this test is marked FFR only because of https://github.com/metallb/metallb/issues/1315
	table.DescribeTable("Single service, two advertisement with different node selectors FRR", func(pairingIPFamily ipfamily.Family, address string, nodesForFirstAdv, nodesForSecondAdv []int) {
		var allNodes *corev1.NodeList
		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)

		expectedNodesForFirstAdv := nodesForSelection(allNodes.Items, nodesForFirstAdv)
		expectedNodesForSecondAdv := nodesForSelection(allNodes.Items, nodesForSecondAdv)

		resources := metallbconfig.ClusterResources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "first-pool",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{address},
					},
				},
			},
			Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "first-adv",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						NodeSelectors:  k8s.SelectorsForNodes(expectedNodesForFirstAdv),
						Communities:    []string{CommunityNoAdv},
						IPAddressPools: []string{"first-pool"},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "second-adv",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						NodeSelectors:  k8s.SelectorsForNodes(expectedNodesForSecondAdv),
						Communities:    []string{CommunityGracefulShut},
						IPAddressPools: []string{"first-pool"},
					},
				},
			},
		}
		for _, c := range FRRContainers {
			err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
			framework.ExpectNoError(err)
		}

		err = ConfigUpdater.Update(resources)
		framework.ExpectNoError(err)

		svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "first-lb", testservice.TrafficPolicyCluster)
		defer testservice.Delete(cs, svc)

		checkCommunitiesOnlyOnNodes(cs, svc, CommunityNoAdv, expectedNodesForFirstAdv, pairingIPFamily)
		checkCommunitiesOnlyOnNodes(cs, svc, CommunityGracefulShut, expectedNodesForSecondAdv, pairingIPFamily)
	},
		table.Entry("IPV4 - two on first, two on second", ipfamily.IPv4, "192.168.10.0/24", []int{0, 1}, []int{0, 1}),
		table.Entry("IPV4 - one on first, two on second", ipfamily.IPv4, "192.168.10.0/24", []int{0}, []int{0, 1}),
		table.Entry("IPV4 - zero on first, two on second", ipfamily.IPv4, "192.168.10.0/24", []int{}, []int{0, 1}),
		table.Entry("IPV6 - one on first, two on second", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116", []int{0}, []int{1, 2}),
	)

	table.DescribeTable("One service, one advertisement with node selector, labeling node",
		func(pairingIPFamily ipfamily.Family, address string) {
			var allNodes *corev1.NodeList
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			ginkgo.By("Setting advertisement with node selector (no matching nodes)")
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{address},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-adv",
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							Communities:    []string{CommunityNoAdv},
							IPAddressPools: []string{"test-pool"},
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"bgp-node-selector-test": "true",
									},
								},
							},
						},
					},
				},
			}
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
				framework.ExpectNoError(err)
			}

			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)
			defer testservice.Delete(cs, svc)

			ginkgo.By("Validating service IP not advertised")
			checkServiceNotOnNodes(cs, svc, allNodes.Items, pairingIPFamily)

			nodeToLabel = &allNodes.Items[0]
			ginkgo.By(fmt.Sprintf("Adding advertisement label to node %s", nodeToLabel.Name))
			k8s.AddLabelToNode(nodeToLabel.Name, "bgp-node-selector-test", "true", cs)

			ginkgo.By(fmt.Sprintf("Validating service IP advertised by %s", nodeToLabel.Name))
			checkServiceOnlyOnNodes(cs, svc, []v1.Node{*nodeToLabel}, pairingIPFamily)

			ginkgo.By("Validating service IP not advertised by other nodes")
			nodesNotSelected := nodesNotSelected(allNodes.Items, []int{0})
			checkServiceNotOnNodes(cs, svc, nodesNotSelected, pairingIPFamily)
		},
		table.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		table.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.Context("Peer selector", func() {
		// nodeForPeers is a map between container index and the indexes of the nodes we want to peer it with.
		// we cant use strings to avoid making assumptions on the names of the containers / nodes.
		table.DescribeTable("IPV4 Should work with a limited set of nodes", func(nodesForPeers map[int][]int) {
			var allNodes *corev1.NodeList
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{"192.168.10.0/24"},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, ipfamily.IPv4,
					func(p *metallbv1beta2.BGPPeer) {
						for containerIndx, nodesIndexes := range nodesForPeers {
							if containerIndx >= len(FRRContainers) {
								ginkgo.Skip(fmt.Sprintf("Asking for container %d, not enough containers %d", containerIndx, len(FRRContainers)))
							}
							if !strings.Contains(p.Name, FRRContainers[containerIndx].Name) {
								continue
							}
							nodes := nodesForSelection(allNodes.Items, nodesIndexes)
							p.Spec.NodeSelectors = k8s.SelectorsForNodes(nodes)
							return
						}
						p.Spec.NodeSelectors = k8s.SelectorsForNodes([]v1.Node{})
					}),
			}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipfamily.IPv4)
				framework.ExpectNoError(err)
			}

			for i, c := range FRRContainers {
				selected := nodesForSelection(allNodes.Items, nodesForPeers[i])
				nonSelected := nodesNotSelected(allNodes.Items, nodesForPeers[i])
				validateFRRPeeredWithNodes(cs, selected, c, ipfamily.IPv4)
				validateFRRNotPeeredWithNodes(cs, nonSelected, c, ipfamily.IPv4)
			}

		},
			table.Entry("First to one, second to two", map[int][]int{
				0: []int{0},
				1: []int{1, 2},
			}),
			table.Entry("First to one, second to one, third to three", map[int][]int{
				0: []int{0},
				1: []int{1},
				2: []int{0, 1, 2},
			}),
			table.Entry("First to one, second to one, third to same as first", map[int][]int{
				0: []int{0},
				1: []int{1},
				2: []int{0},
			}),
		)
	})
})
