// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"time"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/frr"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	testservice "go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("BGP Service Selector", func() {
	var cs clientset.Interface
	testNamespace := ""
	var allNodes []corev1.Node

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
		testNamespace, err = k8s.CreateTestNamespace(cs, "bgpsvcsel")
		Expect(err).NotTo(HaveOccurred())

		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		allNodes = nodes.Items
	})

	ginkgo.DescribeTable("Multiple service selectors",
		func(pairingIPFamily ipfamily.Family, poolAddress string) {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-test-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddress},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "with-multiple-selectors",
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"tier": "frontend"},
								},
								{
									MatchLabels: map[string]string{"tier": "backend"},
								},
							},
						},
					},
				},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating services matching different selectors")
			frontendSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "frontend-svc",
				func(s *corev1.Service) {
					s.Labels = map[string]string{"tier": "frontend"}
				})
			defer testservice.Delete(cs, frontendSvc)

			backendSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "backend-svc",
				func(s *corev1.Service) {
					s.Labels = map[string]string{"tier": "backend"}
				})
			defer testservice.Delete(cs, backendSvc)

			otherSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "other-svc",
				func(s *corev1.Service) {
					s.Labels = map[string]string{"tier": "other"}
				})
			defer testservice.Delete(cs, otherSvc)

			ginkgo.By("Checking that frontend service is advertised")
			for _, c := range FRRContainers {
				validateService(frontendSvc, allNodes, c)
			}

			ginkgo.By("Checking that backend service is advertised")
			for _, c := range FRRContainers {
				validateService(backendSvc, allNodes, c)
			}

			ginkgo.By("Checking that other service is not advertised")
			Consistently(func() error {
				for _, c := range FRRContainers {
					frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
					if err != nil {
						return err
					}

					for _, ip := range otherSvc.Status.LoadBalancer.Ingress {
						_, found := frrRoutesV4[ip.IP]
						if pairingIPFamily == ipfamily.IPv6 {
							_, found = frrRoutesV6[ip.IP]
						}
						if found {
							return fmt.Errorf("other service IP %s should not be advertised to %s", ip.IP, c.Name)
						}
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)
})
