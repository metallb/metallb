// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/frr"
	"go.universe.tf/e2etest/pkg/ipfamily"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
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
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	environmentLabelKey         = "environment"
	environmentLabelProd        = "production"
	environmentLabelStaged      = "staged"
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

			ginkgo.By("Checking that svc-wrong-pool is NOT advertised (pool selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcWrongPool, "pool selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without expose label requesting pool-a - service selector doesn't match")
			svcNoLabel, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-no-label",
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer testservice.Delete(cs, svcNoLabel)

			ginkgo.By("Checking that svc-no-label is NOT advertised (service selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcNoLabel, "service selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without matching label requesting pool-b - neither matches")
			svcNeitherMatch, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-neither",
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer testservice.Delete(cs, svcNeitherMatch)

			ginkgo.By("Checking that svc-neither is NOT advertised (both selectors mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcNeitherMatch, "both selectors mismatch")
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

	ginkgo.DescribeTable("Service selector updates propagate to BGP routes by updating BGPAdvertisement ServiceSelectors",
		func(pairingIPFamily ipfamily.Family, poolAddress string) {
			advName := "adv-environment-selector"
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-update-pool",
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
							Name:      advName,
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{environmentLabelKey: environmentLabelProd},
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

			ginkgo.By("Creating production and staged services")
			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-update-prod",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelProd}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-update-staged",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelStaged}))
			defer testservice.Delete(cs, svcStaged)

			ginkgo.By("Initially only the production service should be advertised")
			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
			}
			Consistently(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcStaged,
					"staged must not match production selector")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Updating BGPAdvertisement ServiceSelectors to match staged")
			var adv metallbv1beta1.BGPAdvertisement
			err = ConfigUpdater.Client().Get(context.Background(), types.NamespacedName{
				Namespace: ConfigUpdater.Namespace(),
				Name:      advName,
			}, &adv)
			Expect(err).NotTo(HaveOccurred())
			adv.Spec.ServiceSelectors = []metav1.LabelSelector{
				{MatchLabels: map[string]string{environmentLabelKey: environmentLabelStaged}},
			}
			err = ConfigUpdater.Client().Update(context.Background(), &adv)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("After update, staged service should be advertised and production should not")
			for _, c := range FRRContainers {
				validateService(svcStaged, allNodes, c)
			}
			Eventually(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcProd,
					"production service must no longer match selector")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("Service selector updates propagate to BGP routes by updating Service labels",
		func(pairingIPFamily ipfamily.Family, poolAddress string) {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-update-pool",
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
							Name:      "adv-environment-selector",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{environmentLabelKey: environmentLabelProd},
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

			ginkgo.By("Creating production and staged services")
			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-update-prod",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelProd}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-update-staged",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelStaged}))
			defer testservice.Delete(cs, svcStaged)

			ginkgo.By("Initially only the production service should be advertised")
			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
			}
			Consistently(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcStaged,
					"staged must not match production selector")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Updating environment labels between the two services")
			svcProd, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svcStaged, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Updating service labels so staged matches selector and not production")
			svcProd.Labels[environmentLabelKey] = environmentLabelStaged
			svcStaged.Labels[environmentLabelKey] = environmentLabelProd
			_, err = cs.CoreV1().Services(testNamespace).Update(context.Background(), svcProd, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			_, err = cs.CoreV1().Services(testNamespace).Update(context.Background(), svcStaged, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Refreshing service objects after label updates before route checks")
			svcProd, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svcStaged, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("After update, staged service should be advertised and not production")
			for _, c := range FRRContainers {
				validateService(svcStaged, allNodes, c)
			}
			Eventually(func() error {
				return checkServiceNotAdvertised(FRRContainers, svcProd,
					"production service must no longer match selector")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("Service selectors with split aggregation length advertisements",
		func(pairingIPFamily ipfamily.Family, poolAddress string, aggLen int32, prodLBIP, stagedLBIP string) {
			const (
				poolName = "selector-aggregation-pool"
			)

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: poolName,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddress},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			}
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			ginkgo.By("Applying split advertisements: one with serviceSelector and one with aggregationLength only")
			aggOnlySpec := metallbv1beta1.BGPAdvertisementSpec{
				IPAddressPools: []string{poolName},
			}
			if pairingIPFamily == ipfamily.IPv6 {
				aggOnlySpec.AggregationLengthV6 = &aggLen
			} else {
				aggOnlySpec.AggregationLength = &aggLen
			}
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "adv-selector-only",
						Namespace: ConfigUpdater.Namespace(),
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools: []string{poolName},
						ServiceSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{environmentLabelKey: environmentLabelProd},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "adv-aggregation-only",
						Namespace: ConfigUpdater.Namespace(),
					},
					Spec: aggOnlySpec,
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating production and staged services")
			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "selector-agg-svc-prod",
				func(s *corev1.Service) {
					s.Spec.LoadBalancerIP = prodLBIP
				},
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelProd}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "selector-agg-svc-staged",
				func(s *corev1.Service) {
					s.Spec.LoadBalancerIP = stagedLBIP
				},
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelStaged}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcStaged)

			ginkgo.By("Verifying external FRR receives routes from both services")
			bits := 32
			if pairingIPFamily == ipfamily.IPv6 {
				bits = 128
			}
			routeHasAggregationLength := func(routes map[string]frr.Route, aggregatedPrefixIP string) error {
				r, ok := routes[aggregatedPrefixIP]
				if !ok {
					return fmt.Errorf("no route for aggregated prefix %q", aggregatedPrefixIP)
				}
				if r.Destination == nil {
					return fmt.Errorf("route for aggregated prefix %q has nil destination", aggregatedPrefixIP)
				}
				ones, _ := r.Destination.Mask.Size()
				if ones != int(aggLen) {
					return fmt.Errorf("route for aggregated prefix %q: want /%d advertised prefix, got /%d (%s)", aggregatedPrefixIP, aggLen, ones, r.Destination.String())
				}
				return nil
			}
			ginkgo.By("Deriving aggregated prefix IP from staged service LoadBalancerIP")
			ip := net.ParseIP(stagedLBIP)
			Expect(ip).NotTo(BeNil())
			subnet := &net.IPNet{
				IP:   ip.Mask(net.CIDRMask(int(aggLen), bits)),
				Mask: net.CIDRMask(int(aggLen), bits),
			}
			// ParseRoutes indexes maps by destination IP string (not full CIDR).
			aggregatedPrefixIP := subnet.IP.String()

			ginkgo.By("Validating production service and staged aggregated routes on all FRR containers")
			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)

				Eventually(func() error {
					routes, err := frr.RoutesForFamily(c, pairingIPFamily)
					if err != nil {
						return err
					}
					if err := routeHasAggregationLength(routes, aggregatedPrefixIP); err != nil {
						return fmt.Errorf("aggregated route for prefix %s not valid on %s: %w", aggregatedPrefixIP, c.Name, err)
					}
					return nil
				}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred(), "timed out waiting for aggregated route from staged service")
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24", int32(24), "192.168.10.10", "192.168.10.20"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, v6PoolAddresses, int32(124), "fc00:f853:ccd:e799::1", "fc00:f853:ccd:e799::2"),
	)

	ginkgo.DescribeTable("Overlapping BGP advertisements where selected service gets all communities and staged gets shared only",
		func(pairingIPFamily ipfamily.Family, poolAddresses string) {
			const prodOnlyCommunity = "65000:300"
			const allServicesCommunity = "65000:400"

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "merge-pool"},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddresses},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "adv-merge-prod-community",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"merge-pool"},
							Communities:    []string{prodOnlyCommunity},
							ServiceSelectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{environmentLabelKey: environmentLabelProd}},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "adv-merge-all-community",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{"merge-pool"},
							Communities:    []string{allServicesCommunity},
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

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "merge-svc-prod",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelProd}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "merge-pool"}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "merge-svc-staged",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelStaged}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "merge-pool"}))
			defer testservice.Delete(cs, svcStaged)

			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
				validateService(svcStaged, allNodes, c)
			}

			ginkgo.By("production routes carry both communities; staged carries only the shared community")
			for _, c := range FRRContainers {
				validateServiceInRoutesForCommunity(c, prodOnlyCommunity, pairingIPFamily, svcProd)
				validateServiceInRoutesForCommunity(c, allServicesCommunity, pairingIPFamily, svcProd)
				validateServiceNotInRoutesForCommunity(c, prodOnlyCommunity, pairingIPFamily, svcStaged)
				validateServiceInRoutesForCommunity(c, allServicesCommunity, pairingIPFamily, svcStaged)
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.50-192.168.10.200"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("Non-overlapping BGP advertisements apply distinct communities by service selector",
		func(pairingIPFamily ipfamily.Family, poolProdAddress, poolStagedAddress string) {
			const poolProd = "nonoverlap-pool-prod"
			const poolStaged = "nonoverlap-pool-staged"
			const nonOverlapCommunityProd = "65000:100"
			const nonOverlapCommunityStaged = "65000:200"

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolProd},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolProdAddress},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolStaged},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolStagedAddress},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "adv-nonoverlap-prod",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{poolProd},
							Communities:    []string{nonOverlapCommunityProd},
							ServiceSelectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{environmentLabelKey: environmentLabelProd}},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "adv-nonoverlap-staged",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{poolStaged},
							Communities:    []string{nonOverlapCommunityStaged},
							ServiceSelectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{environmentLabelKey: environmentLabelStaged}},
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

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "nonoverlap-svc-prod",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelProd}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolProd}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "nonoverlap-svc-staged",
				testservice.WithLabels(map[string]string{environmentLabelKey: environmentLabelStaged}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolStaged}))
			defer testservice.Delete(cs, svcStaged)

			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
				validateService(svcStaged, allNodes, c)
			}

			ginkgo.By("production routes use prod community only; staged routes use staged community only")
			for _, c := range FRRContainers {
				validateServiceInRoutesForCommunity(c, nonOverlapCommunityProd, pairingIPFamily, svcProd)
				validateServiceNotInRoutesForCommunity(c, nonOverlapCommunityStaged, pairingIPFamily, svcProd)
				validateServiceInRoutesForCommunity(c, nonOverlapCommunityStaged, pairingIPFamily, svcStaged)
				validateServiceNotInRoutesForCommunity(c, nonOverlapCommunityProd, pairingIPFamily, svcStaged)
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.50-192.168.10.120", "192.168.20.50-192.168.20.120"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116", "fc00:f853:0ccd:e800::/116"),
	)
})

// checkServiceNotAdvertised returns an error if any service LoadBalancer address is present in FRR routes.
func checkServiceNotAdvertised(containers []*frrcontainer.FRR, svc *corev1.Service, reason string) error {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return fmt.Errorf("service %s/%s has no ingress IPs", svc.Namespace, svc.Name)
	}
	for _, c := range containers {
		frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
		if err != nil {
			return err
		}
		for _, ing := range svc.Status.LoadBalancer.Ingress {
			ingressIP := jigservice.GetIngressPoint(&ing)
			if _, found := frrRoutesV4[ingressIP]; found {
				return fmt.Errorf("%s IP %s should NOT be advertised to %s (%s)", svc.Name, ingressIP, c.Name, reason)
			}
			if _, found := frrRoutesV6[ingressIP]; found {
				return fmt.Errorf("%s IP %s should NOT be advertised to %s (%s)", svc.Name, ingressIP, c.Name, reason)
			}
		}
	}
	return nil
}
