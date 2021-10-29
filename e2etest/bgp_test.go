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
	"time"

	"go.universe.tf/metallb/e2etest/pkg/metrics"
	"go.universe.tf/metallb/e2etest/pkg/routes"

	"golang.org/x/sync/errgroup"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"go.universe.tf/metallb/e2etest/pkg/frr"
	frrconfig "go.universe.tf/metallb/e2etest/pkg/frr/config"
	frrcontainer "go.universe.tf/metallb/e2etest/pkg/frr/container"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

const (
	frrIBGP = "frr-iBGP"
	frrEBGP = "frr-eBGP"
	IBGPAsn = 64512
	EBGPAsn = 64513
)

var (
	frrContainers []*frrcontainer.FRR
)

type containerConfig struct {
	name string
	nc   frrconfig.NeighborConfig
	rc   frrconfig.RouterConfig
}

var _ = ginkgo.Describe("BGP", func() {
	f := framework.NewDefaultFramework("bgp")
	var cs clientset.Interface
	containersConf := []containerConfig{
		{
			name: frrIBGP,
			nc: frrconfig.NeighborConfig{
				ASN:      IBGPAsn,
				Password: "ibgp-test",
			},
			rc: frrconfig.RouterConfig{
				ASN:      IBGPAsn,
				BGPPort:  179,
				RouterID: "10.10.10.10",
				Password: "ibgp-test",
				HoldTime: "180s",
			},
		},
		{
			name: frrEBGP,
			nc: frrconfig.NeighborConfig{
				ASN:      EBGPAsn,
				Password: "ebgp-test",
			},
			rc: frrconfig.RouterConfig{
				ASN:      IBGPAsn,
				BGPPort:  180,
				RouterID: "11.11.11.11",
				Password: "ebgp-test",
			},
		},
	}
	ginkgo.BeforeEach(func() {
		var err error
		// TODO: When adding support for bgp + IPv6 need to validate hostIPv6 as well
		if containersNetwork == "host" {
			if net.ParseIP(hostIPv4) == nil {
				framework.Fail("Invalid hostIPv4")
			}
		}
		cs = f.ClientSet
		frrContainers, err = createFRRContainers(containersConf)
		framework.ExpectNoError(err)
	})

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := updateConfigMap(cs, configFile{})
		framework.ExpectNoError(err)

		err = stopFRRContainers(frrContainers)
		framework.ExpectNoError(err)

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.Context("type=Loadbalancer", func() {
		var allNodes *corev1.NodeList
		ginkgo.BeforeEach(func() {
			var peers []peer
			for _, c := range frrContainers {
				peers = append(peers, peer{
					Addr:     c.Ipv4,
					ASN:      c.RouterConfig.ASN,
					MyASN:    c.NeighborConfig.ASN,
					Port:     c.RouterConfig.BGPPort,
					RouterID: c.RouterConfig.RouterID,
					Password: c.RouterConfig.Password,
					HoldTime: c.RouterConfig.HoldTime,
				})
			}
			configData := configFile{
				Pools: []addressPool{
					{
						Name:     "bgp-test",
						Protocol: BGP,
						Addresses: []string{
							"192.168.10.0/24",
							"fc00:f853:0ccd:e799::/124",
						},
					},
				},
				Peers: peers,
			}
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c)
			}

			err := updateConfigMap(cs, configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c)
			}

			allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Cluster", func() {
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			for _, c := range frrContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Local", func() {
			svc, jig := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeLocal)
			err := jig.Scale(2)
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
		})

		ginkgo.It("would check that FRR is running in the speaker", func() {
			for _, c := range frrContainers {
				paired := frrIsPairedOnPods(cs, c)
				framework.ExpectEqual(paired, true)
			}
		})
	})

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods []*corev1.Pod

		ginkgo.BeforeEach(func() {
			pods, err := cs.CoreV1().Pods("metallb-system").List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=controller",
			})
			framework.ExpectNoError(err)
			framework.ExpectEqual(len(pods.Items), 1, "More than one controller found")
			controllerPod = &pods.Items[0]

			speakers, err := cs.CoreV1().Pods("metallb-system").List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=speaker",
			})
			framework.ExpectNoError(err)
			speakerPods = make([]*corev1.Pod, 0)
			for _, item := range speakers.Items {
				speakerPods = append(speakerPods, &item)
			}
		})

		ginkgo.It("should be exposed by the controller", func() {
			// TODO: The peer address will require update to add support for bgp + IPv6
			poolName := "bgp-test"

			var peers []peer
			var peerAddrs []string
			for _, c := range frrContainers {
				peers = append(peers, peer{
					Addr:     c.Ipv4,
					ASN:      c.RouterConfig.ASN,
					MyASN:    c.NeighborConfig.ASN,
					Port:     c.RouterConfig.BGPPort,
					RouterID: c.RouterConfig.RouterID,
					Password: c.RouterConfig.Password,
					HoldTime: c.RouterConfig.HoldTime,
				})
				peerAddrs = append(peerAddrs, c.Ipv4+fmt.Sprintf(":%d", c.RouterConfig.BGPPort))
			}

			configData := configFile{
				Pools: []addressPool{
					{
						Name:     poolName,
						Protocol: BGP,
						Addresses: []string{
							"192.168.10.0/24",
							"fc00:f853:0ccd:e799::/124",
						},
					},
				},
				Peers: peers,
			}
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c)
			}

			err := updateConfigMap(cs, configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c)
			}

			ginkgo.By("checking the metrics when no service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod)
				if err != nil {
					return err
				}
				err = validateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = validateGaugeValue(272, "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(BeNil())

			for _, speaker := range speakerPods {
				ginkgo.By(fmt.Sprintf("checking speaker %s", speaker.Name))

				Eventually(func() error {
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker)
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
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster) // Is a sleep required here?
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ginkgo.By("checking the metrics when a service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod)
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
					speakerMetrics, err := metrics.ForPod(controllerPod, speaker)
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
		})
	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := updateConfigMap(cs, configFile{})
			framework.ExpectNoError(err)
		})

		table.DescribeTable("set different AddressPools ranges modes", func(addressPools []addressPool) {
			var peers []peer
			for _, c := range frrContainers {
				peers = append(peers, peer{
					Addr:     c.Ipv4,
					ASN:      c.RouterConfig.ASN,
					MyASN:    c.NeighborConfig.ASN,
					Port:     c.RouterConfig.BGPPort,
					RouterID: c.RouterConfig.RouterID,
					Password: c.RouterConfig.Password,
					HoldTime: c.RouterConfig.HoldTime,
				})
			}
			configData := configFile{
				Peers: peers,
				Pools: addressPools,
			}
			for _, c := range frrContainers {
				pairExternalFRRWithNodes(cs, c)
			}

			err := updateConfigMap(cs, configData)
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateFRRPeeredWithNodes(cs, c)
			}

			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			err = validateIPInRange(addressPools, ingressIP)
			framework.ExpectNoError(err)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			for _, c := range frrContainers {
				validateService(cs, svc, allNodes.Items, c)
			}
		},
			table.Entry("AddressPool defined by network prefix", []addressPool{
				{
					Name:     "bgp-test",
					Protocol: BGP,
					Addresses: []string{
						"192.168.10.0/24",
						"fc00:f853:0ccd:e799::/124",
					},
				},
			}),
			table.Entry("AddressPool defined by address range", []addressPool{
				{
					Name:     "bgp-test",
					Protocol: BGP,
					Addresses: []string{
						"192.168.10.0-192.168.10.18",
						"fc00:f853:0ccd:e799::0-fc00:f853:0ccd:e799::18",
					},
				},
			}),
		)
	})
})

func createServiceWithBackend(cs clientset.Interface, namespace string, policy corev1.ServiceExternalTrafficPolicyType) (*corev1.Service, *e2eservice.TestJig) {
	var svc *corev1.Service
	var err error

	serviceName := "external-local-lb"
	jig := e2eservice.NewTestJig(cs, namespace, serviceName)
	timeout := e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
	svc, err = jig.CreateLoadBalancerService(timeout, func(svc *corev1.Service) {
		tweakServicePort(svc)
		svc.Spec.ExternalTrafficPolicy = policy
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

func pairExternalFRRWithNodes(cs clientset.Interface, c *frrcontainer.FRR) {
	bgpConfig, err := frrconfig.BGPPeersForAllNodes(cs, c.NeighborConfig, c.RouterConfig)
	framework.ExpectNoError(err)

	err = c.UpdateBGPConfigFile(bgpConfig)
	framework.ExpectNoError(err)
}

func validateFRRPeeredWithNodes(cs clientset.Interface, c *frrcontainer.FRR) {
	allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	framework.ExpectNoError(err)

	ginkgo.By(fmt.Sprintf("checking all nodes are peered with the frr instance %s", c.Name))
	Eventually(func() error {
		neighbors, err := frr.NeighborsInfo(c)
		framework.ExpectNoError(err)
		err = frr.NeighborsMatchNodes(allNodes.Items, neighbors)
		return err
	}, 2*time.Minute, 1*time.Second).Should(BeNil())
}

func validateService(cs clientset.Interface, svc *corev1.Service, nodes []corev1.Node, c *frrcontainer.FRR) {
	port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
	ingressIP := e2eservice.GetIngressPoint(
		&svc.Status.LoadBalancer.Ingress[0])

	hostport := net.JoinHostPort(ingressIP, port)
	address := fmt.Sprintf("http://%s/", hostport)

	Eventually(func() error {
		err := wgetRetry(address, c)
		if err != nil {
			return err
		}

		advertised := routes.ForIP(ingressIP, c)
		err = routes.MatchNodes(nodes, advertised)
		if err != nil {
			return err
		}

		frrRoutes, _, err := frr.Routes(c)
		if err != nil {
			return err
		}

		routes, ok := frrRoutes[ingressIP]
		if !ok {
			return fmt.Errorf("%s not found in frr routes %v", ingressIP, frrRoutes)
		}

		err = frr.RoutesMatchNodes(nodes, routes)
		if err != nil {
			return err
		}
		return nil
	}, 2*time.Minute, 1*time.Second).Should(BeNil())
}

func frrIsPairedOnPods(cs clientset.Interface, n *frrcontainer.FRR) bool {
	pods, err := cs.CoreV1().Pods("metallb-system").List(context.Background(), metav1.ListOptions{
		LabelSelector: "component=speaker",
	})
	framework.ExpectNoError(err)
	toParse, err := framework.RunKubectl("metallb-system", "exec", pods.Items[0].Name, "-c", "frr", "--", "vtysh", "-c", fmt.Sprintf("show bgp view %d neighbor %s json", n.NeighborConfig.ASN, n.Ipv4))
	framework.ExpectNoError(err)
	res, err := frr.NeighborConnected(toParse)
	framework.ExpectNoError(err)
	return res
}

func createFRRContainers(c []containerConfig) ([]*frrcontainer.FRR, error) {
	frrContainers = make([]*frrcontainer.FRR, 0)
	g := new(errgroup.Group)
	for _, conf := range c {
		conf := conf
		g.Go(func() error {
			c, err := frrcontainer.Start(conf.name, conf.nc, conf.rc, containersNetwork, hostIPv4, hostIPv6)
			if c != nil {
				frrContainers = append(frrContainers, c)
			}
			return err
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
