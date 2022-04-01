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
	"fmt"
	"strings"
	"time"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/metrics"
	metallbconfig "go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/pointer"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"go.universe.tf/metallb/e2etest/pkg/frr"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	"go.universe.tf/metallb/internal/ipfamily"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const (
	v4PoolAddresses      = "192.168.10.0/24"
	v6PoolAddresses      = "fc00:f853:0ccd:e799::/124"
	CommunityNoAdv       = "65535:65282" // 0xFFFFFF02: NO_ADVERTISE
	IPLocalPref          = uint32(300)
	SpeakerContainerName = "speaker"
)

var ConfigUpdater config.Updater

var _ = ginkgo.Describe("BGP", func() {
	var cs clientset.Interface
	var f *framework.Framework
	emptyBGPAdvertisement := metallbv1beta1.BGPAdvertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentGinkgoTestDescription().Failed {
			for _, c := range FRRContainers {
				address := c.Ipv4
				if address == "" {
					address = c.Ipv6
				}
				peerAddr := address + fmt.Sprintf(":%d", c.RouterConfig.BGPPort)
				dump, err := frr.RawDump(c, "/etc/frr/bgpd.conf", "/tmp/frr.log", "/etc/frr/daemons")
				framework.Logf("External frr dump for %s:%s\n%s\nerrors:%v", c.Name, peerAddr, dump, err)
			}

			speakerPods, err := metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)
			for _, pod := range speakerPods {
				if len(pod.Spec.Containers) == 1 { // we dump only in case of frr
					break
				}
				podExec := executor.ForPod(pod.Namespace, pod.Name, "frr")
				dump, err := frr.RawDump(podExec, "/etc/frr/frr.conf", "/etc/frr/frr.log")
				framework.Logf("External frr dump for pod %s\n%s %v", pod.Name, dump, err)
			}
			k8s.DescribeSvc(f.Namespace.Name)
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

	table.DescribeTable("A service of protocol load balancer should work with", func(pairingIPFamily ipfamily.Family, setProtocoltest string, poolAddresses []string, tweak testservice.Tweak) {
		var allNodes *corev1.NodeList
		resources := metallbconfig.ClusterResources{
			Pools: []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-test",
					},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: poolAddresses,
					},
				},
			},
			Peers:   metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
		}
		for _, c := range FRRContainers {
			err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily)
			framework.ExpectNoError(err)
		}

		err := ConfigUpdater.Update(resources)
		framework.ExpectNoError(err)

		for _, c := range FRRContainers {
			validateFRRPeeredWithNodes(cs, c, pairingIPFamily)
		}

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)

		if setProtocoltest == "ExternalTrafficPolicyCluster" {

			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			validateDesiredLB(svc)

			for _, c := range FRRContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		}

		if setProtocoltest == "ExternalTrafficPolicyLocal" {
			svc, jig := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			err = jig.Scale(2)
			framework.ExpectNoError(err)

			epNodes, err := jig.ListNodesWithEndpoint() // Only nodes with an endpoint should be advertising the IP
			framework.ExpectNoError(err)

			defer testservice.Delete(cs, svc)

			for _, c := range FRRContainers {
				validateService(cs, svc, epNodes, c)
			}
		}

		if setProtocoltest == "CheckSpeakerFRRPodRunning" {
			for _, c := range FRRContainers {
				frrIsPairedOnPods(cs, c, pairingIPFamily)
			}
		}
	},
		table.Entry("IPV4 - ExternalTrafficPolicyCluster", ipfamily.IPv4, "ExternalTrafficPolicyCluster", []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
		table.Entry("IPV4 - ExternalTrafficPolicyLocal", ipfamily.IPv4, "ExternalTrafficPolicyLocal", []string{v4PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV4 - FRR running in the speaker POD", ipfamily.IPv4, "CheckSpeakerFRRPodRunning", []string{v4PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV6 - ExternalTrafficPolicyCluster", ipfamily.IPv6, "ExternalTrafficPolicyCluster", []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
		table.Entry("IPV6 - ExternalTrafficPolicyLocal", ipfamily.IPv6, "ExternalTrafficPolicyLocal", []string{v6PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV6 - FRR running in the speaker POD", ipfamily.IPv6, "CheckSpeakerFRRPodRunning", []string{v6PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("DUALSTACK - ExternalTrafficPolicyCluster", ipfamily.DualStack, "ExternalTrafficPolicyCluster", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.DualStack(svc)
			}),
		table.Entry("DUALSTACK - ExternalTrafficPolicyLocal", ipfamily.DualStack, "ExternalTrafficPolicyLocal", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyLocal(svc)
				testservice.DualStack(svc)
			}),
		table.Entry("DUALSTACK - ExternalTrafficPolicyCluster - force V6 only", ipfamily.DualStack, "ExternalTrafficPolicyCluster", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.ForceV6(svc)
			}),
		table.Entry("IPV4 - ExternalTrafficPolicyCluster - request IPv4 via custom annotation", ipfamily.IPv4, "ExternalTrafficPolicyCluster", []string{v4PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.WithSpecificIPs(svc, "192.168.10.100")
			}),
		table.Entry("DUALSTACK - ExternalTrafficPolicyCluster - request Dual Stack via custom annotation", ipfamily.DualStack, "ExternalTrafficPolicyCluster", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.DualStack(svc)
				testservice.WithSpecificIPs(svc, "192.168.10.100", "fc00:f853:ccd:e799::")
			}),
	)

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods []*corev1.Pod

		ginkgo.BeforeEach(func() {
			var err error
			controllerPod, err = metallb.ControllerPod(cs)
			framework.ExpectNoError(err)
			speakerPods, err = metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)
		})

		table.DescribeTable("should be exposed by the controller", func(ipFamily ipfamily.Family, poolAddress string, addressTotal int) {
			poolName := "bgp-test"

			peerAddrToName := make(map[string]string)
			for _, c := range FRRContainers {
				address := c.Ipv4
				if ipFamily == ipfamily.IPv6 {
					address = c.Ipv6
				}
				peerAddr := address + fmt.Sprintf(":%d", c.RouterConfig.BGPPort)
				peerAddrToName[peerAddr] = c.Name
			}

			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: poolName,
						},
						Spec: metallbv1beta1.IPPoolSpec{
							Addresses: []string{poolAddress},
						},
					},
				},
				Peers:   metallb.PeersForContainers(FRRContainers, ipFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, ipFamily)
				framework.ExpectNoError(err)
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				validateFRRPeeredWithNodes(cs, c, ipFamily)
			}

			ginkgo.By("checking the metrics when no service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(addressTotal, "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(BeNil())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, metallb.Namespace)
					if err != nil {
						return err
					}
					for peerAddr, peerName := range peerAddrToName {
						err = metrics.ValidateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": peerAddr}, speakerMetrics)
						if err != nil {
							framework.Logf("frr metrics: %q, neighbor: %s-%s, speaker: %s", speakerMetrics, peerName, peerAddr, speaker.Namespace+"/"+speaker.Name)
							return err
						}
						err = metrics.ValidateGaugeValue(0, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": peerAddr}, speakerMetrics)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}

			ginkgo.By("creating a service")
			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster) // Is a sleep required here?
			defer testservice.Delete(cs, svc)

			ginkgo.By("checking the metrics when a service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(BeNil())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, metallb.Namespace)
					if err != nil {
						return err
					}
					for addr := range peerAddrToName {
						err = metrics.ValidateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateGaugeValue(1, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bgp_updates_total", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}
					}

					err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": speaker.Spec.NodeName, "protocol": "bgp", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
					if err != nil {
						return err
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}
		},
			table.Entry("IPV4 - Checking service", ipfamily.IPv4, v4PoolAddresses, 256),
			table.Entry("IPV6 - Checking service", ipfamily.IPv6, v6PoolAddresses, 16))
	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			framework.ExpectNoError(err)
		})

		table.DescribeTable("set different AddressPools ranges modes", func(addressPools []metallbv1beta1.IPPool, pairingFamily ipfamily.Family, tweak testservice.Tweak) {
			resources := metallbconfig.ClusterResources{
				Pools:   addressPools,
				Peers:   metallb.PeersForContainers(FRRContainers, pairingFamily),
				BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
			}

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily)
				framework.ExpectNoError(err)
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}

			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			for _, i := range svc.Status.LoadBalancer.Ingress {
				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := e2eservice.GetIngressPoint(&i)
				err = config.ValidateIPInRange(addressPools, ingressIP)
				framework.ExpectNoError(err)
			}

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		},
			table.Entry("IPV4 - test AddressPool defined by address range", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"192.168.10.0-192.168.10.18",
						},
					},
				}}, ipfamily.IPv4, testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV4 - test AddressPool defined by network prefix", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"192.168.10.0/24",
						},
					},
				}}, ipfamily.IPv4, testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV6 - test AddressPool defined by address range", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
						},
					},
				}}, ipfamily.IPv6, testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV6 - test AddressPool defined by network prefix", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"fc00:f853:0ccd:e799::/124",
						},
					},
				}}, ipfamily.IPv6, testservice.TrafficPolicyCluster,
			),
			table.Entry("DUALSTACK - test AddressPool defined by address range", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"192.168.10.0-192.168.10.18",
							"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
						},
					},
				}}, ipfamily.DualStack, testservice.TrafficPolicyCluster,
			),
			table.Entry("DUALSTACK - test AddressPool defined by network prefix", []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bgp-test"},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							"192.168.10.0/24",
							"fc00:f853:0ccd:e799::/124",
						},
					},
				}}, ipfamily.DualStack, testservice.TrafficPolicyCluster,
			),
		)
	})
	table.DescribeTable("configure peers with routerid and validate external containers are paired with nodes", func(ipFamily ipfamily.Family) {
		ginkgo.By("configure peer")

		resources := metallbconfig.ClusterResources{
			Peers: metallb.WithRouterID(metallb.PeersForContainers(FRRContainers, ipFamily), "10.10.10.1"),
		}

		err := ConfigUpdater.Update(resources)
		framework.ExpectNoError(err)

		for _, c := range FRRContainers {
			err = frrcontainer.PairWithNodes(cs, c, ipFamily)
			framework.ExpectNoError(err)
		}

		for _, c := range FRRContainers {
			validateFRRPeeredWithNodes(cs, c, ipFamily)
			neighbors, err := frr.NeighborsInfo(c)
			framework.ExpectNoError(err)
			for _, n := range neighbors {
				framework.ExpectEqual(n.RemoteRouterID, "10.10.10.1")
			}
		}
	},
		table.Entry("IPV4", ipfamily.IPv4),
		table.Entry("IPV6", ipfamily.IPv6))

	table.DescribeTable("validate external containers are paired with nodes", func(ipFamily ipfamily.Family) {
		ginkgo.By("configure peer")

		resources := metallbconfig.ClusterResources{
			Peers: metallb.PeersForContainers(FRRContainers, ipFamily, func(p *metallbv1beta2.BGPPeer) {
				p.Spec.PasswordSecret = corev1.SecretReference{Name: metallb.GetBGPPeerSecretName(p.Spec.ASN, p.Spec.Port)}
				p.Spec.Password = ""
			}),
			PasswordSecrets: metallb.BGPPeerSecretReferences(FRRContainers),
		}
		err := ConfigUpdater.Update(resources)
		framework.ExpectNoError(err)
		defer func() {
			for _, s := range resources.PasswordSecrets {
				err := cs.CoreV1().Secrets(metallb.Namespace).Delete(context.Background(), s.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}
		}()

		for _, c := range FRRContainers {
			err = frrcontainer.PairWithNodes(cs, c, ipFamily)
			framework.ExpectNoError(err)
		}

		for _, c := range FRRContainers {
			validateFRRPeeredWithNodes(cs, c, ipFamily)
		}
	},
		table.Entry("IPV4 with Secret Ref set for BGPPeer CR", ipfamily.IPv4),
		table.Entry("IPV6 with Secret Ref set for BGPPeer CR", ipfamily.IPv6))

	ginkgo.Context("BFD", func() {
		table.DescribeTable("should work with the given bfd profile", func(bfd metallbv1beta1.BFDProfile, pairingFamily ipfamily.Family, poolAddresses []string, tweak testservice.Tweak) {
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bfd-test",
						},
						Spec: metallbv1beta1.IPPoolSpec{
							Addresses: poolAddresses,
						},
					},
				},
				Peers:       metallb.WithBFD(metallb.PeersForContainers(FRRContainers, pairingFamily), bfd.Name),
				BGPAdvs:     []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				BFDProfiles: []metallbv1beta1.BFDProfile{bfd},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = true
				})
				framework.ExpectNoError(err)
			}

			svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			defer testservice.Delete(cs, svc)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}
			for _, c := range FRRContainers {
				validateService(cs, svc, allNodes.Items, c)
			}

			Eventually(func() error {
				for _, c := range FRRContainers {
					bfdPeers, err := frr.BFDPeers(c.Executor)
					if err != nil {
						return err
					}
					err = frr.BFDPeersMatchNodes(allNodes.Items, bfdPeers, pairingFamily)
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
			}, 4*time.Minute, 1*time.Second).Should(BeNil())

		},
			table.Entry("IPV4 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV4 - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(60),
						TransmitInterval: pointer.Uint32Ptr(61),
						EchoInterval:     pointer.Uint32Ptr(62),
						EchoMode:         pointer.BoolPtr(false),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV4 - echo mode enabled",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "echo",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(80),
						TransmitInterval: pointer.Uint32Ptr(81),
						EchoInterval:     pointer.Uint32Ptr(82),
						EchoMode:         pointer.BoolPtr(true),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(60),
						TransmitInterval: pointer.Uint32Ptr(61),
						EchoInterval:     pointer.Uint32Ptr(62),
						EchoMode:         pointer.BoolPtr(false),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - echo mode enabled",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "echo",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(80),
						TransmitInterval: pointer.Uint32Ptr(81),
						EchoInterval:     pointer.Uint32Ptr(82),
						EchoMode:         pointer.BoolPtr(true),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("DUALSTACK - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(60),
						TransmitInterval: pointer.Uint32Ptr(61),
						EchoInterval:     pointer.Uint32Ptr(62),
						EchoMode:         pointer.BoolPtr(false),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
				}),
		)

		table.DescribeTable("metrics", func(bfd metallbv1beta1.BFDProfile, pairingFamily ipfamily.Family, poolAddresses []string) {
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bfd-test",
						},
						Spec: metallbv1beta1.IPPoolSpec{
							Addresses: poolAddresses,
						},
					},
				},
				Peers:       metallb.WithBFD(metallb.PeersForContainers(FRRContainers, pairingFamily), bfd.Name),
				BGPAdvs:     []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				BFDProfiles: []metallbv1beta1.BFDProfile{bfd},
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = true
				})
				framework.ExpectNoError(err)
			}

			for _, c := range FRRContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}

			ginkgo.By("checking metrics")
			controllerPod, err := metallb.ControllerPod(cs)
			framework.ExpectNoError(err)
			speakerPods, err := metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)

			var peers []struct {
				addr     string
				multihop bool
			}

			for _, c := range FRRContainers {
				address := c.Ipv4
				if pairingFamily == ipfamily.IPv6 {
					address = c.Ipv6
				}

				peers = append(peers, struct {
					addr     string
					multihop bool
				}{
					address,
					c.NeighborConfig.MultiHop,
				},
				)
			}

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, metallb.Namespace)
					if err != nil {
						return err
					}

					for _, peer := range peers {
						err = metrics.ValidateGaugeValue(1, "metallb_bfd_session_up", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bfd_control_packet_input", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bfd_control_packet_output", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateGaugeValue(0, "metallb_bfd_session_down_events", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bfd_session_up_events", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bfd_zebra_notifications", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						if bfd.Spec.EchoMode != nil && *bfd.Spec.EchoMode {
							echoVal := 1
							if peer.multihop {
								echoVal = 0
							}
							err = metrics.ValidateCounterValue(echoVal, "metallb_bfd_echo_packet_input", map[string]string{"peer": peer.addr}, speakerMetrics)
							if err != nil {
								return err
							}

							err = metrics.ValidateCounterValue(echoVal, "metallb_bfd_echo_packet_output", map[string]string{"peer": peer.addr}, speakerMetrics)
							if err != nil {
								return err
							}
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}

			ginkgo.By("disabling BFD in external FRR containers")
			for _, c := range FRRContainers {
				err := frrcontainer.PairWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = false
				})
				framework.ExpectNoError(err)
			}

			ginkgo.By("validating session down metrics")
			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, metallb.Namespace)
					if err != nil {
						return err
					}

					for _, peer := range peers {
						err = metrics.ValidateGaugeValue(0, "metallb_bfd_session_up", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = metrics.ValidateCounterValue(1, "metallb_bfd_session_down_events", map[string]string{"peer": peer.addr}, speakerMetrics)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}
		},
			table.Entry("IPV4 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}),
			table.Entry("IPV4 - echo mode enabled",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "echo",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(80),
						TransmitInterval: pointer.Uint32Ptr(81),
						EchoInterval:     pointer.Uint32Ptr(82),
						EchoMode:         pointer.BoolPtr(true),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv4, []string{v4PoolAddresses}),
			table.Entry("IPV6 - default",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}),
			table.Entry("IPV6 - echo mode enabled",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "echo",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(80),
						TransmitInterval: pointer.Uint32Ptr(81),
						EchoInterval:     pointer.Uint32Ptr(82),
						EchoMode:         pointer.BoolPtr(true),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.IPv6, []string{v6PoolAddresses}),
			table.Entry("DUALSTACK - full params",
				metallbv1beta1.BFDProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "full1",
					},
					Spec: metallbv1beta1.BFDProfileSpec{
						ReceiveInterval:  pointer.Uint32Ptr(60),
						TransmitInterval: pointer.Uint32Ptr(61),
						EchoInterval:     pointer.Uint32Ptr(62),
						EchoMode:         pointer.BoolPtr(false),
						PassiveMode:      pointer.BoolPtr(false),
						MinimumTTL:       pointer.Uint32Ptr(254),
					},
				}, ipfamily.DualStack, []string{v4PoolAddresses, v6PoolAddresses}),
		)
	})

	ginkgo.Context("validate configuration changes", func() {
		table.DescribeTable("should work after subsequent configuration updates", func(addressRange string, ipFamily ipfamily.Family) {
			var services []*corev1.Service
			var servicesIngressIP []string
			var pools []metallbv1beta1.IPPool

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for i := 0; i < 2; i++ {
				ginkgo.By(fmt.Sprintf("configure addresspool number %d", i+1))
				firstIP, err := config.GetIPFromRangeByIndex(addressRange, i*10+1)
				framework.ExpectNoError(err)
				lastIP, err := config.GetIPFromRangeByIndex(addressRange, i*10+10)
				framework.ExpectNoError(err)
				addressesRange := fmt.Sprintf("%s-%s", firstIP, lastIP)
				pool := metallbv1beta1.IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("test-addresspool%d", i+1),
					},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{addressesRange},
					},
				}
				pools = append(pools, pool)

				resources := metallbconfig.ClusterResources{
					Pools:   pools,
					Peers:   metallb.PeersForContainers(FRRContainers, ipFamily),
					BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				}

				for _, c := range FRRContainers {
					err := frrcontainer.PairWithNodes(cs, c, ipFamily)
					framework.ExpectNoError(err)
				}

				err = ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				for _, c := range FRRContainers {
					validateFRRPeeredWithNodes(cs, c, ipFamily)
				}

				ginkgo.By(fmt.Sprintf("configure service number %d", i+1))
				svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, fmt.Sprintf("svc%d", i+1), testservice.TrafficPolicyCluster, func(svc *corev1.Service) {
					svc.Annotations = map[string]string{"metallb.universe.tf/address-pool": fmt.Sprintf("test-addresspool%d", i+1)}
				})
				defer testservice.Delete(cs, svc)

				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := e2eservice.GetIngressPoint(
					&svc.Status.LoadBalancer.Ingress[0])
				err = config.ValidateIPInRange([]metallbv1beta1.IPPool{pool}, ingressIP)
				framework.ExpectNoError(err)

				services = append(services, svc)
				servicesIngressIP = append(servicesIngressIP, ingressIP)

				for j := 0; j <= i; j++ {
					ginkgo.By(fmt.Sprintf("validate service %d IP didn't change", j+1))
					ip := e2eservice.GetIngressPoint(&services[j].Status.LoadBalancer.Ingress[0])
					framework.ExpectEqual(ip, servicesIngressIP[j])

					ginkgo.By(fmt.Sprintf("checking connectivity of service %d to its external VIP", j+1))
					for _, c := range FRRContainers {
						validateService(cs, svc, allNodes.Items, c)
					}
				}
			}
		},
			table.Entry("IPV4", "192.168.10.0/24", ipfamily.IPv4),
			table.Entry("IPV6", "fc00:f853:0ccd:e799::/116", ipfamily.IPv6))

		table.DescribeTable("configure peers one by one and validate FRR paired with nodes", func(ipFamily ipfamily.Family) {
			for i, c := range FRRContainers {
				ginkgo.By(fmt.Sprintf("configure FRR peer [%s]", c.Name))

				resources := metallbconfig.ClusterResources{
					Peers:   metallb.PeersForContainers([]*frrcontainer.FRR{c}, ipFamily),
					BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
				}
				err := ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				err = frrcontainer.PairWithNodes(cs, c, ipFamily)
				framework.ExpectNoError(err)

				validateFRRPeeredWithNodes(cs, FRRContainers[i], ipFamily)
			}
		},
			table.Entry("IPV4", ipfamily.IPv4),
			table.Entry("IPV6", ipfamily.IPv6))

		table.DescribeTable("configure bgp advertisement and verify it gets propagated",
			func(rangeWithAdvertisement string, rangeWithoutAdvertisement string, advertisement metallbv1beta1.BGPAdvertisement, legacy bool, ipFamily ipfamily.Family) {
				emptyAdvertisement := metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: "empty",
					},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						IPPools: []string{"bgp-with-no-advertisement"},
					},
				}

				poolWithAdvertisement := metallbv1beta1.IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-with-advertisement",
					},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{rangeWithAdvertisement},
					},
				}
				poolWithoutAdvertisement := metallbv1beta1.IPPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bgp-with-no-advertisement",
					},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{rangeWithoutAdvertisement},
					},
				}

				resources := metallbconfig.ClusterResources{
					Peers: metallb.PeersForContainers(FRRContainers, ipFamily),
				}

				if !legacy {
					resources.Pools = []metallbv1beta1.IPPool{poolWithAdvertisement, poolWithoutAdvertisement}
					resources.BGPAdvs = []metallbv1beta1.BGPAdvertisement{emptyAdvertisement, advertisement}
				} else {
					resources.LegacyAddressPools = make([]metallbv1beta1.AddressPool, 0)
					resources.LegacyAddressPools = []metallbv1beta1.AddressPool{
						config.IPPoolToLegacy(poolWithAdvertisement, metallbconfig.BGP, []metallbv1beta1.BGPAdvertisement{advertisement}),
						config.IPPoolToLegacy(poolWithoutAdvertisement, metallbconfig.BGP, []metallbv1beta1.BGPAdvertisement{}),
					}
				}

				for _, c := range FRRContainers {
					err := frrcontainer.PairWithNodes(cs, c, ipFamily)
					framework.ExpectNoError(err)
				}

				err := ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				for _, c := range FRRContainers {
					validateFRRPeeredWithNodes(cs, c, ipFamily)
				}

				ipWithAdvertisement, err := config.GetIPFromRangeByIndex(rangeWithAdvertisement, 0)
				framework.ExpectNoError(err)
				ipWithAdvertisement1, err := config.GetIPFromRangeByIndex(rangeWithAdvertisement, 1)
				framework.ExpectNoError(err)
				ipNoAdvertisement, err := config.GetIPFromRangeByIndex(rangeWithoutAdvertisement, 0)
				framework.ExpectNoError(err)

				svcAdvertisement, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "service-with-adv",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipWithAdvertisement
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcAdvertisement)
				svcAdvertisement1, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "service-with-adv1",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipWithAdvertisement1
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcAdvertisement1)
				svcNoAdvertisement, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "service-no-adv",
					func(s *corev1.Service) {
						s.Spec.LoadBalancerIP = ipNoAdvertisement
					},
					testservice.TrafficPolicyCluster)
				defer testservice.Delete(cs, svcNoAdvertisement)

				allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				framework.ExpectNoError(err)

				for _, c := range FRRContainers {
					validateService(cs, svcAdvertisement, allNodes.Items, c)
					validateService(cs, svcAdvertisement1, allNodes.Items, c)
					validateService(cs, svcNoAdvertisement, allNodes.Items, c)
					Eventually(func() error {
						for _, community := range advertisement.Spec.Communities {
							routes, err := frr.RoutesForCommunity(c, community, ipFamily)
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
					}, 1*time.Minute, 1*time.Second).Should(BeNil())
				}

			},
			table.Entry("IPV4 - community and localpref",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities: []string{CommunityNoAdv},
						LocalPref:   50,
						IPPools:     []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv4),
			table.Entry("IPV4 - localpref",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref: 50,
						IPPools:   []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv4),
			table.Entry("IPV4 - community",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities: []string{CommunityNoAdv},
						IPPools:     []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv4),
			table.Entry("IPV4 - community and localpref - legacy",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities: []string{CommunityNoAdv},
						LocalPref:   50,
						IPPools:     []string{"bgp-with-advertisement"},
					},
				},
				true,
				ipfamily.IPv4),
			table.Entry("IPV4 - localpref - legacy",
				"192.168.10.0/24",
				"192.168.16.0/24",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref: 50,
						IPPools:   []string{"bgp-with-advertisement"},
					},
				},
				true,
				ipfamily.IPv4),
			table.Entry("IPV6 - community and localpref",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref:   50,
						Communities: []string{CommunityNoAdv},
						IPPools:     []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv6),
			table.Entry("IPV6 - community",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						Communities: []string{CommunityNoAdv},
						IPPools:     []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv6),
			table.Entry("IPV6 - localpref",
				"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
				"fc00:f853:0ccd:e799::19-fc00:f853:0ccd:e799::26",
				metallbv1beta1.BGPAdvertisement{
					ObjectMeta: metav1.ObjectMeta{Name: "advertisement"},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						LocalPref: 50,
						IPPools:   []string{"bgp-with-advertisement"},
					},
				},
				false,
				ipfamily.IPv6))

	})

	table.DescribeTable("MetalLB FRR rejects any routes advertised by any neighbor", func(addressesRange, toInject string, pairingIPFamily ipfamily.Family) {
		resources := metallbconfig.ClusterResources{
			Pools: []metallbv1beta1.IPPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rejectroutes",
					},
					Spec: metallbv1beta1.IPPoolSpec{
						Addresses: []string{
							addressesRange,
						},
					},
				},
			},
			Peers:   metallb.PeersForContainers(FRRContainers, pairingIPFamily),
			BGPAdvs: []metallbv1beta1.BGPAdvertisement{emptyBGPAdvertisement},
		}

		neighborAnnounce := func(frr *frrcontainer.FRR) {
			frr.NeighborConfig.ToAdvertise = toInject
		}

		for _, c := range FRRContainers {
			err := frrcontainer.PairWithNodes(cs, c, pairingIPFamily, neighborAnnounce)
			framework.ExpectNoError(err)
		}

		err := ConfigUpdater.Update(resources)
		framework.ExpectNoError(err)

		for _, c := range FRRContainers {
			validateFRRPeeredWithNodes(cs, c, pairingIPFamily)
		}
		speakerPods, err := metallb.SpeakerPods(cs)
		framework.ExpectNoError(err)

		checkRoutesInjected := func() error {
			for _, pod := range speakerPods {
				podExec := executor.ForPod(pod.Namespace, pod.Name, "frr")
				routes, frrRoutesV6, err := frr.Routes(podExec)
				framework.ExpectNoError(err)

				if pairingIPFamily == ipfamily.IPv6 {
					routes = frrRoutesV6
				}

				for _, route := range routes {
					if route.Destination.String() == toInject {
						return fmt.Errorf("Found %s in %s routes", toInject, pod.Name)
					}
				}
			}
			return nil
		}

		Consistently(checkRoutesInjected, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		svc, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb")
		defer testservice.Delete(cs, svc)

		Consistently(checkRoutesInjected, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

	},
		table.Entry("IPV4", "192.168.10.0/24", "172.16.1.10/32", ipfamily.IPv4),
		table.Entry("IPV6", "fc00:f853:0ccd:e799::/116", "fc00:f853:ccd:e800::1/128", ipfamily.IPv6),
	)

	ginkgo.Context("FRR validate reload feedback", func() {
		ginkgo.It("should update MetalLB config and log reload-validate success", func() {
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "new-config",
						},
						Spec: metallbv1beta1.IPPoolSpec{
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
			framework.ExpectNoError(err)

			speakerPods, err := metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)

			for _, pod := range speakerPods {
				Eventually(func() string {
					logs, err := k8s.PodLogsSinceTime(cs, pod, SpeakerContainerName, &beforeUpdateTime)
					framework.ExpectNoError(err)

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
			resources := metallbconfig.ClusterResources{
				Pools: []metallbv1beta1.IPPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "bgp-test",
						},
						Spec: metallbv1beta1.IPPoolSpec{
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
							ReceiveInterval:  pointer.Uint32Ptr(93),
							TransmitInterval: pointer.Uint32Ptr(95),
							EchoInterval:     pointer.Uint32Ptr(97),
							EchoMode:         pointer.BoolPtr(true),
							PassiveMode:      pointer.BoolPtr(true),
							MinimumTTL:       pointer.Uint32Ptr(253),
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
				resources.Peers[i].Spec.KeepaliveTime = metav1.Duration{Duration: 13 * time.Second}
				resources.Peers[i].Spec.HoldTime = metav1.Duration{Duration: 57 * time.Second}
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			speakerPods, err := metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)

			for _, pod := range speakerPods {
				podExecutor := executor.ForPod(pod.Namespace, pod.Name, "frr")

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
						ContainSubstring("log file /etc/frr/frr.log informational"),
						WithTransform(substringCount("\n profile fullbfdprofile1"), Equal(1)),
						ContainSubstring("receive-interval 93"),
						ContainSubstring("transmit-interval 95"),
						ContainSubstring("echo-interval 97"),
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
			framework.ExpectNoError(err)
			framework.ExpectEqual(peer.Spec.Port, uint16(179))
		})
	})

})

// substringCount creates a Gomega transform function that
// counts the number of occurrences in the subject string.
func substringCount(substr string) interface{} {
	return func(action string) int {
		return strings.Count(action, substr)
	}
}
