// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/frr"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"

	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var FRRImage string
var _ = ginkgo.XDescribe("Unnumbered BGP", func() {
	var (
		testNamespace string
		peer          *frrcontainer.FRR
	)

	ginkgo.BeforeEach(func() {
		if _, found := os.LookupEnv("RUN_FRR_CONTAINER_ON_HOST_NETWORK"); found {
			ginkgo.Skip("Skipping this test because RUN_FRR_CONTAINER_ON_HOST_NETWORK is set to true")
		}

		cs := k8sclient.New()
		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		node := allNodes.Items[0]

		peer, err = frrcontainer.SetupP2PPeer(FRRImage, node)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() {
			err := frrcontainer.Delete([]*frrcontainer.FRR{peer})
			Expect(err).NotTo(HaveOccurred())
		})

		err = k8s.DeleteNamespace(k8sclient.New(), "unnumbered-bgp")
		Expect(err).NotTo(HaveOccurred())
		testNamespace, err = k8s.CreateTestNamespace(k8sclient.New(), "unnumbered-bgp")
		Expect(err).NotTo(HaveOccurred())
		ginkgo.DeferCleanup(func() {
			err := k8s.DeleteNamespace(k8sclient.New(), testNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
			dumpBGPPeers(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, []*frrcontainer.FRR{peer})
		}
	})

	ginkgo.Context("FRR when applying unnumbered config", func() {
		ginkgo.It("Sessions should be established", func() {
			resources := config.Resources{
				Peers: []metallbv1beta2.BGPPeer{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "tor",
							Namespace: metallb.Namespace,
						},
						Spec: metallbv1beta2.BGPPeerSpec{
							Interface:     "net0",
							ASN:           65004,
							MyASN:         65000,
							NodeSelectors: []metav1.LabelSelector{},
						},
					},
				},
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{"5.5.5.5/32"},
						},
					},
				},
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{ObjectMeta: metav1.ObjectMeta{Name: "empty"}},
				},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred(), "apply the CR in k8s api failed")

			Eventually(func() error {
				neighbors, err := frr.NeighborsInfo(peer)
				Expect(err).NotTo(HaveOccurred())
				for _, n := range neighbors {
					if !n.Connected {
						return fmt.Errorf("node %v BGP session not established", n)
					}
				}
				return nil
			}, 2*time.Minute, 30*time.Second).ShouldNot(HaveOccurred(), "timed out waiting to validate nodes peered with the frr instance")
		})
	})
})
