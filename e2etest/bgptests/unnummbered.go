// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/container"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/frr"
	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/netdev"
	testservice "go.universe.tf/e2etest/pkg/service"

	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var FRRImage string
var _ = ginkgo.Describe("FRRK8S-MODE Unnumbered BGP", func() {
	var (
		testNamespace         string
		nodeWithP2PConnection corev1.Node
		externalP2PContainer  *frrcontainer.FRR

		cs clientset.Interface
	)

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		cs = k8sclient.New()
		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		nodeWithP2PConnection = allNodes.Items[0]

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
			dumpBGPInfo(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, cs, testNamespace, []*frrcontainer.FRR{externalP2PContainer})
		}
	})

	ginkgo.DescribeTable("Session is established and route is advertised", func(prefixSend []string, iface string, tweak func(svc *corev1.Service)) {
		rc := frrconfig.RouterConfigUnnumbered{
			ASNLocal:  metalLBASN,
			ASNRemote: metalLBASN,
			Hostname:  "tor1",
			Interface: iface,
			RouterID:  "1.1.1.1",
		}

		var err error
		ginkgo.By(fmt.Sprintf("creating p2p %s:%s -- %s:external-p2p-container", nodeWithP2PConnection.Name, iface, iface))
		externalP2PContainer, err = frrcontainer.CreateP2PPeerFor(nodeWithP2PConnection.Name, iface, FRRImage)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.By(fmt.Sprintf("updating frrconfig to %s", externalP2PContainer.Name))
		ginkgo.DeferCleanup(func() {
			err := frrcontainer.Delete([]*frrcontainer.FRR{externalP2PContainer})
			Expect(err).NotTo(HaveOccurred())
		})

		c, err := rc.Config()
		Expect(err).NotTo(HaveOccurred())
		err = externalP2PContainer.UpdateBGPConfigFile(c)
		Expect(err).NotTo(HaveOccurred())

		resources := config.Resources{
			Peers: []metallbv1beta2.BGPPeer{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tor",
						Namespace: metallb.Namespace,
					},
					Spec: metallbv1beta2.BGPPeerSpec{
						Interface:  iface,
						ASN:        rc.ASNRemote,
						MyASN:      rc.ASNLocal,
						BFDProfile: "simple",
					},
				},
			},
			BFDProfiles: []metallbv1beta1.BFDProfile{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple",
				},
			},
			},
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-test",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: prefixSend,
					},
				},
			},
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{
				{ObjectMeta: metav1.ObjectMeta{Name: "empty"}},
			},
		}

		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred(), "apply the CR in k8s api failed")

		nodeP2PContainer := executor.ForContainer(nodeWithP2PConnection.Name)
		nodeLLA, err := netdev.LinkLocalAddressForDevice(nodeP2PContainer, iface)
		Expect(err).NotTo(HaveOccurred())
		ginkgo.By("validating the node and p2p container peered")
		validateUnnumberedBGPPeering(externalP2PContainer, nodeLLA)

		svc, _ := testservice.CreateWithBackend(cs, testNamespace, "unnumbered-lb", tweak)
		ginkgo.By("checking the service gets an ip assigned")
		for _, i := range svc.Status.LoadBalancer.Ingress {
			ingressIP := jigservice.GetIngressPoint(&i)
			err = config.ValidateIPInRange(resources.Pools, ingressIP)
			Expect(err).NotTo(HaveOccurred())
		}

		ginkgo.By(fmt.Sprintf("validating the p2p peer %s received the routes from node", externalP2PContainer.Name))
		validatePeerRoutesViaDevice(externalP2PContainer, iface, nodeLLA, prefixSend)

	},
		ginkgo.Entry("IPV4", []string{"5.5.5.5/32"}, "net10", func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", []string{"5555::1/128"}, "net20", func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", []string{"5.5.5.5/32", "5555::1/128"}, "net30",
			func(svc *corev1.Service) { testservice.DualStack(svc) }),
	)
})

func validateUnnumberedBGPPeering(peer *frrcontainer.FRR, nodeLLA string) {
	ginkgo.By(fmt.Sprintf("validating BGP peering to %s", peer.Name))
	Eventually(func() error {
		neighbors, err := frr.NeighborsInfo(peer)
		if err != nil {
			return err
		}
		for _, n := range neighbors {
			if n.BGPNeighborAddr == nodeLLA && n.Connected && n.BFDInfo.Status == "Up" {
				return nil
			}
		}
		return fmt.Errorf("no connected neighbor was found with %s", nodeLLA)
	}, 2*time.Minute, 10*time.Second).ShouldNot(HaveOccurred(),
		"timed out waiting to validate nodes peered with the frr instance")
}

// validatePeerRoutesViaDevice validates that the peer has BGP routes to the
// specified prefixes with the expected next-hop address on the specified device.
func validatePeerRoutesViaDevice(peer executor.Executor, dev, nextHop string, prefixes ...[]string) {
	ginkgo.By(fmt.Sprintf("validating prefix %s to %s dev %s", prefixes, nextHop, dev))
	Eventually(func() error {
		nextHopAddr := netip.MustParseAddr(nextHop)
		want := make(map[netip.Prefix]map[netip.Addr]struct{})
		for _, prf := range prefixes {
			for _, p := range prf {
				want[netip.MustParsePrefix(p)] = map[netip.Addr]struct{}{nextHopAddr: {}}
			}
		}

		got, err := container.BGPRoutes(peer, dev)
		if err != nil {
			return err
		}
		if !cmp.Equal(want, got) {
			return fmt.Errorf("want %v\n got %v\n diff %v", want, got, cmp.Diff(want, got))
		}
		return nil
	}, 30*time.Second, 5*time.Second).ShouldNot(HaveOccurred(), fmt.Sprintf("peer should have the routes %s", prefixes))
}
