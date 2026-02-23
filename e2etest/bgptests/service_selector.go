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

	ginkgo.DescribeTable("Pool and service selectors together",
		func(pairingIPFamily ipfamily.Family, poolAddressA, poolAddressB string) {
			ginkgo.By("Setting up two pools with labels and advertisement with both selectors")
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "pool-a",
							Labels: map[string]string{"pool": "a"},
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddressA},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "pool-b",
							Labels: map[string]string{"pool": "b"},
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddressB},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-pool-a-expose-true",
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"pool": "a"},
								},
							},
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"expose": "true"},
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

			ginkgo.By("Creating service with expose=true requesting pool-a - should be advertised")
			svcMatching, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-matching",
				testservice.WithLabels(map[string]string{"expose": "true"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer testservice.Delete(cs, svcMatching)

			for _, c := range FRRContainers {
				validateService(svcMatching, allNodes, c)
			}

			ginkgo.By("Creating service with expose=true requesting pool-b - pool doesn't match advertisement")
			svcWrongPool, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-wrong-pool",
				testservice.WithLabels(map[string]string{"expose": "true"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer testservice.Delete(cs, svcWrongPool)

			checkServiceNotAdvertised := func(svc *corev1.Service, reason string) error {
				for _, c := range FRRContainers {
					frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
					if err != nil {
						return err
					}

					for _, ip := range svc.Status.LoadBalancer.Ingress {
						_, found := frrRoutesV4[ip.IP]
						if pairingIPFamily == ipfamily.IPv6 {
							_, found = frrRoutesV6[ip.IP]
						}
						if found {
							return fmt.Errorf("%s IP %s should NOT be advertised to %s (%s)", svc.Name, ip.IP, c.Name, reason)
						}
					}
				}
				return nil
			}

			ginkgo.By("Checking that svc-wrong-pool is NOT advertised (pool selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(svcWrongPool, "pool selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without expose label requesting pool-a - service selector doesn't match")
			svcNoLabel, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-no-label",
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer testservice.Delete(cs, svcNoLabel)

			ginkgo.By("Checking that svc-no-label is NOT advertised (service selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(svcNoLabel, "service selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without matching label requesting pool-b - neither matches")
			svcNeitherMatch, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-neither",
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer testservice.Delete(cs, svcNeitherMatch)

			ginkgo.By("Checking that svc-neither is NOT advertised (both selectors mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(svcNeitherMatch, "both selectors mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Verifying the matching service is still advertised after all checks")
			for _, c := range FRRContainers {
				validateService(svcMatching, allNodes, c)
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24", "192.168.20.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116", "fc00:f853:0ccd:e800::/116"),
	)

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
				testservice.WithLabels(map[string]string{"tier": "frontend"}))
			defer testservice.Delete(cs, frontendSvc)

			backendSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "backend-svc",
				testservice.WithLabels(map[string]string{"tier": "backend"}))
			defer testservice.Delete(cs, backendSvc)

			otherSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "other-svc",
				testservice.WithLabels(map[string]string{"tier": "other"}))
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
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)
})
