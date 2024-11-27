// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/e2etest/pkg/frr/container"
	"go.universe.tf/e2etest/pkg/ipfamily"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/metrics"
	testservice "go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

var _ = ginkgo.Describe("BGP metrics", func() {
	var cs clientset.Interface

	emptyBGPAdvertisement := metallbv1beta1.BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
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
	})

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods []*corev1.Pod
		var promPod *corev1.Pod

		ginkgo.BeforeEach(func() {
			var err error
			controllerPod, err = metallb.ControllerPod(cs)
			Expect(err).NotTo(HaveOccurred())
			speakerPods, err = metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())
			promPod, err = metrics.PrometheusPod(cs, PrometheusNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.DescribeTable("should collect BGP metrics in FRR mode", func(ipFamily ipfamily.Family, poolAddress string, addressTotal int) {
			poolName := "bgp-test"

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
				Peers:   metallb.PeersForContainers(FRRContainers, ipFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, ipFamily)
			}

			ginkgo.By("creating a service")
			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", testservice.TrafficPolicyCluster) // Is a sleep required here?
			defer testservice.Delete(cs, svc)

			selectors := labelsForPeers(FRRContainers, ipFamily)

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))
				Eventually(func() error {
					metricsPod, metricsPrefix, err := FRRProvider.BGPMetricsPodFor(speaker.Namespace, speaker.Name)
					if err != nil {
						return err
					}

					speakerMetrics, err := metrics.ForPod(promPod, metricsPod, metallb.Namespace)
					if err != nil {
						return err
					}
					for _, selector := range selectors {
						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_opens_sent", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_opens_sent{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_opens_received", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_opens_received{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bgp_updates_total_received", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_updates_total_received{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_keepalives_sent", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_keepalives_sent{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_keepalives_received", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_keepalives_received{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_route_refresh_sent", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_route_refresh_sent{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_total_sent", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_total_sent{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bgp_total_received", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_total_received{%s} >= 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			}
		},
			ginkgo.Entry("IPV4 - Checking service", ipfamily.IPv4, v4PoolAddresses, 256),
			ginkgo.Entry("IPV6 - Checking service", ipfamily.IPv6, v6PoolAddresses, 16),
		)

		ginkgo.DescribeTable("should be exposed by the controller", func(ipFamily ipfamily.Family, poolAddress string, addressTotal int) {
			poolName := "bgp-test"

			peerAddrToName := make(map[string]string)
			for _, c := range FRRContainers {
				address := c.Ipv4
				if ipFamily == ipfamily.IPv6 {
					address = c.Ipv6
				}
				peerAddr := address + fmt.Sprintf(":%d", c.RouterConfig.BGPPort)
				if FRRProvider != nil && FRRProvider.FRRK8sBased() {
					peerAddr = address
				}
				peerAddrToName[peerAddr] = c.Name
			}

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
				Peers:   metallb.PeersForContainers(FRRContainers, ipFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipFamily)
				Expect(err).NotTo(HaveOccurred())
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateFRRPeeredWithAllNodes(cs, c, ipFamily)
			}

			ginkgo.By("checking the metrics when no service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(promPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_allocator_addresses_in_use_total{pool="%s"} == 0`, poolName), metrics.There)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(addressTotal, "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_allocator_addresses_total{pool="%s"} == %d`, poolName, addressTotal), metrics.There)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			selectors := labelsForPeers(FRRContainers, ipFamily)

			// This is the only test where the same bgp metrics can be from either
			// the speaker container or frr depending on the deployment's bgp mode.
			bgpMetricsPodForSpeaker := func(p *corev1.Pod) (*corev1.Pod, string, error) {
				if FRRProvider == nil {
					return p, "metallb", nil
				}
				return FRRProvider.BGPMetricsPodFor(p.Namespace, p.Name)
			}

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					metricsPod, metricsPrefix, err := bgpMetricsPodForSpeaker(speaker)
					if err != nil {
						return err
					}

					speakerMetrics, err := metrics.ForPod(promPod, metricsPod, metallb.Namespace)
					if err != nil {
						return err
					}
					for _, selector := range selectors {
						err = metrics.ValidateGaugeValue(1, fmt.Sprintf("%s_bgp_session_up", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_session_up{%s} == 1`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}
						err = metrics.ValidateGaugeValue(0, fmt.Sprintf("%s_bgp_announced_prefixes_total", metricsPrefix), selector.labelsBGP, speakerMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_announced_prefixes_total{%s} == 0`, selector.labelsForQueryBGP), metrics.There)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			}

			ginkgo.By("creating a service")
			svc, _ := testservice.CreateWithBackend(cs, testNamespace, "external-local-lb", testservice.TrafficPolicyCluster) // Is a sleep required here?
			defer testservice.Delete(cs, svc)

			ginkgo.By("checking the metrics when a service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(promPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_allocator_addresses_in_use_total{pool="%s"} == 1`, poolName), metrics.There)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					bgpMetricsPod, metricsPrefix, err := bgpMetricsPodForSpeaker(speaker)
					if err != nil {
						return err
					}

					bgpMetrics, err := metrics.ForPod(promPod, bgpMetricsPod, metallb.Namespace)
					if err != nil {
						return err
					}

					speakerMetrics, err := metrics.ForPod(promPod, speaker, metallb.Namespace)
					if err != nil {
						return err
					}

					for addr := range peerAddrToName {
						err = metrics.ValidateGaugeValue(1, fmt.Sprintf("%s_bgp_session_up", metricsPrefix), map[string]string{"peer": addr}, bgpMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_session_up{peer="%s"} == 1`, addr), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateGaugeValue(1, fmt.Sprintf("%s_bgp_announced_prefixes_total", metricsPrefix), map[string]string{"peer": addr}, bgpMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_announced_prefixes_total{peer="%s"} == 1`, addr), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bgp_updates_total", metricsPrefix), map[string]string{"peer": addr}, bgpMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bgp_updates_total{peer="%s"} >= 1`, addr), metrics.There)
						if err != nil {
							return err
						}
					}

					err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": speaker.Spec.NodeName, "protocol": "bgp", "service": fmt.Sprintf("%s/%s", testNamespace, svc.Name)}, speakerMetrics)
					if err != nil {
						return err
					}
					return nil
				}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
			}
		},
			ginkgo.Entry("IPV4 - Checking service", ipfamily.IPv4, v4PoolAddresses, 256),
			ginkgo.Entry("IPV6 - Checking service", ipfamily.IPv6, v6PoolAddresses, 16))
	})

	ginkgo.DescribeTable("BFD metrics from FRR", func(bfd metallbv1beta1.BFDProfile, pairingFamily ipfamily.Family, poolAddresses []string) {
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

		for _, c := range FRRContainers {
			validateFRRPeeredWithAllNodes(cs, c, pairingFamily)
		}

		ginkgo.By("checking metrics")
		speakerPods, err := metallb.SpeakerPods(cs)
		Expect(err).NotTo(HaveOccurred())
		promPod, err := metrics.PrometheusPod(cs, PrometheusNamespace)
		Expect(err).NotTo(HaveOccurred())

		selectors := labelsForPeers(FRRContainers, pairingFamily)

		for _, speaker := range speakerPods {
			ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

			Eventually(func() error {
				metricsPod, metricsPrefix, err := FRRProvider.BGPMetricsPodFor(speaker.Namespace, speaker.Name)
				if err != nil {
					return err
				}

				speakerMetrics, err := metrics.ForPod(promPod, metricsPod, metallb.Namespace)
				if err != nil {
					return err
				}

				for _, selector := range selectors {
					err = metrics.ValidateGaugeValue(1, fmt.Sprintf("%s_bfd_session_up", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_session_up{%s} == 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bfd_control_packet_input", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_control_packet_input{%s} >= 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bfd_control_packet_output", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_control_packet_output{%s} >= 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateGaugeValueCompare(metrics.GreaterOrEqualThan(0), fmt.Sprintf("%s_bfd_session_down_events", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_session_down_events{%s} >= 0`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bfd_session_up_events", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_session_up_events{%s} >= 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bfd_zebra_notifications", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_zebra_notifications{%s} >= 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					if bfd.Spec.EchoMode != nil && *bfd.Spec.EchoMode {
						echoVal := 1
						if selector.noEcho {
							echoVal = 0
						}
						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(echoVal), fmt.Sprintf("%s_bfd_echo_packet_input", metricsPrefix), selector.labelsBFD, speakerMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_echo_packet_input{%s} >= %d`, selector.labelsForQueryBFD, echoVal), metrics.There)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(echoVal), fmt.Sprintf("%s_bfd_echo_packet_output", metricsPrefix), selector.labelsBFD, speakerMetrics)
						if err != nil {
							return err
						}
						err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_echo_packet_output{%s} >= %d`, selector.labelsForQueryBFD, echoVal), metrics.There)
						if err != nil {
							return err
						}
					}
				}
				return nil
			}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
		}

		ginkgo.By("disabling BFD in external FRR containers")
		for _, c := range FRRContainers {
			err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
				container.NeighborConfig.BFDEnabled = false
			})
			Expect(err).NotTo(HaveOccurred())
		}

		ginkgo.By("validating session down metrics")
		for _, speaker := range speakerPods {
			ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

			Eventually(func() error {
				metricsPod, metricsPrefix, err := FRRProvider.BGPMetricsPodFor(speaker.Namespace, speaker.Name)
				if err != nil {
					return err
				}

				speakerMetrics, err := metrics.ForPod(promPod, metricsPod, metallb.Namespace)
				if err != nil {
					return err
				}

				for _, selector := range selectors {
					err = metrics.ValidateGaugeValue(0, fmt.Sprintf("%s_bfd_session_up", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_session_up{%s} == 0`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}

					err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), fmt.Sprintf("%s_bfd_session_down_events", metricsPrefix), selector.labelsBFD, speakerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_bfd_session_down_events{%s} >= 1`, selector.labelsForQueryBFD), metrics.There)
					if err != nil {
						return err
					}
				}
				return nil
			}, 2*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
		}
	},
		ginkgo.Entry("IPV4 - default",
			metallbv1beta1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}, ipfamily.IPv4, []string{v4PoolAddresses}),
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
			}, ipfamily.IPv4, []string{v4PoolAddresses}),
		ginkgo.Entry("IPV6 - default",
			metallbv1beta1.BFDProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}, ipfamily.IPv6, []string{v6PoolAddresses}),
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
			}, ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}),
	)

	ginkgo.It("FRR metrics related to config should be exposed", func() {
		controllerPod, err := metallb.ControllerPod(cs)
		Expect(err).NotTo(HaveOccurred())

		speakers, err := metallb.SpeakerPods(cs)
		Expect(err).NotTo(HaveOccurred())
		allPods := append(speakers, controllerPod)

		bfdProfile := metallbv1beta1.BFDProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bfd",
			},
		}

		ginkgo.By("Creating an invalid configuration")

		resources := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "metrics-test",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{v4PoolAddresses},
					},
				},
			},
			Peers:   metallb.WithBFD(metallb.PeersForContainers(FRRContainers, ipfamily.IPv4), "bfd"),
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
		}
		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

		promPod, err := metrics.PrometheusPod(cs, PrometheusNamespace)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Checking the config stale metric on the speakers")
		for _, pod := range speakers {
			ginkgo.By(fmt.Sprintf("checking pod %s", pod.Name))
			Eventually(func() error {
				podMetrics, err := metrics.ForPod(promPod, pod, metallb.Namespace)
				Expect(err).NotTo(HaveOccurred())
				err = metrics.ValidateGaugeValue(1, "metallb_k8s_client_config_stale_bool", map[string]string{}, podMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_k8s_client_config_stale_bool{pod="%s"} == 1`, pod.Name), metrics.There)
				if err != nil {
					return err
				}
				return nil
			}, time.Minute, 1*time.Second).ShouldNot(HaveOccurred(), "on pod", pod.Name)
		}

		resources.BFDProfiles = []metallbv1beta1.BFDProfile{bfdProfile}
		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range allPods {
			ginkgo.By(fmt.Sprintf("checking pod %s", pod.Name))
			Eventually(func() error {
				podMetrics, err := metrics.ForPod(promPod, pod, metallb.Namespace)
				Expect(err).NotTo(HaveOccurred())
				err = metrics.ValidateGaugeValue(0, "metallb_k8s_client_config_stale_bool", map[string]string{}, podMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_k8s_client_config_stale_bool{pod="%s"} == 0`, pod.Name), metrics.There)
				if err != nil {
					return err
				}
				// we don't know how many events we are processing
				err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(0), "metallb_k8s_client_updates_total", map[string]string{}, podMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_k8s_client_updates_total{pod="%s"} > 0`, pod.Name), metrics.There)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(1, "metallb_k8s_client_config_loaded_bool", map[string]string{}, podMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_k8s_client_config_loaded_bool{pod="%s"} == 1`, pod.Name), metrics.There)
				if err != nil {
					return err
				}
				return nil
			}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred(), "on pod", pod.Name)
		}
	})
})

type peerPrometheus struct {
	labelsForQueryBGP string
	labelsBGP         map[string]string
	labelsForQueryBFD string
	labelsBFD         map[string]string
	noEcho            bool
}

func labelsForPeers(peers []*frrcontainer.FRR, ipFamily ipfamily.Family) []peerPrometheus {
	res := make([]peerPrometheus, 0)
	for _, c := range peers {
		address := c.Ipv4
		if ipFamily == ipfamily.IPv6 {
			address = c.Ipv6
		}
		peerAddr := address + fmt.Sprintf(":%d", c.RouterConfig.BGPPort)
		if FRRProvider != nil && FRRProvider.FRRK8sBased() {
			peerAddr = address
		}

		// Note: we deliberately don't add the vrf label in case of the default vrf to validate that
		// it is still possible to list the metrics using only the peer label, which is what most users
		// who don't care about vrfs should do.
		labelsBGP := map[string]string{"peer": peerAddr}
		labelsForQueryBGP := fmt.Sprintf(`peer="%s"`, peerAddr)
		labelsBFD := map[string]string{"peer": address}
		labelsForQueryBFD := fmt.Sprintf(`peer="%s"`, address)

		noEcho := c.NeighborConfig.MultiHop
		if c.RouterConfig.VRF != "" {
			labelsBGP["vrf"] = c.RouterConfig.VRF
			labelsForQueryBGP = fmt.Sprintf(`peer="%s",vrf="%s"`, peerAddr, c.RouterConfig.VRF)
			labelsBFD["vrf"] = c.RouterConfig.VRF
			labelsForQueryBFD = fmt.Sprintf(`peer="%s",vrf="%s"`, address, c.RouterConfig.VRF)
			noEcho = true // TODO: Need to understand if echo not working across VRFs is expected. If it is, set a webhook to prevent it.
		}
		res = append(res, peerPrometheus{
			labelsBGP:         labelsBGP,
			labelsForQueryBGP: labelsForQueryBGP,
			labelsBFD:         labelsBFD,
			labelsForQueryBFD: labelsForQueryBFD,
			noEcho:            noEcho,
		})
	}
	return res
}
