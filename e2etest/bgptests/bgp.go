/*
Copyright 2016 The Kubernetes Authors.
Copyright 2021 The MetalLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// https://github.com/kubernetes/kubernetes/blob/92aff21558831b829fbc8cbca3d52edc80c01aa3/test/e2e/network/loadbalancer.go#L878

package bgptests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	frrk8sv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"
	"go.universe.tf/e2etest/l2tests"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/status"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"

	"go.universe.tf/e2etest/pkg/frr"
	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	frrprovider "go.universe.tf/e2etest/pkg/frr/provider"
	"go.universe.tf/e2etest/pkg/ipfamily"
	testservice "go.universe.tf/e2etest/pkg/service"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

const (
	v4PoolAddresses       = "192.168.10.0/24"
	v6PoolAddresses       = "fc00:f853:0ccd:e799::/124"
	CommunityNoAdv        = "65535:65282" // 0xFFFFFF02: NO_ADVERTISE
	CommunityGracefulShut = "65535:0"     // GRACEFUL_SHUTDOWN
	SpeakerContainerName  = "speaker"

	GracefulRestartEnabled  = true
	GracefulRestartDisabled = false
)

var (
	ConfigUpdater       config.Updater
	FRRProvider         frrprovider.Provider
	Reporter            *k8sreporter.KubernetesReporter
	ReportPath          string
	PrometheusNamespace string
)

var _ = ginkgo.Describe("BGP", func() {
	var cs clientset.Interface
	emptyBGPAdvertisement := metallbv1beta1.BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}
	noAdvCommunity := metallbv1beta1.Community{
		ObjectMeta: metav1.ObjectMeta{Name: "community1"},
		Spec: metallbv1beta1.CommunitySpec{
			Communities: []metallbv1beta1.CommunityAlias{
				{
					Name:  "NO_ADVERTISE",
					Value: CommunityNoAdv,
				},
			},
		},
	}
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

	ginkgo.DescribeTable("A service of protocol load balancer should work with ETP=cluster", func(pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {

		_, svc := setupBGPService(cs, testNamespace, pairingIPFamily, poolAddresses, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
			tweak(svc)
		})
		defer testservice.Delete(cs, svc)

		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		testservice.ValidateDesiredLB(svc)

		for _, c := range FRRContainers {
			validateService(svc, allNodes.Items, c)
		}
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
			}),
		ginkgo.Entry("IPV4 - request IPv4 via custom annotation", ipfamily.IPv4, []string{v4PoolAddresses},
			func(svc *corev1.Service) {
				testservice.WithSpecificIPs(svc, "192.168.10.100")
			}),
		ginkgo.Entry("DUALSTACK - request Dual Stack via custom annotation", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
				testservice.WithSpecificIPs(svc, "192.168.10.100", "fc00:f853:ccd:e799::")
			}),
	)

	ginkgo.Describe("GracefulRestart, when speakers restart", func() {

		ginkgo.AfterEach(func() {
			for _, c := range FRRContainers {
				c.NeighborConfig.GracefulRestart = false
			}
		})

		assertDuringSpeakerRestart := func(gracefulRestart bool, pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
			_, svc := setupBGPService(cs, testNamespace, pairingIPFamily, poolAddresses,
				FRRContainers, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					tweak(svc)
				})
			defer testservice.Delete(cs, svc)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			testservice.ValidateDesiredLB(svc)

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}
			err = metallb.RestartSpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())

			if gracefulRestart == GracefulRestartDisabled {
				Eventually(func() error {
					for _, c := range FRRContainers {
						err := validateServiceNoWait(svc, allNodes.Items, c)
						if errors.Is(err, ErrStaleRoute) {
							Expect(err).NotTo(HaveOccurred(),
								"a stale route cannot be observed if GR disabled")
						}
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, time.Second).Should(HaveOccurred(), "a downtime should be observed")
				return
			}

			Eventually(func() error {
				for _, c := range FRRContainers {
					err := validateServiceNoWait(svc, allNodes.Items, c)
					if errors.Is(err, ErrStaleRoute) {
						continue // when GR, is normal to observe stale routes
					}
					Expect(err).NotTo(HaveOccurred(), "downtime was observed")
				}

				pods, err := metallb.SpeakerPods(cs)
				if err != nil {
					return err
				}

				for _, p := range pods {
					if !k8s.PodIsReady(p) {
						return fmt.Errorf("speaker pods are not ready")
					}
				}

				return nil
			}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred(), "no downtime until speakers are ready")

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}
		}

		ginkgo.Context("and when GR enabled", func() {

			assertDuringSpeakerRestartWithGR := func(pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
				assertDuringSpeakerRestart(GracefulRestartEnabled, pairingIPFamily, poolAddresses, tweak)
			}

			ginkgo.BeforeEach(func() {
				for _, c := range FRRContainers {
					c.NeighborConfig.GracefulRestart = true
				}
			})

			ginkgo.DescribeTable("dataplane should keep working", assertDuringSpeakerRestartWithGR,
				ginkgo.Entry("FRR-MODE IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
				ginkgo.Entry("FRR-MODE IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
				ginkgo.Entry("FRR-MODE DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
					func(svc *corev1.Service) { testservice.DualStack(svc) }),
			)
		})

		ginkgo.Context("when GR disabled", func() {
			assertDuringSpeakerRestartWithoutGR := func(pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
				assertDuringSpeakerRestart(GracefulRestartDisabled, pairingIPFamily, poolAddresses, tweak)
			}

			ginkgo.BeforeEach(func() {
				for _, c := range FRRContainers {
					c.NeighborConfig.GracefulRestart = false
				}
			})

			ginkgo.DescribeTable("dataplane should have a downtime", assertDuringSpeakerRestartWithoutGR,
				ginkgo.Entry("FRR-MODE IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
				ginkgo.Entry("FRR-MODE IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
				ginkgo.Entry("FRR-MODE DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
					func(svc *corev1.Service) { testservice.DualStack(svc) }),
			)
		})
	})

	ginkgo.Describe("Service with ETP=cluster", func() {
		ginkgo.It("IPV4 - should not be announced from a node with a NetworkUnavailable condition", func() {
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			nodeToSet := allNodes.Items[0].Name

			_, svc := setupBGPService(cs, testNamespace, ipfamily.IPv4, []string{v4PoolAddresses}, FRRContainers, func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
			})
			defer testservice.Delete(cs, svc)
			testservice.ValidateDesiredLB(svc)

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}

			err = k8s.SetNodeCondition(cs, nodeToSet, corev1.NodeNetworkUnavailable, corev1.ConditionTrue)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				err = k8s.SetNodeCondition(cs, nodeToSet, corev1.NodeNetworkUnavailable, corev1.ConditionFalse)
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("validating service is not announced from the unavailable node")
			for _, c := range FRRContainers {
				Eventually(func() error {
					return validateServiceNoWait(svc, []corev1.Node{allNodes.Items[0]}, c)
				}, time.Minute, time.Second).Should(HaveOccurred())
			}

			ginkgo.By("validating service is announced from the other available nodes")
			for _, c := range FRRContainers {
				Eventually(func() error {
					return validateServiceNoWait(svc, allNodes.Items[1:], c)
				}, time.Minute, time.Second).ShouldNot(HaveOccurred())
			}
		})
	})

	ginkgo.DescribeTable("A service of protocol load balancer should work with ETP=local", func(pairingIPFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {

		jig, svc := setupBGPService(cs, testNamespace, pairingIPFamily, poolAddresses, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyLocal(svc)
			tweak(svc)
		})
		defer testservice.Delete(cs, svc)

		testservice.ValidateDesiredLB(svc)

		err := jig.Scale(context.TODO(), 2)
		Expect(err).NotTo(HaveOccurred())

		epNodes, err := jig.ListNodesWithEndpoint(context.TODO()) // Only nodes with an endpoint should be advertising the IP
		Expect(err).NotTo(HaveOccurred())

		for _, c := range FRRContainers {
			validateService(svc, epNodes, c)
		}
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.DualStack(svc)
			}),
	)

	ginkgo.DescribeTable("FRR must be deployed when enabled", func(pairingIPFamily ipfamily.Family, poolAddresses []string) {

		_, svc := setupBGPService(cs, testNamespace, pairingIPFamily, poolAddresses, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
		})
		defer testservice.Delete(cs, svc)
		for _, c := range FRRContainers {
			frrIsPairedOnPods(cs, c, pairingIPFamily)
		}

	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}),
	)

	ginkgo.DescribeTable("A load balancer service should work with overlapping IPs", func(pairingIPFamily ipfamily.Family, poolAddresses []string) {
		var allNodes *corev1.NodeList
		resources := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-test",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: poolAddresses,
					},
				},
			},
			Peers:   metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
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

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		serviceIP, err := config.GetIPFromRangeByIndex(poolAddresses[0], 1)
		Expect(err).NotTo(HaveOccurred())

		svc, _ := testservice.CreateWithBackendPort(cs, testNamespace, "first-service",
			testservice.TestServicePort,
			func(svc *corev1.Service) {
				svc.Spec.LoadBalancerIP = serviceIP
				svc.Annotations = map[string]string{"metallb.io/allow-shared-ip": "foo"}
				svc.Spec.Ports[0].Port = int32(testservice.TestServicePort)
			})
		defer testservice.Delete(cs, svc)
		svc1, _ := testservice.CreateWithBackendPort(cs, testNamespace, "second-service",
			testservice.TestServicePort+1,
			func(svc *corev1.Service) {
				svc.Spec.LoadBalancerIP = serviceIP
				svc.Annotations = map[string]string{"metallb.io/allow-shared-ip": "foo"}
				svc.Spec.Ports[0].Port = int32(testservice.TestServicePort + 1)
			})
		defer testservice.Delete(cs, svc1)

		testservice.ValidateDesiredLB(svc)
		testservice.ValidateDesiredLB(svc1)

		for _, c := range FRRContainers {
			validateService(svc, allNodes.Items, c)
			validateService(svc1, allNodes.Items, c)
		}
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}),
	)

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.DescribeTable("set different AddressPools ranges modes", func(addressPools []metallbv1beta1.IPAddressPool, pairingFamily ipfamily.Family, tweak testservice.Tweak) {
			resources := config.Resources{
				Pools:   addressPools,
				Peers:   metallb.PeersForContainers(FRRContainers, pairingFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingFamily)
			}

			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			for _, i := range svc.Status.LoadBalancer.Ingress {
				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := jigservice.GetIngressPoint(&i)
				err = config.ValidateIPInRange(addressPools, ingressIP)
				Expect(err).NotTo(HaveOccurred())
			}

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}
		},
			ginkgo.Entry("IPV4 - test AddressPool defined by address range", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.0-192.168.10.18",
						},
					},
				}}, ipfamily.IPv4, testservice.TrafficPolicyCluster,
			),
			ginkgo.Entry("IPV4 - test AddressPool defined by network prefix", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.0/24",
						},
					},
				}}, ipfamily.IPv4, testservice.TrafficPolicyCluster,
			),
			ginkgo.Entry("IPV6 - test AddressPool defined by address range", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
						},
					},
				}}, ipfamily.IPv6, testservice.TrafficPolicyCluster,
			),
			ginkgo.Entry("IPV6 - test AddressPool defined by network prefix", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"fc00:f853:0ccd:e799::/124",
						},
					},
				}}, ipfamily.IPv6, testservice.TrafficPolicyCluster,
			),
			ginkgo.Entry("DUALSTACK - test AddressPool defined by address range", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.0-192.168.10.18",
							"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
						},
					},
				}}, ipfamily.DualStack, testservice.TrafficPolicyCluster,
			),
			ginkgo.Entry("DUALSTACK - test AddressPool defined by network prefix", []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							"192.168.10.0/24",
							"fc00:f853:0ccd:e799::/124",
						},
					},
				}}, ipfamily.DualStack, testservice.TrafficPolicyCluster,
			),
		)
	})
	ginkgo.DescribeTable("configure peers with routerid and validate external containers are paired with nodes", func(ipFamily ipfamily.Family) {
		ginkgo.By("configure peer")

		resources := config.Resources{
			Peers: metallb.WithRouterID(metallb.PeersForContainers(FRRContainers, ipFamily), "10.10.10.1"),
		}

		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		for _, c := range FRRContainers {
			err = frrcontainer.PairWithNodes(cs, c, ipFamily)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, c := range FRRContainers {
			validateFRRPeeredWithAllNodes(cs, c, ipFamily)
			neighbors, err := frr.NeighborsInfo(c)
			Expect(err).NotTo(HaveOccurred())
			for _, n := range neighbors {
				Expect(n.RemoteRouterID).To(Equal("10.10.10.1"))
			}
		}
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4),
		ginkgo.Entry("IPV6", ipfamily.IPv6))

	ginkgo.DescribeTable("FRR configure peers with GracefulRestart and validate external containers are paired with nodes", func(ipFamily ipfamily.Family) {
		ginkgo.By("configure peer")

		resources := config.Resources{
			Peers: metallb.WithGracefulRestart(metallb.PeersForContainers(FRRContainers, ipFamily)),
		}

		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		for _, c := range FRRContainers {
			err = frrcontainer.PairWithNodes(cs, c, ipFamily)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, c := range FRRContainers {
			validateFRRPeeredWithAllNodes(cs, c, ipFamily)
			neighbors, err := frr.NeighborsInfo(c)
			Expect(err).NotTo(HaveOccurred())
			for _, n := range neighbors {
				Expect(n.GRInfo.RemoteGrMode).To(Equal("Restart"))
			}
		}
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4),
		ginkgo.Entry("IPV6", ipfamily.IPv6))

	ginkgo.DescribeTable("validate external containers are paired with nodes", func(ipFamily ipfamily.Family) {
		ginkgo.By("configure peer")

		resources := config.Resources{
			Peers: metallb.PeersForContainers(FRRContainers, ipFamily, func(p *metallbv1beta2.BGPPeer) {
				p.Spec.PasswordSecret = corev1.SecretReference{Name: metallb.GetBGPPeerSecretName(p.Spec.ASN, p.Spec.Port, p.Spec.VRFName)}
				p.Spec.Password = ""
			}),
			PasswordSecrets: metallb.BGPPeerSecretReferences(FRRContainers),
		}
		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			for _, s := range resources.PasswordSecrets {
				err := cs.CoreV1().Secrets(metallb.Namespace).Delete(context.Background(), s.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}
		}()

		for _, c := range FRRContainers {
			err = frrcontainer.PairWithNodes(cs, c, ipFamily)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, c := range FRRContainers {
			validateFRRPeeredWithAllNodes(cs, c, ipFamily)
		}
	},
		ginkgo.Entry("IPV4 with Secret Ref set for BGPPeer CR", ipfamily.IPv4),
		ginkgo.Entry("IPV6 with Secret Ref set for BGPPeer CR", ipfamily.IPv6))

	ginkgo.DescribeTable("ServiceBGPStatus", func(ipFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
		validateStatusesFor := func(nodes []string, peers sets.Set[string], svc *corev1.Service, expectNoResources bool) error {
			for _, n := range nodes {
				s, err := status.BGPForServiceAndNode(ConfigUpdater.Client(), svc, n)
				if expectNoResources && !k8serrors.IsNotFound(err) {
					return fmt.Errorf("expected status for node %s to not be there, got %v with err %w", n, s, err)
				}
				if expectNoResources && k8serrors.IsNotFound(err) {
					continue
				}
				if err != nil {
					return err
				}
				statusPeers := sets.New(s.Status.Peers...)
				if !peers.Equal(statusPeers) {
					return fmt.Errorf("expected status peers to be %v, got %v for node %s\n diff: %s", peers, s.Status.Peers, n, cmp.Diff(sets.List(peers), s.Status.Peers))
				}
			}
			return nil
		}

		peers := metallb.PeersForContainers(FRRContainers, ipFamily)
		peersNames := sets.Set[string]{}
		for _, p := range peers {
			peersNames.Insert(p.Name)
		}

		bgpAdv := metallbv1beta1.BGPAdvertisement{
			ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: ConfigUpdater.Namespace()},
			Spec:       metallbv1beta1.BGPAdvertisementSpec{},
		}

		ginkgo.By("Creating the service advertised to all peers")
		resources := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-test",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: poolAddresses,
					},
				},
			},
			Peers:   peers,
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{bgpAdv},
		}

		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
			tweak(svc)
		})
		svcDeleted := false
		defer func() {
			if !svcDeleted {
				testservice.Delete(cs, svc)
			}
		}()

		for _, i := range svc.Status.LoadBalancer.Ingress {
			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			ingressIP := jigservice.GetIngressPoint(&i)
			err = config.ValidateIPInRange(resources.Pools, ingressIP)
			Expect(err).NotTo(HaveOccurred())
		}

		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		nodesNames := []string{}
		for _, n := range nodes.Items {
			nodesNames = append(nodesNames, n.Name)
		}

		ginkgo.By("Verifying all nodes create a status for the service")
		Eventually(func() error {
			return validateStatusesFor(nodesNames, peersNames, svc, false)
		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

		ginkgo.By("Adding a dummy peer to the adv")
		err = ConfigUpdater.Client().Get(context.TODO(), types.NamespacedName{Namespace: bgpAdv.Namespace, Name: bgpAdv.Name}, &bgpAdv)
		Expect(err).ToNot(HaveOccurred())
		bgpAdv.Spec.Peers = append(sets.List(peersNames), "dummy")
		err = ConfigUpdater.Client().Update(context.TODO(), &bgpAdv)
		Expect(err).ToNot(HaveOccurred())

		Consistently(func() error {
			return validateStatusesFor(nodesNames, peersNames, svc, false)
		}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred(), "expected status peers to be the same as before after adding a dummy peer")

		ginkgo.By("Removing the first peer")
		peer0 := peers[0]
		peer0.Namespace = ConfigUpdater.Namespace()
		err = ConfigUpdater.Client().Delete(context.TODO(), &peer0)
		Expect(err).ToNot(HaveOccurred())
		peersNames.Delete(peer0.Name)
		Eventually(func() error {
			return validateStatusesFor(nodesNames, peersNames, svc, peersNames.Len() == 0)
		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

		ginkgo.By("Updating the node selector of the adv to not include the first node")
		err = ConfigUpdater.Client().Get(context.TODO(), types.NamespacedName{Namespace: bgpAdv.Namespace, Name: bgpAdv.Name}, &bgpAdv)
		Expect(err).ToNot(HaveOccurred())
		bgpAdv.Spec.NodeSelectors = []metav1.LabelSelector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Operator: "In",
						Key:      "kubernetes.io/hostname",
						Values:   nodesNames[1:],
					},
				}},
		}
		err = ConfigUpdater.Client().Update(context.TODO(), &bgpAdv)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			return validateStatusesFor([]string{nodesNames[0]}, sets.Set[string]{}, svc, true)
		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
		Eventually(func() error {
			return validateStatusesFor(nodesNames[1:], peersNames, svc, peersNames.Len() == 0)
		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())

		ginkgo.By("Validating the the statuses are deleted after deleting the service")
		testservice.Delete(cs, svc)
		svcDeleted = true
		Eventually(func() error {
			return validateStatusesFor(nodesNames, sets.Set[string]{}, svc, true)
		}, 2*time.Minute, 2*time.Second).ShouldNot(HaveOccurred())
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{v4PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{v6PoolAddresses}, func(_ *corev1.Service) {}),
		ginkgo.Entry("DUALSTACK", ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}, testservice.DualStack))

	ginkgo.Context("BFD", func() {
		ginkgo.DescribeTable("should work with the given bfd profile", func(bfd metallbv1beta1.BFDProfile, pairingFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bfd-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: poolAddresses,
						},
					},
				},
				Peers:       metallb.WithBFD(metallb.PeersForContainers(FRRContainers, pairingFamily), bfd.Name),
				BGPAdvs:     []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				BFDProfiles: []metallbv1beta1.BFDProfile{bfd},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = true
				})
				Expect(err).NotTo(HaveOccurred())
			}

			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingFamily)
			}

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}

			Eventually(func() error {
				for _, c := range FRRContainers {
					bfdPeers, err := frr.BFDPeers(c.Executor)
					if err != nil {
						return err
					}
					err = frr.BFDPeersMatchNodes(allNodes.Items, bfdPeers, pairingFamily, c.RouterConfig.VRF)
					if err != nil {
						return err
					}
					for _, peerConfig := range bfdPeers {
						toCompare := config.BFDProfileWithDefaults(bfd, peerConfig.Multihop)
						ginkgo.By(fmt.Sprintf("Checking bfd parameters on %s", peerConfig.Peer))
						err := checkBFDConfigPropagated(toCompare, peerConfig)
						if err != nil {
							return err
						}
					}
				}
				return nil
			}, 4*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("checking the sessions don't flap when changing the configuration")

			previousNeighbors := map[string]frr.NeighborsMap{}
			for _, c := range FRRContainers {
				neighbors, err := frr.NeighborsInfo(c)
				Expect(err).NotTo(HaveOccurred())
				previousNeighbors[c.Name] = neighbors
			}
			ginkgo.By("creating another the service")
			svc1, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb1", tweak)
			defer testservice.Delete(cs, svc1)

			Consistently(func() error {
				for _, c := range FRRContainers {
					neighbors, err := frr.NeighborsInfo(c)
					Expect(err).NotTo(HaveOccurred())
					Expect(neighbors).To(HaveLen(len(previousNeighbors[c.Name])))

					for _, n := range neighbors {
						previousDropped := previousNeighbors[c.Name][n.ID].ConnectionsDropped
						if n.ConnectionsDropped > previousDropped {
							return fmt.Errorf("increased connections dropped from %s to %s, previous: %d current %d", c.Name, n.ID, previousDropped, n.ConnectionsDropped)
						}
					}
				}
				return nil
			}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		},
			ginkgo.Entry("IPV4 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			ginkgo.Entry("IPV4 - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  ptr.To(uint32(60)),
						TransmitInterval: ptr.To(uint32(61)),
						EchoInterval:     ptr.To(uint32(62)),
						EchoMode:         ptr.To(false),
						PassiveMode:      ptr.To(false),
						MinimumTTL:       ptr.To(uint32(254)),
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			ginkgo.Entry("IPV4 - echo mode enabled",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "echo",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  ptr.To(uint32(80)),
						TransmitInterval: ptr.To(uint32(81)),
						EchoInterval:     ptr.To(uint32(82)),
						EchoMode:         ptr.To(true),
						PassiveMode:      ptr.To(false),
						MinimumTTL:       ptr.To(uint32(254)),
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			ginkgo.Entry("IPV6 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			ginkgo.Entry("IPV6 - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  ptr.To(uint32(60)),
						TransmitInterval: ptr.To(uint32(61)),
						EchoInterval:     ptr.To(uint32(62)),
						EchoMode:         ptr.To(false),
						PassiveMode:      ptr.To(false),
						MinimumTTL:       ptr.To(uint32(254)),
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			ginkgo.Entry("DUALSTACK - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  ptr.To(uint32(60)),
						TransmitInterval: ptr.To(uint32(61)),
						EchoInterval:     ptr.To(uint32(62)),
						EchoMode:         ptr.To(false),
						PassiveMode:      ptr.To(false),
						MinimumTTL:       ptr.To(uint32(254)),
					},
				}, ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
				}),
		)
	})

	ginkgo.Context("validate configuration changes", func() {
		ginkgo.DescribeTable("should work after subsequent configuration updates", func(addressRange string, ipFamily ipfamily.Family) {
			var services []*corev1.Service
			var servicesIngressIP []string
			var pools []metallbv1beta1.IPAddressPool

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 2; i++ {
				ginkgo.By(fmt.Sprintf("configure addresspool number %d", i+1))
				firstIP, err := config.GetIPFromRangeByIndex(addressRange, i*10+1)
				Expect(err).NotTo(HaveOccurred())
				lastIP, err := config.GetIPFromRangeByIndex(addressRange, i*10+10)
				Expect(err).NotTo(HaveOccurred())
				addressesRange := fmt.Sprintf("%s-%s", firstIP, lastIP)
				pool := metallbv1beta1.IPAddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("test-addresspool%d", i+1),
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{addressesRange},
					},
				}
				pools = append(pools, pool)

				resources := config.Resources{
					Pools:   pools,
					Peers:   metallb.PeersForContainers(FRRContainers, ipFamily),
					BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				}

				for _, c := range FRRContainers {
					err := frrcontainer.PairWithNodes(cs, c, ipFamily)
					Expect(err).NotTo(HaveOccurred())
				}

				err = ConfigUpdater.Update(resources)
				Expect(err).NotTo(HaveOccurred())

				for _, c := range FRRContainers {
					validateFRRPeeredWithAllNodes(cs, c, ipFamily)
				}

				ginkgo.By(fmt.Sprintf("configure service number %d", i+1))
				svc, _ := testservice.CreateWithBackend(cs, testNamespace, fmt.Sprintf("svc%d", i+1), testservice.TrafficPolicyCluster, func(svc *corev1.Service) {
					svc.Annotations = map[string]string{"metallb.io/address-pool": fmt.Sprintf("test-addresspool%d", i+1)}
				})
				defer testservice.Delete(cs, svc)

				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := jigservice.GetIngressPoint(
					&svc.Status.LoadBalancer.Ingress[0])
				err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool}, ingressIP)
				Expect(err).NotTo(HaveOccurred())

				services = append(services, svc)
				servicesIngressIP = append(servicesIngressIP, ingressIP)

				for j := 0; j <= i; j++ {
					ginkgo.By(fmt.Sprintf("validate service %d IP didn't change", j+1))
					ip := jigservice.GetIngressPoint(&services[j].Status.LoadBalancer.Ingress[0])
					Expect(ip).To(Equal(servicesIngressIP[j]))

					ginkgo.By(fmt.Sprintf("checking connectivity of service %d to its external VIP", j+1))
					for _, c := range FRRContainers {
						validateService(svc, allNodes.Items, c)
					}
				}
			}
		},
			ginkgo.Entry("IPV4", "192.168.10.0/24", ipfamily.IPv4),
			ginkgo.Entry("IPV6", "fc00:f853:0ccd:e799::/116", ipfamily.IPv6))

		ginkgo.DescribeTable("configure peers one by one and validate FRR paired with nodes", func(ipFamily ipfamily.Family) {
			for i, c := range FRRContainers {
				ginkgo.By(fmt.Sprintf("configure FRR peer [%s]", c.Name))

				resources := config.Resources{
					Peers:   metallb.PeersForContainers([]*frrcontainer.FRR{c}, ipFamily),
					BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				}
				err := ConfigUpdater.Update(resources)
				Expect(err).NotTo(HaveOccurred())

				err = frrcontainer.PairWithNodes(cs, c, ipFamily)
				Expect(err).NotTo(HaveOccurred())

				validateFRRPeeredWithAllNodes(cs, FRRContainers[i], ipFamily)
			}
		},
			ginkgo.Entry("IPV4", ipfamily.IPv4),
			ginkgo.Entry("IPV6", ipfamily.IPv6))

		ginkgo.DescribeTable("configure bgp advertisement and verify it gets propagated",
			func(rangeWithAdvertisement string, rangeWithoutAdvertisement string, advertisement metallbv1beta1.BGPAdvertisement,
				ipFamily ipfamily.Family, communities []metallbv1beta1.Community) {
				emptyAdvertisement := metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: "empty",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPAddressPools: []string{"bgp-with-no-advertisement"},
					},
				}

				poolWithAdvertisement := metallbv1beta1.IPAddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "bgp-with-advertisement",
						Labels: map[string]string{"test": "bgp-with-advertisement"},
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{rangeWithAdvertisement},
					},
				}
				poolWithoutAdvertisement := metallbv1beta1.IPAddressPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-with-no-advertisement",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{rangeWithoutAdvertisement},
					},
				}

				resources := config.Resources{
					Peers:       metallb.PeersForContainers(FRRContainers, ipFamily),
					Communities: communities,
				}

				resources.Pools = []metallbv1beta1.IPAddressPool{poolWithAdvertisement, poolWithoutAdvertisement}
				resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{emptyAdvertisement, advertisement}

				for _, c := range FRRContainers {
					err := frrcontainer.PairWithNodes(cs, c, ipFamily)
					Expect(err).NotTo(HaveOccurred())
				}

				err := ConfigUpdater.Update(resources)
				Expect(err).NotTo(HaveOccurred())

				for _, c := range FRRContainers {
					validateFRRPeeredWithAllNodes(cs, c, ipFamily)
				}

				ipWithAdvertisement, err := config.GetIPFromRangeByIndex(rangeWithAdvertisement, 0)
				Expect(err).NotTo(HaveOccurred())
				ipWithAdvertisement1, err := config.GetIPFromRangeByIndex(rangeWithAdvertisement, 1)
				Expect(err).NotTo(HaveOccurred())
				ipNoAdvertisement, err := config.GetIPFromRangeByIndex(rangeWithoutAdvertisement, 0)
				Expect(err).NotTo(HaveOccurred())

				svcAdvertisement, _ := testservice.CreateWithBackend(cs, testNamespace, "service-with-adv",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipWithAdvertisement
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcAdvertisement)
				svcAdvertisement1, _ := testservice.CreateWithBackend(cs, testNamespace, "service-with-adv1",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipWithAdvertisement1
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcAdvertisement1)
				svcNoAdvertisement, _ := testservice.CreateWithBackend(cs, testNamespace, "service-no-adv",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipNoAdvertisement
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcNoAdvertisement)

				allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				for _, c := range FRRContainers {
					validateService(svcAdvertisement, allNodes.Items, c)
					validateService(svcAdvertisement1, allNodes.Items, c)
					validateService(svcNoAdvertisement, allNodes.Items, c)
					Eventually(func() error {
						for _, community := range advertisement.Spec.Communities {
							// Get community value for test cases with Community CRD.
							communityValue, err := communityForAlias(community, communities)
							if err != nil {
								communityValue = community
							}
							routes, err := frr.RoutesForCommunity(c, communityValue, ipFamily)
							if err != nil {
								return err
							}
							if _, ok := routes[ipNoAdvertisement]; ok {
								return fmt.Errorf("found %s route for community %s", ipNoAdvertisement, community)
							}
							if _, ok := routes[ipWithAdvertisement1]; !ok {
								return fmt.Errorf("%s route not found for community %s", ipWithAdvertisement1, community)
							}
							if _, ok := routes[ipWithAdvertisement]; !ok {
								return fmt.Errorf("%s route not found for community %s", ipWithAdvertisement, community)
							}
						}
						// LocalPref check is only valid for iBGP sessions
						if advertisement.Spec.LocalPref != 0 && strings.Contains(c.Name, "ibgp") {
							localPrefix, err := frr.LocalPrefForPrefix(c, ipWithAdvertisement, ipFamily)
							if err != nil {
								return err
							}
							if localPrefix != advertisement.Spec.LocalPref {
								return fmt.Errorf("%s %s not matching local pref", c.Name, ipWithAdvertisement)
							}
							localPrefix, err = frr.LocalPrefForPrefix(c, ipWithAdvertisement1, ipFamily)
							if err != nil {
								return err
							}
							if localPrefix != advertisement.Spec.LocalPref {
								return fmt.Errorf("%s %s not matching local pref", c.Name, ipWithAdvertisement1)
							}
							localPrefix, err = frr.LocalPrefForPrefix(c, ipNoAdvertisement, ipFamily)
							if err != nil {
								return err
							}
							if localPrefix == advertisement.Spec.LocalPref {
								return fmt.Errorf("%s %s matching local pref", c.Name, ipNoAdvertisement)
							}

						}
						return nil
					}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
				}

			},
			ginkgo.Entry("IPV4 - community and localpref",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{CommunityNoAdv},
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("FRR - IPV4 - large community and localpref",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{"large:123:456:7890"},
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV4 - localpref",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV4 - community",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{CommunityNoAdv},
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV4 - community from CRD",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{"NO_ADVERTISE"},
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{noAdvCommunity}),
			ginkgo.Entry("IPV4 - ip pool selector",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities: []string{CommunityNoAdv},
						LocalPref:   50,
						IPAddressPoolSelectors: []metav1.LabelSelector{
							{
								MatchLabels: map[string]string{
									"test": "bgp-with-advertisement",
								},
							},
						},
					},
				},
				ipfamily.IPv4,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV6 - community and localpref",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref:      50,
						Communities:    []string{CommunityNoAdv},
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv6,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV6 - community",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{CommunityNoAdv},
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv6,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("IPV6 - community from CRD",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{"NO_ADVERTISE"},
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv6,
				[]metallbv1beta1.Community{noAdvCommunity}),
			ginkgo.Entry("IPV6 - localpref",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv6,
				[]metallbv1beta1.Community{}),
			ginkgo.Entry("FRR - IPV6 - large community and localpref",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities:    []string{"large:123:456:7890"},
						LocalPref:      50,
						IPAddressPools: []string{"bgp-with-advertisement"},
					},
				},
				ipfamily.IPv6,
				[]metallbv1beta1.Community{}))
	})

	ginkgo.Context("MetalLB FRR rejects", func() {
		ginkgo.DescribeTable("any routes advertised by any neighbor", func(addressesRange, toInject string, pairingIPFamily ipfamily.Family) {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rejectroutes",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								addressesRange,
							},
						},
					},
				},
				Peers:   metallb.PeersForContainers(FRRContainers, pairingIPFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily, func(frr *frrcontainer.FRR) {
					if pairingIPFamily == ipfamily.IPv4 {
						frr.NeighborConfig.ToAdvertiseV4 = []string{toInject}
					} else {
						frr.NeighborConfig.ToAdvertiseV6 = []string{toInject}
					}
				})
				Expect(err).NotTo(HaveOccurred())
			}

			speakerPods, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())
			checkRoute := func() error {
				isRouteInjected, where := isRouteInjected(speakerPods, pairingIPFamily, toInject, "all")
				if isRouteInjected {
					return fmt.Errorf("route %s injected in %s", toInject, where)
				}
				return nil
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			Consistently(checkRoute, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb")
			defer testservice.Delete(cs, svc)

			Consistently(checkRoute, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		},
			ginkgo.Entry("IPV4", "192.168.10.0/24", "172.16.1.1/32", ipfamily.IPv4),
			ginkgo.Entry("IPV6", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::1/128", ipfamily.IPv6),
		)
	})

	ginkgo.Context("MetalLB allows adding extra FRR configuration", func() {
		type toApply string
		var configmap toApply = "configmap"
		var frrconfiguration toApply = "frrconfiguration"
		type whenApply string
		var before whenApply = "before"
		var after whenApply = "after"
		ginkgo.AfterEach(func() {
			err := k8s.RemoveConfigmap(cs, "bgpextras", metallb.Namespace)
			Expect(err).NotTo(HaveOccurred())
		})
		ginkgo.DescribeTable("to accept any routes advertised by any neighbor", func(addressesRange, toInject string, pairingIPFamily ipfamily.Family, what toApply, when whenApply) {
			resources := config.Resources{
				Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			}

			toFilter := "172.16.2.1/32"
			if pairingIPFamily == ipfamily.IPv6 {
				toFilter = "fc00:f853:ccd:e801::2/128"
			}

			for i, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily, func(frr *frrcontainer.FRR) {
					// We advertise a different route for each different container, to ensure all
					// of them are able to advertise regardless of the configuration
					if pairingIPFamily == ipfamily.IPv4 {
						frr.NeighborConfig.ToAdvertiseV4 = []string{fmt.Sprintf(toInject, i+1), toFilter}
					} else {
						frr.NeighborConfig.ToAdvertiseV6 = []string{fmt.Sprintf(toInject, i+1), toFilter}
					}
				})
				Expect(err).NotTo(HaveOccurred())
			}

			speakerPods, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())
			checkRoutesAreInjected := func() error {
				for i, c := range FRRContainers {
					injected, _ := isRouteInjected(speakerPods, pairingIPFamily, fmt.Sprintf(toInject, i+1), c.RouterConfig.VRF)
					if !injected {
						return fmt.Errorf("route not injected from %s", c.Name)
					}
					injected, podName := isRouteInjected(speakerPods, pairingIPFamily, toFilter, c.RouterConfig.VRF)
					if injected {
						return fmt.Errorf("failed to filter route injected from %s to %s", c.Name, podName)
					}
				}
				return nil
			}

			applyConfigMap := func() {
				data := ""
				data += "ip prefix-list allowed permit 172.16.1.0/24 le 32\n"
				data += "ipv6 prefix-list allowed permit fc00:f853:ccd:e800::/64 le 128\n"
				for _, c := range FRRContainers {
					ip := c.Ipv4
					if pairingIPFamily == ipfamily.IPv6 {
						ip = c.Ipv6
					}
					ruleName := ip
					if c.RouterConfig.VRF != "" {
						ruleName = fmt.Sprintf("%s-%s", ip, c.RouterConfig.VRF)
					}
					data += fmt.Sprintf("route-map %s-in permit 20\n", ruleName)
					if pairingIPFamily == ipfamily.IPv4 {
						data += "  match ip address prefix-list allowed\n"
					} else {
						data += "  match ipv6 address prefix-list allowed\n"
					}
				}
				extraData := map[string]string{
					"extras": data,
				}

				err = k8s.CreateConfigmap(cs, "bgpextras", metallb.Namespace, extraData)
				Expect(err).NotTo(HaveOccurred())
			}

			applyFRRConfiguration := func() {
				config := frrk8sv1beta1.FRRConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "receiveroutes",
						Namespace: testNamespace,
					},
					Spec: frrk8sv1beta1.FRRConfigurationSpec{
						BGP: frrk8sv1beta1.BGPConfig{
							Routers: []frrk8sv1beta1.Router{},
						},
					},
				}

				routers := map[string]frrk8sv1beta1.Router{}
				for _, p := range resources.Peers {
					p := p
					r := routers[p.Spec.VRFName]
					r.ASN = p.Spec.MyASN
					r.VRF = p.Spec.VRFName

					keepAliveTime := p.Spec.KeepaliveTime
					if keepAliveTime == nil && p.Spec.HoldTime != nil {
						keepAliveTime = &metav1.Duration{Duration: p.Spec.HoldTime.Duration / 3}
					}
					r.Neighbors = append(r.Neighbors, frrk8sv1beta1.Neighbor{
						ASN:           p.Spec.ASN,
						Address:       p.Spec.Address,
						Password:      p.Spec.Password,
						Port:          &p.Spec.Port,
						HoldTime:      p.Spec.HoldTime,
						KeepaliveTime: keepAliveTime,
						EBGPMultiHop:  p.Spec.EBGPMultiHop,
						BFDProfile:    p.Spec.BFDProfile,
						ToReceive: frrk8sv1beta1.Receive{
							Allowed: frrk8sv1beta1.AllowedInPrefixes{
								Mode: frrk8sv1beta1.AllowRestricted,
								Prefixes: []frrk8sv1beta1.PrefixSelector{
									{
										Prefix: "172.16.1.0/24",
										LE:     32,
									},
									{
										Prefix: "fc00:f853:ccd:e800::/64",
										LE:     128,
									},
								},
							},
						},
					})
					routers[p.Spec.VRFName] = r
				}

				for _, router := range routers {
					config.Spec.BGP.Routers = append(config.Spec.BGP.Routers, router)
				}

				err := ConfigUpdater.Client().Create(context.Background(), &config)
				Expect(err).NotTo(HaveOccurred())
			}

			apply := applyConfigMap
			if what == frrconfiguration {
				apply = applyFRRConfiguration
			}

			if when == before {
				ginkgo.By("Applying the config that allows incoming routes")
				apply()
			}

			ginkgo.By("Applying the FRR configuration")
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, pairingIPFamily)
			}

			if when == after {
				ginkgo.By("Applying the config that allows incoming routes")
				apply()
			}
			Eventually(checkRoutesAreInjected, time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			_, svc := setupBGPService(cs, testNamespace, pairingIPFamily, []string{addressesRange}, FRRContainers, func(svc *corev1.Service) {})
			defer testservice.Delete(cs, svc)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			testservice.ValidateDesiredLB(svc)

			for _, container := range FRRContainers {
				ginkgo.By(fmt.Sprintf("validating the service from %s", container.Name))
				validateService(svc, allNodes.Items, container)
			}
		},
			ginkgo.Entry("FRR-MODE IPV4 - before config", "192.168.10.0/24", "172.16.1.%d/32", ipfamily.IPv4, configmap, before),
			ginkgo.Entry("FRR-MODE IPV6 - before config", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::%d/128", ipfamily.IPv6, configmap, before),
			ginkgo.Entry("FRR-MODE IPV4 - after config", "192.168.10.0/24", "172.16.1.%d/32", ipfamily.IPv4, configmap, after),
			ginkgo.Entry("FRR-MODE IPV6 - after config", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::%d/128", ipfamily.IPv6, configmap, after),
			ginkgo.Entry("FRRK8S-MODE IPV4 - before config", "192.168.10.0/24", "172.16.1.%d/32", ipfamily.IPv4, frrconfiguration, before),
			ginkgo.Entry("FRRK8S-MODE IPV6 - before config", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::%d/128", ipfamily.IPv6, frrconfiguration, before),
			ginkgo.Entry("FRRK8S-MODE IPV4 - after config", "192.168.10.0/24", "172.16.1.%d/32", ipfamily.IPv4, frrconfiguration, after),
			ginkgo.Entry("FRRK8S-MODE IPV6 - after config", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::%d/128", ipfamily.IPv6, frrconfiguration, after),
		)
	})

	ginkgo.Context("FRR-MODE FRR validate reload feedback", func() {
		ginkgo.It("should update MetalLB config and log reload-validate success", func() {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "new-config",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								v4PoolAddresses,
							},
						},
					},
				},
				Peers:   metallb.PeersForContainers(FRRContainers, ipfamily.IPv4),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			beforeUpdateTime := metav1.Now()

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			speakerPods, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range speakerPods {
				Eventually(func() string {
					logs, err := k8s.PodLogsSinceTime(cs, pod, SpeakerContainerName, &beforeUpdateTime)
					Expect(err).NotTo(HaveOccurred())

					return logs
				}, 2*time.Minute, 1*time.Second).Should(
					And(
						ContainSubstring("reload-validate"),
						ContainSubstring("success"),
					),
				)
			}
		})
	})

	ginkgo.Context("validate FRR running configuration", func() {
		ginkgo.It("Full BFD profile", func() {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{v4PoolAddresses},
						},
					},
				},
				Peers:   metallb.WithBFD(metallb.PeersForContainers(FRRContainers, ipfamily.IPv4), "fullbfdprofile1"),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				BFDProfiles: []metallbv1beta1.BFDProfile{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "fullbfdprofile1"},
						Spec: metallbv1beta1.BFDProfileSpec{
							ReceiveInterval:  ptr.To(uint32(93)),
							TransmitInterval: ptr.To(uint32(95)),
							EchoInterval:     ptr.To(uint32(97)),
							EchoMode:         ptr.To(true),
							PassiveMode:      ptr.To(true),
							MinimumTTL:       ptr.To(uint32(253)),
						},
					},
				},
			}

			resources.Peers = append(resources.Peers, metallbv1beta2.BGPPeer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "defaultport",
				},
				Spec: metallbv1beta2.BGPPeerSpec{
					ASN:     metalLBASN,
					MyASN:   metalLBASN,
					Address: "192.168.1.1",
				},
			})

			for i := range resources.Peers {
				resources.Peers[i].Spec.KeepaliveTime = &metav1.Duration{Duration: 13 * time.Second}
				resources.Peers[i].Spec.HoldTime = &metav1.Duration{Duration: 57 * time.Second}
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			speakerPods, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range speakerPods {
				podExecutor, err := FRRProvider.FRRExecutorFor(pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() string {
					// We need to assert against the output of the command as a bare string, as
					// there is no json version of the command.
					cfgStr, err := podExecutor.Exec("vtysh", "-c", "show running-config")
					if err != nil {
						return err.Error()
					}

					return cfgStr
				}, 1*time.Minute).Should(
					And(
						ContainSubstring("log file /etc/frr/frr.log"),
						WithTransform(substringCount("\n profile fullbfdprofile1"), Equal(1)),
						ContainSubstring("receive-interval 93"),
						ContainSubstring("transmit-interval 95"),
						MatchRegexp("echo.*interval 97"), // TODO: this is backward compatible to 7.5, let's remove it when we consolidate the frr version
						ContainSubstring("minimum-ttl 253"),
						ContainSubstring("passive-mode"),
						ContainSubstring("echo-mode"),
						ContainSubstring("timers 13 57"),
					),
				)
			}

			ginkgo.By("Checking the default value on the bgppeer crds is set")
			peer := metallbv1beta2.BGPPeer{}
			err = ConfigUpdater.Client().Get(context.Background(), types.NamespacedName{Name: "defaultport", Namespace: metallb.Namespace}, &peer)
			Expect(err).NotTo(HaveOccurred())
			Expect(peer.Spec.Port).To(Equal(uint16(179)))
		})
		ginkgo.It("BGP Peer parameters", func() {
			connectTime := time.Second * 5
			resources := config.Resources{
				Peers: metallb.PeersForContainers(FRRContainers, ipfamily.IPv4, func(p *metallbv1beta2.BGPPeer) {
					p.Spec.ConnectTime = ptr.To(metav1.Duration{Duration: connectTime})
					p.Spec.DisableMP = true
				}),
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			speakerPods, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())

			for _, pod := range speakerPods {
				podExec, err := FRRProvider.FRRExecutorFor(pod.Namespace, pod.Name)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() error {
					neighbors, err := frr.NeighborsInfo(podExec)
					if err != nil {
						return err
					}
					if len(neighbors) == 0 {
						return fmt.Errorf("expected at least 1 neighbor, got %d", len(neighbors))
					}
					for _, neighbor := range neighbors {
						if neighbor.ConfiguredConnectTime != int(connectTime.Seconds()) {
							return fmt.Errorf("expected connect time to be %d, got %d", int(connectTime.Seconds()), neighbor.ConfiguredConnectTime)
						}

						neighborFamily := ipfamily.ForAddress(net.ParseIP(neighbor.ID))
						for _, family := range neighbor.AddressFamilies {
							if !strings.Contains(family, string(neighborFamily)) {
								return fmt.Errorf("expected %s neigbour to contain only %s families but contains %s", neighbor.ID, neighborFamily, family)
							}
						}
					}
					return nil
				}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred())
			}

		})
	})
	ginkgo.DescribeTable("A service of protocol load balancer should work with two protocols", func(pairingIPFamily ipfamily.Family, poolAddresses []string) {
		_, svc := setupBGPService(cs, testNamespace, pairingIPFamily, poolAddresses, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
		})
		defer testservice.Delete(cs, svc)

		allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Checking the service is reacheable via BGP")
		for _, c := range FRRContainers {
			validateService(svc, allNodes.Items, c)
		}

		checkServiceL2 := func() error {
			for _, ip := range svc.Status.LoadBalancer.Ingress {
				ingressIP := jigservice.GetIngressPoint(&ip)
				err := mac.RequestAddressResolution(ingressIP, executor.Host)
				if err != nil {
					return err
				}
			}
			return nil
		}

		ginkgo.By("Checking the service is not reacheable via L2")
		Consistently(checkServiceL2, 3*time.Second, 1*time.Second).Should(HaveOccurred())

		ginkgo.By("Creating the l2 advertisement")
		l2Advertisement := metallbv1beta1.L2Advertisement{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "l2adv",
				Namespace: metallb.Namespace,
			},
		}

		err = ConfigUpdater.Client().Create(context.Background(), &l2Advertisement)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Checking the service is reacheable via L2")
		Eventually(func() error {
			return testservice.ValidateL2(svc)
		}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

		ginkgo.By("Checking the service is still reacheable via BGP")
		for _, c := range FRRContainers {
			validateService(svc, allNodes.Items, c)
		}

		ginkgo.By("Deleting the l2 advertisement")
		err = ConfigUpdater.Client().Delete(context.Background(), &l2Advertisement)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Checking the service is not reacheable via L2 anymore")
		// We use arping here, because the client's cache may still be filled with the mac and the ip of the
		// destination
		Eventually(checkServiceL2, 5*time.Second, 1*time.Second).Should(HaveOccurred())
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4, []string{l2tests.IPV4ServiceRange}),
		ginkgo.Entry("IPV6", ipfamily.IPv6, []string{l2tests.IPV6ServiceRange}),
	)
	ginkgo.DescribeTable("FRR establishes connections with dynamic ASN ", func(pairingIPFamily ipfamily.Family) {
		resources := config.Resources{
			Peers: metallb.PeersForContainers(FRRContainers, pairingIPFamily, func(p *metallbv1beta2.BGPPeer) {
				dynamicASN := metallbv1beta2.InternalASNMode
				if p.Spec.ASN != p.Spec.MyASN {
					dynamicASN = metallbv1beta2.ExternalASNMode
				}
				p.Spec.ASN = 0
				p.Spec.DynamicASN = dynamicASN
			}),
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
	},
		ginkgo.Entry("IPV4", ipfamily.IPv4),
		ginkgo.Entry("IPV6", ipfamily.IPv6),
	)
})

// substringCount creates a Gomega transform function that
// counts the number of occurrences in the subject string.
func substringCount(substr string) interface{} {
	return func(action string) int {
		return strings.Count(action, substr)
	}
}

// communityForAlias checks if the given community name exists in the community crs,
// and if so, returns the value of the matching community.
func communityForAlias(communityName string, cs []metallbv1beta1.Community) (string, error) {
	for _, c := range cs {
		for _, communityAlias := range c.Spec.Communities {
			if communityName == communityAlias.Name {
				return communityAlias.Value, nil
			}
		}
	}
	return "", fmt.Errorf("community name %s not found in Community CRs", communityName)
}
