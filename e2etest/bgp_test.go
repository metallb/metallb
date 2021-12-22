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

package e2e

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/metrics"
	"go.universe.tf/metallb/e2etest/pkg/routes"

	"golang.org/x/sync/errgroup"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	dto "github.com/prometheus/client_model/go"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	bgpfrr "go.universe.tf/metallb/internal/bgp/frr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const (
	frrIBGP         = "frr-iBGP"
	frrEBGP         = "frr-eBGP"
	MetalLBASN      = 64512
	ExternalASN     = 64513
	baseRouterID    = "10.10.10.%d"
	v4PoolAddresses = "192.168.10.0/24"
	v6PoolAddresses = "fc00:f853:0ccd:e799::/124"
	CommunityNoAdv  = "65535:65282" // 0xFFFFFF02: NO_ADVERTISE
	IPLocalPref     = uint32(300)
)

var (
	frrContainers []*frrcontainer.FRR
)

func setupContainers(ipv4Addresses, ipv6Addresses []string) []*frrcontainer.FRR {

	ibgpContainerConfig := frrcontainer.Config{
		Name: frrIBGP,
		Neighbor: frrconfig.NeighborConfig{
			ASN:      MetalLBASN,
			Password: "ibgp-test",
		},
		Router: frrconfig.RouterConfig{
			ASN:      MetalLBASN,
			BGPPort:  179,
			Password: "ibgp-test",
		},
		Network:  containersNetwork,
		HostIPv4: hostIPv4,
		HostIPv6: hostIPv6,
	}
	ebgpContainerConfig := frrcontainer.Config{
		Name: frrEBGP,
		Neighbor: frrconfig.NeighborConfig{
			ASN:      MetalLBASN,
			Password: "ebgp-test",
		},
		Router: frrconfig.RouterConfig{
			ASN:      ExternalASN,
			BGPPort:  180,
			Password: "ebgp-test",
		},
		Network:  containersNetwork,
		HostIPv4: hostIPv4,
		HostIPv6: hostIPv6,
	}

	var res []*frrcontainer.FRR
	var err error
	if containersNetwork == "host" {
		res, err = createFRRContainers(ibgpContainerConfig)
	} else {
		Expect(len(ipv4Addresses)).Should(BeNumerically(">=", 2))
		Expect(len(ipv6Addresses)).Should(BeNumerically(">=", 2))

		ibgpContainerConfig.IPv4Address = ipv4Addresses[0]
		ibgpContainerConfig.IPv6Address = ipv6Addresses[0]
		ebgpContainerConfig.IPv4Address = ipv4Addresses[1]
		ebgpContainerConfig.IPv6Address = ipv6Addresses[1]
		res, err = createFRRContainers(ibgpContainerConfig, ebgpContainerConfig)
	}
	framework.ExpectNoError(err)
	return res
}

func tearDownContainers(containers []*frrcontainer.FRR) {
	err := stopFRRContainers(containers)
	framework.ExpectNoError(err)
}

var _ = ginkgo.Describe("BGP", func() {
	var configUpdater config.Updater
	var cs clientset.Interface
	var f *framework.Framework

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentGinkgoTestDescription().Failed {
			for _, c := range frrContainers {
				dump, err := frr.RawDump(c, "/etc/frr/bgpd.conf", "/tmp/frr.log", "/etc/frr/daemons")
				framework.Logf("External frr dump for %s:\n%s\nerrors:%v", c.Name, dump, err)
			}

			speakerPods := getSpeakerPods(cs)
			for _, pod := range speakerPods {
				podExec := executor.ForPod(pod.Namespace, pod.Name, "frr")
				dump, err := frr.RawDump(podExec, "/etc/frr/frr.conf", "/etc/frr/frr.log")
				framework.Logf("External frr dump for pod %s\n%s %v", pod.Name, dump, err)
			}
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.AfterEach(func() {
		ginkgo.By("Clearing the previous configuration")
		// Clean previous configuration.
		err := configUpdater.Clean()
		framework.ExpectNoError(err)

		for _, c := range frrContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			framework.ExpectNoError(err)
		}
	})

	f = framework.NewDefaultFramework("bgp")

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
		configUpdater = config.UpdaterForConfigMap(cs, configMapName, testNameSpace)
		if useOperator {
			clientconfig := f.ClientConfig()
			var err error
			configUpdater, err = config.UpdaterForOperator(clientconfig, testNameSpace)
			framework.ExpectNoError(err)
		}
	})

	table.DescribeTable("A service of protocol load balancer should work with", func(pairingIPFamily, setProtocoltest string, poolAddresses []string, tweak testservice.Tweak) {
		var allNodes *corev1.NodeList
		configData := config.File{
			Pools: []config.AddressPool{
				{
					Name:      "bgp-test",
					Protocol:  config.BGP,
					Addresses: poolAddresses,
				},
			},
			Peers: peersForContainers(frrContainers, pairingIPFamily),
		}
		for _, c := range frrContainers {
			pairExternalFRRWithNodes(cs, c, pairingIPFamily)
		}

		err := configUpdater.Update(configData)
		framework.ExpectNoError(err)

		for _, c := range frrContainers {
			validateFRRPeeredWithNodes(cs, c, pairingIPFamily)
		}

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)

		if setProtocoltest == "ExternalTrafficPolicyCluster" {
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			for _, c := range frrContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		}

		if setProtocoltest == "ExternalTrafficPolicyLocal" {
			svc, jig := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			err = jig.Scale(2)
			framework.ExpectNoError(err)

			epNodes, err := jig.ListNodesWithEndpoint() // Only nodes with an endpoint should be advertising the IP
			framework.ExpectNoError(err)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			for _, c := range frrContainers {
				validateService(cs, svc, epNodes, c)
			}
		}

		if setProtocoltest == "CheckSpeakerFRRPodRunning" {
			for _, c := range frrContainers {
				frrIsPairedOnPods(cs, c, pairingIPFamily)
			}
		}
	},
		table.Entry("IPV4 - ExternalTrafficPolicyCluster", "ipv4", "ExternalTrafficPolicyCluster", []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
		table.Entry("IPV4 - ExternalTrafficPolicyLocal", "ipv4", "ExternalTrafficPolicyLocal", []string{v4PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV4 - FRR running in the speaker POD", "ipv4", "CheckSpeakerFRRPodRunning", []string{v4PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV6 - ExternalTrafficPolicyCluster", "ipv6", "ExternalTrafficPolicyCluster", []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
		table.Entry("IPV6 - ExternalTrafficPolicyLocal", "ipv6", "ExternalTrafficPolicyLocal", []string{v6PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("IPV6 - FRR running in the speaker POD", "ipv6", "CheckSpeakerFRRPodRunning", []string{v6PoolAddresses}, testservice.TrafficPolicyLocal),
		table.Entry("DUALSTACK - ExternalTrafficPolicyCluster", "dual", "ExternalTrafficPolicyCluster", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.DualStack(svc)
			}),
		table.Entry("DUALSTACK - ExternalTrafficPolicyLocal", "dual", "ExternalTrafficPolicyLocal", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyLocal(svc)
				testservice.DualStack(svc)
			}),
		table.Entry("DUALSTACK - ExternalTrafficPolicyCluster - force V6 only", "dual", "ExternalTrafficPolicyCluster", []string{v4PoolAddresses, v6PoolAddresses},
			func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
				testservice.ForceV6(svc)
			}),
	)

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods []*corev1.Pod

		ginkgo.BeforeEach(func() {
			pods, err := cs.CoreV1().Pods(testNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/component=controller",
			})
			framework.ExpectNoError(err)
			framework.ExpectEqual(len(pods.Items), 1, "Expected one controller pod")
			controllerPod = &pods.Items[0]
			speakerPods = getSpeakerPods(cs)
		})

		table.DescribeTable("should be exposed by the controller", func(ipFamily, poolAddress string, addressTotal int) {
			poolName := "bgp-test"

			var peerAddrs []string
			for _, c := range frrContainers {
				address := c.Ipv4
				if ipFamily == "ipv6" {
					address = c.Ipv6
				}
				peerAddrs = append(peerAddrs, address+fmt.Sprintf(":%d", c.RouterConfig.BGPPort))
			}

			configData := config.File{
				Pools: []config.AddressPool{
					{
						Name:      poolName,
						Protocol:  config.BGP,
						Addresses: []string{poolAddress},
					},
				},
				Peers: peersForContainers(frrContainers, ipFamily),
			}
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c, ipFamily)
			}

			err := configUpdater.Update(configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c, ipFamily)
			}

			ginkgo.By("checking the metrics when no service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, testNameSpace)
				if err != nil {
					return err
				}
				err = validateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = validateGaugeValue(addressTotal, "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(BeNil())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, testNameSpace)
					if err != nil {
						return err
					}
					for _, addr := range peerAddrs {
						err = validateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}
						err = validateGaugeValue(0, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}

			ginkgo.By("creating a service")
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster) // Is a sleep required here?
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ginkgo.By("checking the metrics when a service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, testNameSpace)
				if err != nil {
					return err
				}
				err = validateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(BeNil())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, testNameSpace)
					if err != nil {
						return err
					}
					for _, addr := range peerAddrs {
						err = validateGaugeValue(1, "metallb_bgp_session_up", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateGaugeValue(1, "metallb_bgp_announced_prefixes_total", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bgp_updates_total", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}
					}

					err = validateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": speaker.Spec.NodeName, "protocol": "bgp", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
					if err != nil {
						return err
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}
		},
			table.Entry("IPV4 - Checking service", "ipv4", v4PoolAddresses, 256),
			table.Entry("IPV6 - Checking service", "ipv6", v6PoolAddresses, 16))
	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := configUpdater.Clean()
			framework.ExpectNoError(err)
		})

		table.DescribeTable("set different AddressPools ranges modes", func(addressPools []config.AddressPool, pairingFamily string, tweak testservice.Tweak) {
			configData := config.File{
				Peers: peersForContainers(frrContainers, pairingFamily),
				Pools: addressPools,
			}
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c, pairingFamily)
			}

			err := configUpdater.Update(configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}

			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			for _, i := range svc.Status.LoadBalancer.Ingress {
				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := e2eservice.GetIngressPoint(&i)
				err = validateIPInRange(addressPools, ingressIP)
				framework.ExpectNoError(err)
			}

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		},
			table.Entry("IPV4 - test AddressPool defined by address range", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"192.168.10.0-192.168.10.18",
					},
				}}, "ipv4", testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV4 - test AddressPool defined by network prefix", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"192.168.10.0/24",
					},
				}}, "ipv4", testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV6 - test AddressPool defined by address range", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
					},
				}}, "ipv6", testservice.TrafficPolicyCluster,
			),
			table.Entry("IPV6 - test AddressPool defined by network prefix", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"fc00:f853:0ccd:e799::/124",
					},
				}}, "ipv6", testservice.TrafficPolicyCluster,
			),
			table.Entry("DUALSTACK - test AddressPool defined by address range", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"192.168.10.0-192.168.10.18",
						"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
					},
				}}, "dual", testservice.TrafficPolicyCluster,
			),
			table.Entry("DUALSTACK - test AddressPool defined by network prefix", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"192.168.10.0/24",
						"fc00:f853:0ccd:e799::/124",
					},
				}}, "dual", testservice.TrafficPolicyCluster,
			),
		)
	})

	ginkgo.Context("BFD", func() {
		table.DescribeTable("should work with the given bfd profile", func(bfd config.BfdProfile, pairingFamily string, poolAddresses []string, tweak testservice.Tweak) {
			configData := config.File{
				Pools: []config.AddressPool{
					{
						Name:      "bfd-test",
						Protocol:  config.BGP,
						Addresses: poolAddresses,
					},
				},
				Peers:       withBFD(peersForContainers(frrContainers, pairingFamily), bfd.Name),
				BFDProfiles: []config.BfdProfile{bfd},
			}
			err := configUpdater.Update(configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = true
				})
			}

			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", tweak)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}
			for _, c := range frrContainers {
				validateService(cs, svc, allNodes.Items, c)
			}

			Eventually(func() error {
				for _, c := range frrContainers {
					bfdPeers, err := frr.BFDPeers(c.Executor)
					if err != nil {
						return err
					}
					toCompare := config.BFDProfileWithDefaults(bfd)
					err = frr.BFDPeersMatchNodes(allNodes.Items, bfdPeers, pairingFamily)
					if err != nil {
						return err
					}
					for _, peerConfig := range bfdPeers {
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
				config.BfdProfile{
					Name: "bar",
				}, "ipv4", []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV4 - full params",
				config.BfdProfile{
					Name:             "full1",
					ReceiveInterval:  uint32Ptr(60),
					TransmitInterval: uint32Ptr(61),
					EchoInterval:     uint32Ptr(62),
					EchoMode:         boolPtr(false),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv4", []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV4 - echo mode enabled",
				config.BfdProfile{
					Name:             "echo",
					ReceiveInterval:  uint32Ptr(80),
					TransmitInterval: uint32Ptr(81),
					EchoInterval:     uint32Ptr(82),
					EchoMode:         boolPtr(true),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv4", []string{v4PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - default",
				config.BfdProfile{
					Name: "bar",
				}, "ipv6", []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - full params",
				config.BfdProfile{
					Name:             "full1",
					ReceiveInterval:  uint32Ptr(60),
					TransmitInterval: uint32Ptr(61),
					EchoInterval:     uint32Ptr(62),
					EchoMode:         boolPtr(false),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv6", []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("IPV6 - echo mode enabled",
				config.BfdProfile{
					Name:             "echo",
					ReceiveInterval:  uint32Ptr(80),
					TransmitInterval: uint32Ptr(81),
					EchoInterval:     uint32Ptr(82),
					EchoMode:         boolPtr(true),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv6", []string{v6PoolAddresses}, testservice.TrafficPolicyCluster),
			table.Entry("DUALSTACK - full params",
				config.BfdProfile{
					Name:             "full1",
					ReceiveInterval:  uint32Ptr(60),
					TransmitInterval: uint32Ptr(61),
					EchoInterval:     uint32Ptr(62),
					EchoMode:         boolPtr(false),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "dual", []string{v4PoolAddresses, v6PoolAddresses}, func(svc *corev1.Service) {
					testservice.TrafficPolicyCluster(svc)
					testservice.DualStack(svc)
				}),
		)

		table.DescribeTable("metrics", func(bfd config.BfdProfile, pairingFamily string, poolAddresses []string) {
			configData := config.File{
				Pools: []config.AddressPool{
					{
						Name:      "bfd-test",
						Protocol:  config.BGP,
						Addresses: poolAddresses,
					},
				},
				Peers:       withBFD(peersForContainers(frrContainers, pairingFamily), bfd.Name),
				BFDProfiles: []config.BfdProfile{bfd},
			}
			err := configUpdater.Update(configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = true
				})
			}

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c, pairingFamily)
			}

			ginkgo.By("checking metrics")
			pods, err := cs.CoreV1().Pods(testNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/component=controller",
			})
			framework.ExpectNoError(err)
			framework.ExpectEqual(len(pods.Items), 1, "Expected one controller pod")
			controllerPod := &pods.Items[0]
			speakerPods := getSpeakerPods(cs)

			var peerAddrs []string
			for _, c := range frrContainers {
				address := c.Ipv4
				if pairingFamily == "ipv6" {
					address = c.Ipv6
				}
				peerAddrs = append(peerAddrs, address)
			}

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, testNameSpace)
					if err != nil {
						return err
					}

					for _, addr := range peerAddrs {
						err = validateGaugeValue(1, "metallb_bfd_session_up", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bfd_control_packet_input", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bfd_control_packet_output", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateGaugeValue(0, "metallb_bfd_session_down_events", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bfd_session_up_events", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bfd_zebra_notifications", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						if bfd.EchoMode != nil && *bfd.EchoMode {
							err = validateCounterValue(1, "metallb_bfd_echo_packet_input", map[string]string{"peer": addr}, speakerMetrics)
							if err != nil {
								return err
							}

							err = validateCounterValue(1, "metallb_bfd_echo_packet_output", map[string]string{"peer": addr}, speakerMetrics)
							if err != nil {
								return err
							}
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}

			ginkgo.By("disabling BFD in external FRR containers")
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c, pairingFamily, func(container *frrcontainer.FRR) {
					container.NeighborConfig.BFDEnabled = false
				})
			}

			ginkgo.By("validating session down metrics")
			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker, testNameSpace)
					if err != nil {
						return err
					}

					for _, addr := range peerAddrs {
						err = validateGaugeValue(0, "metallb_bfd_session_up", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}

						err = validateCounterValue(1, "metallb_bfd_session_down_events", map[string]string{"peer": addr}, speakerMetrics)
						if err != nil {
							return err
						}
					}
					return nil
				}, 2*time.Minute, 1*time.Second).Should(BeNil())
			}
		},
			table.Entry("IPV4 - default",
				config.BfdProfile{
					Name: "bar",
				}, "ipv4", []string{v4PoolAddresses}),
			table.Entry("IPV4 - echo mode enabled",
				config.BfdProfile{
					Name:             "echo",
					ReceiveInterval:  uint32Ptr(80),
					TransmitInterval: uint32Ptr(81),
					EchoInterval:     uint32Ptr(82),
					EchoMode:         boolPtr(true),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv4", []string{v4PoolAddresses}),
			table.Entry("IPV6 - default",
				config.BfdProfile{
					Name: "bar",
				}, "ipv6", []string{v6PoolAddresses}),
			table.Entry("IPV6 - echo mode enabled",
				config.BfdProfile{
					Name:             "echo",
					ReceiveInterval:  uint32Ptr(80),
					TransmitInterval: uint32Ptr(81),
					EchoInterval:     uint32Ptr(82),
					EchoMode:         boolPtr(true),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "ipv6", []string{v6PoolAddresses}),
			table.Entry("DUALSTACK - full params",
				config.BfdProfile{
					Name:             "full1",
					ReceiveInterval:  uint32Ptr(60),
					TransmitInterval: uint32Ptr(61),
					EchoInterval:     uint32Ptr(62),
					EchoMode:         boolPtr(false),
					PassiveMode:      boolPtr(false),
					MinimumTTL:       uint32Ptr(254),
				}, "dual", []string{v4PoolAddresses, v6PoolAddresses}),
		)
	})

	ginkgo.Context("validate configuration changes", func() {
		table.DescribeTable("should work after subsequent configuration updates", func(addressRange string, ipFamily string) {
			var services []*corev1.Service
			var servicesIngressIP []string
			var pools []config.AddressPool

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for i := 0; i < 2; i++ {
				ginkgo.By(fmt.Sprintf("configure addresspool number %d", i+1))
				firstIP, err := getIPFromRangeByIndex(addressRange, i*10+1)
				framework.ExpectNoError(err)
				lastIP, err := getIPFromRangeByIndex(addressRange, i*10+10)
				framework.ExpectNoError(err)
				addressesRange := fmt.Sprintf("%s-%s", firstIP, lastIP)
				pool := config.AddressPool{
					Name:     fmt.Sprintf("test-addresspool%d", i+1),
					Protocol: config.BGP,
					Addresses: []string{
						addressesRange,
					},
				}
				pools = append(pools, pool)

				configData := config.File{
					Pools: pools,
					Peers: peersForContainers(frrContainers, ipFamily),
				}

				for _, c := range frrContainers {
					pairExternalFRRWithNodes(cs, c, ipFamily)
				}

				err = configUpdater.Update(configData)
				framework.ExpectNoError(err)

				for _, c := range frrContainers {
					validateFRRPeeredWithNodes(cs, c, ipFamily)
				}

				ginkgo.By(fmt.Sprintf("configure service number %d", i+1))
				svc, _ := createServiceWithBackend(cs, f.Namespace.Name, fmt.Sprintf("svc%d", i+1), testservice.TrafficPolicyCluster, func(svc *corev1.Service) {
					svc.Annotations = map[string]string{"metallb.universe.tf/address-pool": fmt.Sprintf("test-addresspool%d", i+1)}
				})

				defer func() {
					err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
					framework.ExpectNoError(err)
				}()

				ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
				ingressIP := e2eservice.GetIngressPoint(
					&svc.Status.LoadBalancer.Ingress[0])
				err = validateIPInRange([]config.AddressPool{pool}, ingressIP)
				framework.ExpectNoError(err)

				services = append(services, svc)
				servicesIngressIP = append(servicesIngressIP, ingressIP)

				for j := 0; j <= i; j++ {
					ginkgo.By(fmt.Sprintf("validate service %d IP didn't change", j+1))
					ip := e2eservice.GetIngressPoint(&services[j].Status.LoadBalancer.Ingress[0])
					framework.ExpectEqual(ip, servicesIngressIP[j])

					ginkgo.By(fmt.Sprintf("checking connectivity of service %d to its external VIP", j+1))
					for _, c := range frrContainers {
						validateService(cs, svc, allNodes.Items, c)
					}
				}
			}
		},
			table.Entry("IPV4", "192.168.10.0/24", "ipv4"),
			table.Entry("IPV6", "fc00:f853:0ccd:e799::/116", "ipv6"))

		table.DescribeTable("configure peers one by one and validate FRR paired with nodes", func(ipFamily string) {
			for i, c := range frrContainers {
				ginkgo.By("configure peer")

				configData := config.File{
					Peers: peersForContainers([]*frrcontainer.FRR{c}, ipFamily),
				}
				err := configUpdater.Update(configData)
				framework.ExpectNoError(err)

				pairExternalFRRWithNodes(cs, c, ipFamily)

				validateFRRPeeredWithNodes(cs, frrContainers[i], ipFamily)
			}
		},
			table.Entry("IPV4", "ipv4"),
			table.Entry("IPV6", "ipv6"))

		table.DescribeTable("configure bgp community and verify it gets propagated",
			func(addressPools []config.AddressPool, ipFamily string) {
				configData := config.File{
					Peers: peersForContainers(frrContainers, ipFamily),
					Pools: addressPools,
				}
				for _, c := range frrContainers {
					pairExternalFRRWithNodes(cs, c, ipFamily)
				}

				err := configUpdater.Update(configData)
				framework.ExpectNoError(err)

				for _, c := range frrContainers {
					validateFRRPeeredWithNodes(cs, c, ipFamily)
				}

				svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)

				defer func() {
					err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
					framework.ExpectNoError(err)
				}()

				for _, i := range svc.Status.LoadBalancer.Ingress {
					ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
					ingressIP := e2eservice.GetIngressPoint(&i)
					err = validateIPInRange(addressPools, ingressIP)
					framework.ExpectNoError(err)
				}

				allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				framework.ExpectNoError(err)

				for _, c := range frrContainers {
					validateService(cs, svc, allNodes.Items, c)
					Eventually(func() error {
						return frr.ContainsCommunity(c, "no-advertise")
					}, 4*time.Minute, 1*time.Second).Should(BeNil())
				}
			},
			table.Entry("IPV4", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"192.168.10.0/24",
					},
					BGPAdvertisements: []config.BgpAdvertisement{
						{
							Communities: []string{
								CommunityNoAdv,
							},
						},
					},
				}}, "ipv4"),
			table.Entry("IPV6", []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
					},
					BGPAdvertisements: []config.BgpAdvertisement{
						{
							Communities: []string{
								CommunityNoAdv,
							},
						},
					},
				}}, "ipv6"))

		table.DescribeTable("configure bgp local-preference and verify it gets propagated",
			func(poolAddresses []string, ipFamily string, localPref uint32) {
				configData := config.File{
					Pools: []config.AddressPool{
						{
							Name:      "bgp-test",
							Protocol:  config.BGP,
							Addresses: poolAddresses,
							BGPAdvertisements: []config.BgpAdvertisement{
								{
									LocalPref: uint32Ptr(localPref),
								},
							},
						},
					},
					Peers: peersForContainers(frrContainers, ipFamily),
				}
				for _, c := range frrContainers {
					pairExternalFRRWithNodes(cs, c, ipFamily)
				}

				err := configUpdater.Update(configData)
				framework.ExpectNoError(err)

				for _, c := range frrContainers {
					validateFRRPeeredWithNodes(cs, c, ipFamily)
				}

				svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)

				defer func() {
					err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
					framework.ExpectNoError(err)
				}()

				allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
				framework.ExpectNoError(err)

				for _, c := range frrContainers {
					validateService(cs, svc, allNodes.Items, c)
				}
				// LocalPref check is only valid for iBGP sessions
				for _, c := range frrContainers {
					if c.Name == "frrIBGP" {
						Eventually(func() error {
							return frr.RoutesMatchLocalPref(c, ipFamily, localPref)
						}, 4*time.Minute, 1*time.Second).Should(BeNil())
					}
				}
			},
			table.Entry("IPV4", []string{v4PoolAddresses}, "ipv4", IPLocalPref),
			table.Entry("IPV6", []string{v6PoolAddresses}, "ipv6", IPLocalPref))
	})

	table.DescribeTable("MetalLB FRR rejects any routes advertised by any neighbor", func(addressesRange, pairingIPFamily, toInject string) {
		configData := config.File{
			Pools: []config.AddressPool{
				{
					Name:     "bgp-test",
					Protocol: config.BGP,
					Addresses: []string{
						addressesRange,
					},
				},
			},
			Peers: peersForContainers(frrContainers, pairingIPFamily),
		}
		neighborAnnounce := func(frr *frrcontainer.FRR) {
			frr.NeighborConfig.ToAdvertise = toInject
		}

		for _, c := range frrContainers {
			pairExternalFRRWithNodes(cs, c, pairingIPFamily, neighborAnnounce)
		}

		err := configUpdater.Update(configData)
		framework.ExpectNoError(err)

		for _, c := range frrContainers {
			validateFRRPeeredWithNodes(cs, c, pairingIPFamily)
		}
		speakerPods := getSpeakerPods(cs)
		checkRoutesInjected := func() error {
			for _, pod := range speakerPods {
				podExec := executor.ForPod(pod.Namespace, pod.Name, "frr")
				routes, frrRoutesV6, err := frr.Routes(podExec)
				framework.ExpectNoError(err)

				if pairingIPFamily == "ipv6" {
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
		svc, _ := createServiceWithBackend(cs, f.Namespace.Name, "external-local-lb")

		defer func() {
			err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		Consistently(checkRoutesInjected, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

	},
		table.Entry("IPV4", "192.168.10.0/24", "ipv4", "172.16.1.10/32"),
		table.Entry("IPV6", "fc00:f853:0ccd:e799::/116", "ipv6", "fc00:f853:ccd:e800::1/128"),
	)

})

func createServiceWithBackend(cs clientset.Interface, namespace string, jigName string, tweak ...func(svc *corev1.Service)) (*corev1.Service, *e2eservice.TestJig) {
	var svc *corev1.Service
	var err error

	jig := e2eservice.NewTestJig(cs, namespace, jigName)
	timeout := e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
	svc, err = jig.CreateLoadBalancerService(timeout, func(svc *corev1.Service) {
		tweakServicePort(svc)
		for _, f := range tweak {
			f(svc)
		}
	})

	framework.ExpectNoError(err)
	_, err = jig.Run(func(rc *corev1.ReplicationController) {
		tweakRCPort(rc)
	})
	framework.ExpectNoError(err)
	return svc, jig
}

func validateGaugeValue(expectedValue int, metricName string, labels map[string]string, allMetrics []map[string]*dto.MetricFamily) error {
	found := false
	for _, m := range allMetrics {
		value, err := metrics.GaugeForLabels(metricName, labels, m)
		if err != nil {
			continue
		}
		if value != expectedValue {
			return fmt.Errorf("invalid value %d for %s, expecting %d", value, metricName, expectedValue)
		}
		found = true
	}

	if !found {
		return fmt.Errorf("metric %s not found", metricName)
	}
	return nil

}

func validateCounterValue(expectedMax int, metricName string, labels map[string]string, allMetrics []map[string]*dto.MetricFamily) error {
	var err error
	var value int
	found := false
	for _, m := range allMetrics {
		value, err = metrics.CounterForLabels(metricName, labels, m)
		if err != nil {
			continue
		}
		found = true
		if value < expectedMax {
			return fmt.Errorf("invalid value %d for %s, expecting more than %d", value, metricName, expectedMax)
		}
	}

	if !found {
		return fmt.Errorf("metric %s not found", metricName)
	}
	return nil
}

func pairExternalFRRWithNodes(cs clientset.Interface, c *frrcontainer.FRR, ipFamily string, modifiers ...func(c *frrcontainer.FRR)) {
	config := *c
	for _, m := range modifiers {
		m(&config)
	}
	bgpConfig, err := frrconfig.BGPPeersForAllNodes(cs, config.NeighborConfig, config.RouterConfig, ipFamily)
	framework.ExpectNoError(err)

	err = c.UpdateBGPConfigFile(bgpConfig)
	framework.ExpectNoError(err)
}

func validateFRRPeeredWithNodes(cs clientset.Interface, c *frrcontainer.FRR, ipFamily string) {
	allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	framework.ExpectNoError(err)

	ginkgo.By(fmt.Sprintf("checking all nodes are peered with the frr instance %s", c.Name))
	Eventually(func() error {
		neighbors, err := frr.NeighborsInfo(c)
		framework.ExpectNoError(err)
		err = frr.NeighborsMatchNodes(allNodes.Items, neighbors, ipFamily)
		return err
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func validateService(cs clientset.Interface, svc *corev1.Service, nodes []corev1.Node, c *frrcontainer.FRR) {
	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))

	if len(svc.Status.LoadBalancer.Ingress) == 2 {
		ip1 := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
		ip2 := net.ParseIP(svc.Status.LoadBalancer.Ingress[1].IP)
		framework.ExpectNotEqual(ip1.To4(), ip2.To4())
	}
	for _, ip := range svc.Status.LoadBalancer.Ingress {

		ingressIP := e2eservice.GetIngressPoint(&ip)
		hostport := net.JoinHostPort(ingressIP, port)
		address := fmt.Sprintf("http://%s/", hostport)

		Eventually(func() error {
			err := wgetRetry(address, c)
			if err != nil {
				return err
			}

			frrRoutesV4, frrRoutesV6, err := frr.Routes(c)
			if err != nil {
				return err
			}
			serviceIPFamily := "ipv4"
			frrRoutes, ok := frrRoutesV4[ingressIP]
			if !ok {
				frrRoutes, ok = frrRoutesV6[ingressIP]
				serviceIPFamily = "ipv6"
			}
			if !ok {
				return fmt.Errorf("%s not found in frr routes %v %v", ingressIP, frrRoutesV4, frrRoutesV6)
			}

			err = frr.RoutesMatchNodes(nodes, frrRoutes, serviceIPFamily)
			if err != nil {
				return err
			}

			advertised := routes.ForIP(ingressIP, c)
			err = routes.MatchNodes(nodes, advertised, serviceIPFamily)
			if err != nil {
				return err
			}

			return nil
		}, 4*time.Minute, 1*time.Second).Should(BeNil())
	}
}

func frrIsPairedOnPods(cs clientset.Interface, n *frrcontainer.FRR, ipFamily string) {
	pods := getSpeakerPods(cs)
	podExecutor := executor.ForPod(testNameSpace, pods[0].Name, "frr")

	Eventually(func() error {
		addresses := n.AddressesForFamily(ipFamily)
		for _, address := range addresses {
			toParse, err := podExecutor.Exec("vtysh", "-c", fmt.Sprintf("show bgp neighbor %s json", address))
			if err != nil {
				return err
			}
			res, err := frr.NeighborConnected(toParse)
			if err != nil {
				return err
			}
			if res != true {
				return fmt.Errorf("expecting neighbor %s to be connected", n.Ipv4)
			}
		}
		return nil
	}, 4*time.Minute, 1*time.Second).Should(BeNil())
}

func createFRRContainers(c ...frrcontainer.Config) ([]*frrcontainer.FRR, error) {
	m := sync.Mutex{}
	frrContainers = make([]*frrcontainer.FRR, 0)
	g := new(errgroup.Group)
	for _, conf := range c {
		conf := conf
		g.Go(func() error {
			toFind := map[string]bool{
				"zebra":    true,
				"watchfrr": true,
				"bgpd":     true,
				"bfdd":     true,
			}
			c, err := frrcontainer.Start(conf)
			if c != nil {
				err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
					daemons, err := frr.Daemons(c)
					if err != nil {
						return false, err
					}
					for _, d := range daemons {
						delete(toFind, d)
					}
					if len(toFind) > 0 {
						return false, nil
					}
					return true, nil
				})
				m.Lock()
				defer m.Unlock()
				frrContainers = append(frrContainers, c)
			}
			if err != nil {
				return errors.Wrapf(err, "Failed to wait for daemons %v", toFind)
			}
			return nil
		})
	}
	err := g.Wait()

	return frrContainers, err
}

func stopFRRContainers(containers []*frrcontainer.FRR) error {
	g := new(errgroup.Group)
	for _, c := range containers {
		c := c
		g.Go(func() error {
			err := c.Stop()
			return err
		})
	}

	return g.Wait()
}

func peersForContainers(containers []*frrcontainer.FRR, ipFamily string) []config.Peer {
	var peers []config.Peer

	for i, c := range containers {
		addresses := c.AddressesForFamily(ipFamily)
		holdTime := ""
		if i > 0 {
			holdTime = fmt.Sprintf("%ds", i*180)
		}
		for _, address := range addresses {
			peers = append(peers, config.Peer{
				Addr:     address,
				ASN:      c.RouterConfig.ASN,
				MyASN:    c.NeighborConfig.ASN,
				Port:     c.RouterConfig.BGPPort,
				RouterID: fmt.Sprintf(baseRouterID, i),
				Password: c.RouterConfig.Password,
				HoldTime: holdTime,
			})
		}
	}
	return peers
}

func withBFD(peers []config.Peer, bfdProfile string) []config.Peer {
	for i := range peers {
		peers[i].BFDProfile = bfdProfile
	}
	return peers
}

func checkBFDConfigPropagated(nodeConfig config.BfdProfile, peerConfig bgpfrr.BFDPeer) error {
	if peerConfig.Status != "up" {
		return fmt.Errorf("Peer status not up")
	}
	if peerConfig.RemoteReceiveInterval != int(*nodeConfig.ReceiveInterval) {
		return fmt.Errorf("RemoteReceiveInterval: expecting %d, got %d", *nodeConfig.ReceiveInterval, peerConfig.RemoteReceiveInterval)
	}
	if peerConfig.RemoteTransmitInterval != int(*nodeConfig.TransmitInterval) {
		return fmt.Errorf("RemoteTransmitInterval: expecting %d, got %d", *nodeConfig.TransmitInterval, peerConfig.RemoteTransmitInterval)
	}
	if peerConfig.RemoteEchoInterval != int(*nodeConfig.EchoInterval) {
		return fmt.Errorf("EchoInterval: expecting %d, got %d", *nodeConfig.EchoInterval, peerConfig.RemoteEchoInterval)
	}
	return nil
}

func getSpeakerPods(cs clientset.Interface) []*corev1.Pod {
	speakers, err := cs.CoreV1().Pods(testNameSpace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=speaker",
	})
	framework.ExpectNoError(err)
	framework.ExpectNotEqual(len(speakers.Items), 0, "No speaker pods found")
	speakerPods := make([]*corev1.Pod, 0)
	for _, item := range speakers.Items {
		i := item
		speakerPods = append(speakerPods, &i)
	}
	return speakerPods
}

func uint32Ptr(n uint32) *uint32 {
	return &n
}

func boolPtr(b bool) *bool {
	return &b
}
