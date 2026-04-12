// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go.universe.tf/e2etest/l2tests"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/frr"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/iprange"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/mac"
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
	mergeAdvertisementCommunity = "65000:300"
	mergeAdvertisementLocalPref = uint32(400)
	nonOverlapCommunityProd     = "65000:100"
	nonOverlapCommunityStaged   = "65000:200"
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

			ginkgo.By("Waiting for all nodes to be peered before validating advertised routes")
			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			ginkgo.By("Creating service with expose=true requesting pool-a - should be advertised")
			svcMatching, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-matching",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"expose": "true"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer testservice.Delete(cs, svcMatching)

			for _, c := range FRRContainers {
				validateService(svcMatching, allNodes, c)
			}

			ginkgo.By("Creating service with expose=true requesting pool-b - pool doesn't match advertisement")
			svcWrongPool, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-wrong-pool",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"expose": "true"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer testservice.Delete(cs, svcWrongPool)

			ginkgo.By("Checking that svc-wrong-pool is NOT advertised (pool selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, svcWrongPool.Namespace, svcWrongPool.Name, "pool selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without expose label requesting pool-a - service selector doesn't match")
			svcNoLabel, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-no-label",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer testservice.Delete(cs, svcNoLabel)

			ginkgo.By("Checking that svc-no-label is NOT advertised (service selector mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, svcNoLabel.Namespace, svcNoLabel.Name, "service selector mismatch")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("Creating service without matching label requesting pool-b - neither matches")
			svcNeitherMatch, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-neither",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"expose": "false"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer testservice.Delete(cs, svcNeitherMatch)

			ginkgo.By("Checking that svc-neither is NOT advertised (both selectors mismatch)")
			Consistently(func() error {
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, svcNeitherMatch.Namespace, svcNeitherMatch.Name, "both selectors mismatch")
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
			pool := poolAddress
			if pairingIPFamily == ipfamily.IPv6 {
				pool = l2tests.IPV6ServiceRange
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-test-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{pool},
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
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"tier": "frontend"}))
			defer testservice.Delete(cs, frontendSvc)

			backendSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "backend-svc",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{"tier": "backend"}))
			defer testservice.Delete(cs, backendSvc)

			otherSvc, _ := testservice.CreateWithBackend(cs, testNamespace, "other-svc",
				withIPFamilyForPool(pairingIPFamily),
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
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, otherSvc.Namespace, otherSvc.Name, "tier=other does not match advertisement selectors")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("Service selector updates propagate to BGP routes",
		func(pairingIPFamily ipfamily.Family, poolAddress string, updateBGPAdvertisement bool) {
			const (
				envLabelKey = "environment"
				envProd     = "production"
				envStaged   = "staged"
			)
			advName := "adv-environment-selector"
			pool := poolAddress
			if pairingIPFamily == ipfamily.IPv6 {
				pool = l2tests.IPV6ServiceRange
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-update-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{pool},
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
									MatchLabels: map[string]string{envLabelKey: envProd},
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
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{envLabelKey: envProd}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "svc-update-staged",
				withIPFamilyForPool(pairingIPFamily),
				testservice.WithLabels(map[string]string{envLabelKey: envStaged}))
			defer testservice.Delete(cs, svcStaged)

			ginkgo.By("Initially only the production service should be advertised")
			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
			}
			Consistently(func() error {
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, svcStaged.Namespace, svcStaged.Name, "staged must not match production selector")
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			if updateBGPAdvertisement {
				ginkgo.By("Updating BGPAdvertisement ServiceSelectors to match staged")
				var adv metallbv1beta1.BGPAdvertisement
				err = ConfigUpdater.Client().Get(context.Background(), types.NamespacedName{
					Namespace: ConfigUpdater.Namespace(),
					Name:      advName,
				}, &adv)
				Expect(err).NotTo(HaveOccurred())
				adv.Spec.ServiceSelectors = []metav1.LabelSelector{
					{MatchLabels: map[string]string{envLabelKey: envStaged}},
				}
				err = ConfigUpdater.Client().Update(context.Background(), &adv)
				Expect(err).NotTo(HaveOccurred())
			} else {
				ginkgo.By("Swapping environment labels between the two services")
				svcProd, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				svcStaged, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if svcProd.Labels == nil {
					svcProd.Labels = map[string]string{}
				}
				if svcStaged.Labels == nil {
					svcStaged.Labels = map[string]string{}
				}
				svcProd.Labels[envLabelKey] = envStaged
				svcStaged.Labels[envLabelKey] = envProd
				_, err = cs.CoreV1().Services(testNamespace).Update(context.Background(), svcProd, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
				_, err = cs.CoreV1().Services(testNamespace).Update(context.Background(), svcStaged, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
				svcProd, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				svcStaged, err = cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			}

			ginkgo.By("After update, staged service should be advertised and production should not")
			for _, c := range FRRContainers {
				validateService(svcStaged, allNodes, c)
			}
			Eventually(func() error {
				return checkServiceNotAdvertised(cs, FRRContainers, pairingIPFamily, svcProd.Namespace, svcProd.Name, "production service must no longer match selector")
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4 by updating BGPAdvertisement ServiceSelectors", ipfamily.IPv4, "192.168.10.0/24", true),
		ginkgo.Entry("IPV6 by updating BGPAdvertisement ServiceSelectors", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116", true),
		ginkgo.Entry("IPV4 by updating Service labels", ipfamily.IPv4, "192.168.10.0/24", false),
		ginkgo.Entry("IPV6 by updating Service labels", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116", false),
	)

	ginkgo.DescribeTable("Service selectors with aggregation length webhook validation",
		func(pairingIPFamily ipfamily.Family, poolAddress string) {
			const (
				poolName            = "selector-aggregation-pool"
				invalidAdvName      = "adv-selector-with-aggregation"
				selectorOnlyAdvName = "adv-selector-only"
				aggOnlyAdvName      = "adv-aggregation-only"
			)

			pool := poolAddress
			aggLenV4 := int32(24)
			aggLenV6 := int32(64)
			var aggLenV4Ptr *int32
			if pairingIPFamily == ipfamily.IPv6 {
				pool = l2tests.IPV6ServiceRange
				// Use /64 when possible, but keep aggregation valid for compact ranges (for example /126).
				parsedRanges, err := iprange.Parse(pool)
				Expect(err).NotTo(HaveOccurred())
				maxPrefixLen := int32(64)
				for _, cidr := range parsedRanges {
					ones, bits := cidr.Mask.Size()
					if bits == 128 && int32(ones) > maxPrefixLen {
						maxPrefixLen = int32(ones)
					}
				}
				aggLenV6 = maxPrefixLen
			} else {
				aggLenV4Ptr = &aggLenV4
			}
			aggLenV6Ptr := &aggLenV6

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: poolName,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{pool},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Rejecting BGPAdvertisement that combines serviceSelector with non-default aggregation lengths")
			invalidAggLenV4 := int32(24)
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      invalidAdvName,
						Namespace: ConfigUpdater.Namespace(),
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools:       []string{poolName},
						AggregationLength:    &invalidAggLenV4,
						AggregationLengthV6:  aggLenV6Ptr,
						ServiceSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{"environment": "production"},
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(SatisfyAny(
				ContainSubstring("serviceSelectors and aggregationLength are mutually exclusive"),
				ContainSubstring("invalid aggregation length"),
			))

			ginkgo.By("Accepting split advertisements: one with serviceSelector and one with aggregationLength only")
			resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      selectorOnlyAdvName,
						Namespace: ConfigUpdater.Namespace(),
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools: []string{poolName},
						ServiceSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{"environment": "production"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      aggOnlyAdvName,
						Namespace: ConfigUpdater.Namespace(),
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools:      []string{poolName},
						AggregationLength:   aggLenV4Ptr,
						AggregationLengthV6: aggLenV6Ptr,
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating production and staged services")
			svcProd, _ := testservice.CreateWithBackend(cs, testNamespace, "selector-agg-svc-prod",
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "production"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "selector-agg-svc-staged",
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "staged"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcStaged)

			ginkgo.By("Verifying external FRR receives routes from both services")
			expectedAggLen := aggLenV4
			if pairingIPFamily == ipfamily.IPv6 {
				expectedAggLen = aggLenV6
			}
			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
				Eventually(func() error {
					svc, err := cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(svc.Status.LoadBalancer.Ingress) == 0 {
						return fmt.Errorf("service %s/%s has no ingress IPs", svc.Namespace, svc.Name)
					}
					routes, err := frr.RoutesForFamily(c, pairingIPFamily)
					if err != nil {
						return err
					}
					for _, ing := range svc.Status.LoadBalancer.Ingress {
						ingressIP := jigservice.GetIngressPoint(&ing)
						ip := net.ParseIP(ingressIP)
						if ip == nil {
							return fmt.Errorf("invalid ingress IP %q", ingressIP)
						}
						bits := 32
						if pairingIPFamily == ipfamily.IPv6 {
							bits = 128
						}
						subnet := &net.IPNet{IP: ip.Mask(net.CIDRMask(int(expectedAggLen), bits)), Mask: net.CIDRMask(int(expectedAggLen), bits)}
						key := subnet.IP.String() // ParseRoutes map is keyed by destination IP string, not CIDR.
						route, ok := routes[key]
						if !ok {
							return fmt.Errorf("aggregated route key %q for %s not found on %s", key, ingressIP, c.Name)
						}
						if route.Destination == nil {
							return fmt.Errorf("aggregated route key %q has nil destination on %s", key, c.Name)
						}
						ones, _ := route.Destination.Mask.Size()
						if ones != int(expectedAggLen) {
							return fmt.Errorf("route %q on %s has prefix /%d, expected /%d", route.Destination.String(), c.Name, ones, expectedAggLen)
						}
						if err := frr.RoutesMatchNodes(allNodes, route, pairingIPFamily, c.RouterConfig.VRF); err != nil {
							return fmt.Errorf("peer: %s errored matching nodes for %s: %w", c.Name, route.Destination.String(), err)
						}
					}
					return nil
				}, 4*time.Minute, 1*time.Second).ShouldNot(HaveOccurred(), "timed out waiting for aggregated route from staged service")
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.0/24"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("BGPAdvertisement and L2Advertisement service selectors apply to matching services only",
		func(pairingIPFamily ipfamily.Family, poolV4, poolV6 string, runV4, runV6 bool) {
			const poolName = "bgp-l2-selector-pool"
			const envKey = "environment"
			const envProd = "production"
			const envStaged = "staged"

			addresses := []string{}
			if runV4 {
				addresses = append(addresses, poolV4)
			}
			if runV6 {
				addresses = append(addresses, poolV6)
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolName},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: addresses,
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "bgp-adv-prod-only",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{poolName},
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{envKey: envProd},
								},
							},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "l2-adv-prod-only",
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							IPAddressPools: []string{poolName},
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{envKey: envProd},
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

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			createSvc := func(name, env string, familyTweak testservice.Tweak) *corev1.Service {
				svc, _ := testservice.CreateWithBackend(cs, testNamespace, name,
					testservice.TrafficPolicyCluster,
					familyTweak,
					testservice.WithLabels(map[string]string{envKey: env}),
					testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
				Expect(svc).NotTo(BeNil())
				return svc
			}

			waitForIngress := func(svc *corev1.Service) *corev1.Service {
				var latest *corev1.Service
				Eventually(func() error {
					s, err := cs.CoreV1().Services(svc.Namespace).Get(context.Background(), svc.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(s.Status.LoadBalancer.Ingress) == 0 {
						return fmt.Errorf("service %s/%s has no ingress IPs yet", s.Namespace, s.Name)
					}
					latest = s
					return nil
				}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
				return latest
			}

			expectAddressResolution := func(svc *corev1.Service, want bool) {
				check := func() error {
					svcLatest, err := cs.CoreV1().Services(svc.Namespace).Get(context.Background(), svc.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if len(svcLatest.Status.LoadBalancer.Ingress) == 0 {
						return fmt.Errorf("service %s/%s has no ingress IPs", svcLatest.Namespace, svcLatest.Name)
					}
					for _, ing := range svcLatest.Status.LoadBalancer.Ingress {
						ingressIP := jigservice.GetIngressPoint(&ing)
						if err := mac.RequestAddressResolution(ingressIP, executor.Host); err != nil {
							return err
						}
					}
					return nil
				}
				if want {
					Eventually(check, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
				} else {
					Consistently(check, 5*time.Second, 1*time.Second).Should(HaveOccurred())
				}
			}

			var prodSvcs []*corev1.Service
			var stagedSvcs []*corev1.Service
			if runV4 {
				prodV4 := createSvc("svc-prod4", envProd, testservice.ForceV4)
				stagedV4 := createSvc("svc-stg4", envStaged, testservice.ForceV4)
				prodSvcs = append(prodSvcs, prodV4)
				stagedSvcs = append(stagedSvcs, stagedV4)
			}
			if runV6 {
				prodV6 := createSvc("svc-prod6", envProd, testservice.ForceV6)
				stagedV6 := createSvc("svc-stg6", envStaged, testservice.ForceV6)
				prodSvcs = append(prodSvcs, prodV6)
				stagedSvcs = append(stagedSvcs, stagedV6)
			}
			for _, svc := range append(prodSvcs, stagedSvcs...) {
				defer testservice.Delete(cs, svc)
			}

			for i, svc := range prodSvcs {
				prodSvcs[i] = waitForIngress(svc)
			}
			for i, svc := range stagedSvcs {
				stagedSvcs[i] = waitForIngress(svc)
			}

			ginkgo.By("Verifying only production services are present in BGP routes")
			for _, svc := range prodSvcs {
				for _, c := range FRRContainers {
					validateService(svc, allNodes, c)
				}
			}
			for _, svc := range stagedSvcs {
				Eventually(func() error {
					ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
					return checkServiceNotAdvertised(cs, FRRContainers, ipfamily.ForAddress(net.ParseIP(ingressIP)), svc.Namespace, svc.Name, "service selector mismatch for BGPAdvertisement")
				}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			}

			ginkgo.By("Verifying ARP/NDP responses only for production services")
			for _, svc := range prodSvcs {
				expectAddressResolution(svc, true)
			}
			for _, svc := range stagedSvcs {
				expectAddressResolution(svc, false)
			}
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, l2tests.IPV4ServiceRange, "", true, false),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "", l2tests.IPV6ServiceRange, false, true),
		ginkgo.Entry("DUALSTACK", ipfamily.DualStack, l2tests.IPV4ServiceRange, l2tests.IPV6ServiceRange, true, true),
	)

	ginkgo.DescribeTable("Overlapping BGP advertisements merge attributes with service selectors",
		func(pairingIPFamily ipfamily.Family, poolAddresses string) {
			const poolName = "merge-pool"
			const advCommunityName = "adv-merge-community"
			const advLocalPrefName = "adv-merge-localpref"

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolName},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAddresses},
						},
					},
				},
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{
					// Production matches both advs; MetalLB rejects overlapping ads for one service
					// if LOCAL_PREF differs (checkBGPAdvConflicts / conflictingAdvertisements).
					// Use the same localPref on the community adv as on the catch-all adv.
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      advCommunityName,
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{poolName},
							LocalPref:      mergeAdvertisementLocalPref,
							Communities:    []string{mergeAdvertisementCommunity},
							ServiceSelectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{"environment": "production"}},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      advLocalPrefName,
							Namespace: ConfigUpdater.Namespace(),
						},
						Spec: metallbv1beta1.BGPAdvertisementSpec{
							IPAddressPools: []string{poolName},
							LocalPref:      mergeAdvertisementLocalPref,
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
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "production"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "merge-svc-staged",
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "staged"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolName}))
			defer testservice.Delete(cs, svcStaged)

			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
				validateService(svcStaged, allNodes, c)
			}

			ginkgo.By("production routes carry merge community and localPref on iBGP; staged only localPref")
			Eventually(func() error {
				svcP, err := cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				svcS, err := cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				return verifyServiceBGPAttributes(FRRContainers, svcP, svcS, mergeAdvertisementCommunity, mergeAdvertisementLocalPref, true, false)
			}, 4*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("deleting BGPAdvertisement with service selector and community")
			advDel := &metallbv1beta1.BGPAdvertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      advCommunityName,
					Namespace: ConfigUpdater.Namespace(),
				},
			}
			Expect(ConfigUpdater.Client().Delete(context.Background(), advDel)).To(Succeed())

			ginkgo.By("all service routes have localPref only on iBGP, without merge community")
			Eventually(func() error {
				svcP, err := cs.CoreV1().Services(testNamespace).Get(context.Background(), svcProd.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				svcS, err := cs.CoreV1().Services(testNamespace).Get(context.Background(), svcStaged.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				return verifyServiceBGPAttributes(FRRContainers, svcP, svcS, mergeAdvertisementCommunity, mergeAdvertisementLocalPref, false, false)
			}, 4*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		},
		ginkgo.Entry("IPV4", ipfamily.IPv4, "192.168.10.50-192.168.10.200"),
		ginkgo.Entry("IPV6", ipfamily.IPv6, "fc00:f853:0ccd:e799::/116"),
	)

	ginkgo.DescribeTable("Non-overlapping BGP advertisements apply distinct communities by service selector",
		func(pairingIPFamily ipfamily.Family, poolProdV4, poolStagedV4 string) {
			const poolProd = "nonoverlap-pool-prod"
			const poolStaged = "nonoverlap-pool-staged"

			poolProdAddr, poolStagedAddr := poolProdV4, poolStagedV4

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolProd},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolProdAddr},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: poolStaged},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolStagedAddr},
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
								{MatchLabels: map[string]string{"environment": "production"}},
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
								{MatchLabels: map[string]string{"environment": "staged"}},
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
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "production"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolProd}))
			defer testservice.Delete(cs, svcProd)

			svcStaged, _ := testservice.CreateWithBackend(cs, testNamespace, "nonoverlap-svc-staged",
				withIPFamilyForPool(pairingIPFamily),
				testservice.TrafficPolicyCluster,
				testservice.WithLabels(map[string]string{"environment": "staged"}),
				testservice.WithAnnotations(map[string]string{"metallb.io/address-pool": poolStaged}))
			defer testservice.Delete(cs, svcStaged)

			for _, c := range FRRContainers {
				validateService(svcProd, allNodes, c)
				validateService(svcStaged, allNodes, c)
			}

			ginkgo.By("production routes use prod community only; staged routes use staged community only")
			for _, c := range FRRContainers {
				if !strings.Contains(c.Name, "ibgp") {
					continue
				}
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

// withIPFamilyForPool makes the LoadBalancer Service single-stack for the pool under test.
// On a dual-stack kind cluster, the default Service may otherwise request IPv4 while the
// pool is IPv6-only (or the reverse), and MetalLB never assigns an external IP.
func withIPFamilyForPool(f ipfamily.Family) testservice.Tweak {
	if f == ipfamily.IPv6 {
		return testservice.ForceV6
	}
	return testservice.ForceV4
}

// checkServiceNotAdvertised returns an error if any service LoadBalancer address is present in FRR routes.
func checkServiceNotAdvertised(cs clientset.Interface, frrContainers []*frrcontainer.FRR, ipFamily ipfamily.Family, svcNamespace, svcName, reason string) error {
	svc, err := cs.CoreV1().Services(svcNamespace).Get(context.Background(), svcName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return fmt.Errorf("service %s/%s has no ingress IPs", svc.Namespace, svc.Name)
	}
	for _, c := range frrContainers {
		frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
		if err != nil {
			return err
		}
		for _, ing := range svc.Status.LoadBalancer.Ingress {
			ingressIP := jigservice.GetIngressPoint(&ing)
			_, found := frrRoutesV4[ingressIP]
			if ipFamily == ipfamily.IPv6 {
				_, found = frrRoutesV6[ingressIP]
			}
			if found {
				return fmt.Errorf("%s IP %s should NOT be advertised to %s (%s)", svc.Name, ingressIP, c.Name, reason)
			}
		}
	}
	return nil
}

func verifyPrefixInCommunity(c *frrcontainer.FRR, prefix, community string, wantPresent bool) error {
	fam := ipfamily.ForAddress(net.ParseIP(prefix))
	routes, err := frr.RoutesForCommunity(c, community, fam)
	if err != nil {
		return err
	}
	_, inCommunityTable := routes[prefix]
	if wantPresent {
		if !inCommunityTable {
			return fmt.Errorf("prefix %s not found for community %s on %s", prefix, community, c.Name)
		}
		return nil
	}
	if inCommunityTable {
		return fmt.Errorf("prefix %s should not be announced with community %s on %s", prefix, community, c.Name)
	}
	return nil
}

// verifyPrefixInCommunityAnyIBGP succeeds if at least one iBGP test router sees the community on
// the prefix. Multi-hop vs single-hop FRR views can differ; we only need one correct iBGP witness.
// eBGP peers are skipped (community checks are iBGP-scoped in this suite).
func verifyPrefixInCommunityAnyIBGP(containers []*frrcontainer.FRR, prefix, community string) error {
	var joined error
	for _, c := range containers {
		if !strings.Contains(c.Name, "ibgp") {
			continue
		}
		if err := verifyPrefixInCommunity(c, prefix, community, true); err == nil {
			return nil
		} else {
			joined = errors.Join(joined, err)
		}
	}
	return fmt.Errorf("prefix %s for community %s: not found on any ibgp peer: %w", prefix, community, joined)
}

// verifyPrefixInCommunityEveryIBGP requires every iBGP test router to match wantPresent for the prefix.
func verifyPrefixInCommunityEveryIBGP(containers []*frrcontainer.FRR, prefix, community string, wantPresent bool) error {
	for _, c := range containers {
		if !strings.Contains(c.Name, "ibgp") {
			continue
		}
		if err := verifyPrefixInCommunity(c, prefix, community, wantPresent); err != nil {
			return err
		}
	}
	return nil
}

func verifyLocalPrefOnIBGPSession(c *frrcontainer.FRR, prefix string, want uint32) error {
	if !strings.Contains(c.Name, "ibgp") {
		return nil
	}
	fam := ipfamily.ForAddress(net.ParseIP(prefix))
	lp, err := frr.LocalPrefForPrefix(c, prefix, fam)
	if err != nil {
		return err
	}
	if lp != want {
		return fmt.Errorf("localPref for %s on %s: got %d want %d", prefix, c.Name, lp, want)
	}
	return nil
}

func verifyServiceBGPAttributes(containers []*frrcontainer.FRR, svcProd, svcStaged *corev1.Service, community string, localPref uint32, prodHasCommunity, stagedHasCommunity bool) error {
	for _, ing := range svcProd.Status.LoadBalancer.Ingress {
		p := jigservice.GetIngressPoint(&ing)
		if prodHasCommunity {
			if err := verifyPrefixInCommunityAnyIBGP(containers, p, community); err != nil {
				return err
			}
		} else {
			if err := verifyPrefixInCommunityEveryIBGP(containers, p, community, false); err != nil {
				return err
			}
		}
	}
	for _, ing := range svcStaged.Status.LoadBalancer.Ingress {
		p := jigservice.GetIngressPoint(&ing)
		if stagedHasCommunity {
			if err := verifyPrefixInCommunityAnyIBGP(containers, p, community); err != nil {
				return err
			}
		} else {
			if err := verifyPrefixInCommunityEveryIBGP(containers, p, community, false); err != nil {
				return err
			}
		}
	}
	for _, c := range containers {
		for _, ing := range svcProd.Status.LoadBalancer.Ingress {
			p := jigservice.GetIngressPoint(&ing)
			if err := verifyLocalPrefOnIBGPSession(c, p, localPref); err != nil {
				return err
			}
		}
		for _, ing := range svcStaged.Status.LoadBalancer.Ingress {
			p := jigservice.GetIngressPoint(&ing)
			if err := verifyLocalPrefOnIBGPSession(c, p, localPref); err != nil {
				return err
			}
		}
	}
	return nil
}
