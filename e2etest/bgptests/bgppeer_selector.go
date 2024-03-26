// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"

	"fmt"
	"strings"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	testservice "go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("BGP Peer Selector", func() {
	var cs clientset.Interface
	var frrContainerForAdv1 *frrcontainer.FRR
	var frrContainerForAdv2 *frrcontainer.FRR
	testNamespace := ""

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			dumpBGPInfo(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, cs, testNamespace)
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err := k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			Expect(err).NotTo(HaveOccurred())
		}
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "bgp")
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		var frrContainersForAdvertisement []*frrcontainer.FRR
		for _, c := range FRRContainers {
			// Connectivity between a multi hop FRR container to a BGP peer is going through
			// the single hop container.
			// The containers chosen here are the ones a service IP is advertised to.
			// Since the single hop container might not be familiar with the service IP
			// (if it wasn't chosen for the advertisement), the connectivity check will fail.
			if !strings.Contains(c.Name, "multi") {
				frrContainersForAdvertisement = append(frrContainersForAdvertisement, c)
			}
		}

		if len(frrContainersForAdvertisement) < 2 {
			ginkgo.Skip("This test requires 2 external frr containers")
		}

		frrContainerForAdv1 = frrContainersForAdvertisement[0]
		frrContainerForAdv2 = frrContainersForAdvertisement[1]
	})

	ginkgo.DescribeTable("A service IP will not be advertised to peers outside the BGPAdvertisement peers list",
		func(addressRange1 []string, addressRange2 []string, ipFamily ipfamily.Family, tweak testservice.Tweak) {
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			resources := config.Resources{
				Peers: metallb.PeersForContainers(FRRContainers, ipFamily),
			}

			pool1 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addresspool1",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: addressRange1,
				},
			}

			pool2 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addresspool2",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: addressRange2,
				},
			}

			ginkgo.By(fmt.Sprintf("setting peer selector for addresspool number 1 to peer %s", frrContainerForAdv1.Name))
			bgpPeersForAdv := getPeersNames(frrContainerForAdv1.Name, resources.Peers)
			bgpAdv1 := metallbv1beta1.BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-advertisement1",
				},
				Spec: metallbv1beta1.BGPAdvertisementSpec{
					IPAddressPools: []string{"test-addresspool1"},
					Peers:          bgpPeersForAdv,
				},
			}

			ginkgo.By(fmt.Sprintf("setting peer selector for addresspool number 2 to peer %s", frrContainerForAdv2.Name))
			bgpPeersForAdv = getPeersNames(frrContainerForAdv2.Name, resources.Peers)
			bgpAdv2 := metallbv1beta1.BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-advertisement2",
				},
				Spec: metallbv1beta1.BGPAdvertisementSpec{
					IPAddressPools: []string{"test-addresspool2"},
					Peers:          bgpPeersForAdv,
				},
			}

			resources.Pools = []metallbv1beta1.IPAddressPool{pool1, pool2}
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{bgpAdv1, bgpAdv2}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svcAdvertisement1, _ := testservice.CreateWithBackend(cs, testNamespace, "first-lb", testservice.WithSpecificPool("test-addresspool1"), tweak)
			defer testservice.Delete(cs, svcAdvertisement1)
			svcAdvertisement2, _ := testservice.CreateWithBackend(cs, testNamespace, "second-lb", testservice.WithSpecificPool("test-addresspool2"), tweak)
			defer testservice.Delete(cs, svcAdvertisement2)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("checking connectivity of service 1 to external frr container %s", frrContainerForAdv1.Name))
			validateService(svcAdvertisement1, allNodes.Items, frrContainerForAdv1)
			ginkgo.By("checking service 1 not advertised to other frr containers")
			validateServiceNotAdvertised(svcAdvertisement1, FRRContainers, frrContainerForAdv1.Name, ipFamily)

			ginkgo.By(fmt.Sprintf("checking connectivity of service 2 to external frr container %s", frrContainerForAdv2.Name))
			validateService(svcAdvertisement2, allNodes.Items, frrContainerForAdv2)
			ginkgo.By("checking service 2 not advertised to other frr containers")
			validateServiceNotAdvertised(svcAdvertisement2, FRRContainers, frrContainerForAdv2.Name, ipFamily)

			ginkgo.By("removing peer selectors from bgpadvertisements")
			resources.BGPAdvs[0].Spec.Peers = nil
			resources.BGPAdvs[1].Spec.Peers = nil
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				ginkgo.By(fmt.Sprintf("checking connectivity of service 1 to external frr container %s", c.Name))
				validateService(svcAdvertisement1, allNodes.Items, c)
			}

			for _, c := range FRRContainers {
				ginkgo.By(fmt.Sprintf("checking connectivity of service 2 to external frr container %s", c.Name))
				validateService(svcAdvertisement2, allNodes.Items, c)
			}
		},
		ginkgo.Entry("IPV4", []string{"192.168.10.0/24"},
			[]string{"192.168.16.0/24"}, ipfamily.IPv4, func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", []string{"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18"},
			[]string{"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26"}, ipfamily.IPv6, func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", []string{"192.168.10.0/24", "fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18"},
			[]string{"192.168.16.0/24", "fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26"}, ipfamily.DualStack,
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
			}),
		ginkgo.Entry("DUALSTACK - force V6 only", []string{"192.168.10.0/24", "fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18"},
			[]string{"192.168.16.0/24", "fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26"}, ipfamily.DualStack,
			func(svc *corev1.Service) {
				testservice.ForceV6(svc)
			}))

	ginkgo.DescribeTable("A service advertised through two different bgpadvertisements to two different peers",
		func(addressRange []string, ipFamily ipfamily.Family, tweak testservice.Tweak) {
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			resources := config.Resources{
				Peers: metallb.PeersForContainers(FRRContainers, ipFamily),
			}

			pool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-addresspool",
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: addressRange,
				},
			}

			ginkgo.By(fmt.Sprintf("setting bgpadvertisement 1 with peer selector to peer %s", frrContainerForAdv1.Name))
			bgpPeersForAdv := getPeersNames(frrContainerForAdv1.Name, resources.Peers)
			community1 := "65531:65281"
			bgpAdv1 := metallbv1beta1.BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-advertisement1",
				},
				Spec: metallbv1beta1.BGPAdvertisementSpec{
					IPAddressPools: []string{"test-addresspool"},
					Peers:          bgpPeersForAdv,
					Communities:    []string{community1},
				},
			}

			ginkgo.By(fmt.Sprintf("setting bgpadvertisement 2 with peer selector to peer %s", frrContainerForAdv2.Name))
			bgpPeersForAdv = getPeersNames(frrContainerForAdv2.Name, resources.Peers)
			community2 := "65532:65282"
			bgpAdv2 := metallbv1beta1.BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-advertisement2",
				},
				Spec: metallbv1beta1.BGPAdvertisementSpec{
					IPAddressPools: []string{"test-addresspool"},
					Peers:          bgpPeersForAdv,
					Communities:    []string{community2},
				},
			}

			resources.Pools = []metallbv1beta1.IPAddressPool{pool}
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{bgpAdv1, bgpAdv2}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			for _, c := range FRRContainers {
				if frrContainerForAdv1.Name == c.Name {
					ginkgo.By(fmt.Sprintf("checking service in routes of peer %s for community 1", c.Name))
					validateServiceInRoutesForCommunity(c, community1, ipFamily, svc)
				} else {
					ginkgo.By(fmt.Sprintf("checking service not in routes of peer %s for community 1", c.Name))
					validateServiceNotInRoutesForCommunity(c, community1, ipFamily, svc)
				}
			}

			for _, c := range FRRContainers {
				if frrContainerForAdv2.Name == c.Name {
					ginkgo.By(fmt.Sprintf("checking service in routes of peer %s for community 2", c.Name))
					validateServiceInRoutesForCommunity(c, community2, ipFamily, svc)
				} else {
					ginkgo.By(fmt.Sprintf("checking service not in routes of peer %s for community 2", c.Name))
					validateServiceNotInRoutesForCommunity(c, community2, ipFamily, svc)
				}
			}
		},
		ginkgo.Entry("IPV4", []string{"192.168.10.0/24"}, ipfamily.IPv4, func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", []string{"fc00:f853:0ccd:e799::/116"}, ipfamily.IPv6, func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", []string{"192.168.10.0/24", "fc00:f853:0ccd:e799::/116"},
			ipfamily.DualStack,
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
			}),
		ginkgo.Entry("DUALSTACK - force V6 only", []string{"192.168.10.0/24", "fc00:f853:0ccd:e799::/116"},
			ipfamily.DualStack,
			func(svc *corev1.Service) {
				testservice.ForceV6(svc)
			}))
})

func getPeersNames(frrContainerName string, peers []v1beta2.BGPPeer) []string {
	res := []string{}
	for _, p := range peers {
		if strings.Contains(p.Name, frrContainerName) {
			res = append(res, p.Name)
		}
	}
	return res
}
