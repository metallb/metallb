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

package l2tests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/mac"
	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/metrics"
	"go.universe.tf/metallb/e2etest/pkg/service"
	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	"go.universe.tf/metallb/e2etest/pkg/wget"
	internalconfig "go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	pkgerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

var (
	ConfigUpdater    config.Updater
	IPV4ServiceRange string
	IPV6ServiceRange string
)

var _ = ginkgo.Describe("L2", func() {
	f := framework.NewDefaultFramework("l2")
	var loadBalancerCreateTimeout time.Duration
	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
		loadBalancerCreateTimeout = e2eservice.GetServiceLoadBalancerCreationTimeout(cs)

		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			k8s.DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.Context("type=Loadbalancer", func() {
		ginkgo.BeforeEach(func() {
			configData := config.File{
				Pools: []config.AddressPool{
					{
						Name:     "l2-test",
						Protocol: config.Layer2,
						Addresses: []string{
							IPV4ServiceRange,
							IPV6ServiceRange,
						},
					},
				},
			}
			err := ConfigUpdater.Update(configData)
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Cluster", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err := wget.Do(address, executor.Host)
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Local", func() {
			svc, jig := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyLocal)
			err := jig.Scale(5)
			framework.ExpectNoError(err)

			epNodes, err := jig.ListNodesWithEndpoint() // Only nodes with an endpoint could be advertising the IP
			framework.ExpectNoError(err)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)

			ginkgo.By(fmt.Sprintf("checking connectivity to its external VIP %s", hostport))
			err = wget.Do(address, executor.Host)
			framework.ExpectNoError(err)

			// Give the speakers enough time to settle and for the announcer to complete its gratuitous.
			gomega.Eventually(func() error {
				advNode, err := advertisingNodeFromMAC(epNodes, ingressIP, executor.Host)
				if err != nil {
					return err
				}

				for i := 0; i < 5; i++ {
					name, err := service.GetEndpointHostName(hostport, executor.Host)
					if err != nil {
						return err
					}

					ginkgo.By(fmt.Sprintf("checking that pod %s is on node %s", name, advNode.Name))
					pod, err := cs.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					if pod.Spec.NodeName != advNode.Name {
						return fmt.Errorf("traffic arrived to a pod on node %s which is not the announcing node %s", pod.Spec.NodeName, advNode.Name)
					}
				}

				return nil
			}, 5*time.Second, 1*time.Second).Should(gomega.BeNil())
		})

	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {

		table.DescribeTable("set different AddressPools ranges modes", func(getAddressPools func() []config.AddressPool) {
			configData := config.File{
				Pools: getAddressPools(),
			}
			err := ConfigUpdater.Update(configData)
			framework.ExpectNoError(err)

			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			err = config.ValidateIPInRange(getAddressPools(), ingressIP)
			framework.ExpectNoError(err)

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err = wget.Do(address, executor.Host)
			framework.ExpectNoError(err)
		},
			table.Entry("AddressPool defined by address range", func() []config.AddressPool {
				return []config.AddressPool{
					{
						Name:     "l2-test",
						Protocol: config.Layer2,
						Addresses: []string{
							IPV4ServiceRange,
							IPV6ServiceRange,
						},
					},
				}
			}),
			table.Entry("AddressPool defined by network prefix", func() []config.AddressPool {
				var ipv4AddressesByCIDR []string
				var ipv6AddressesByCIDR []string

				cidrs, err := internalconfig.ParseCIDR(IPV4ServiceRange)
				framework.ExpectNoError(err)
				for _, cidr := range cidrs {
					ipv4AddressesByCIDR = append(ipv4AddressesByCIDR, cidr.String())
				}

				cidrs, err = internalconfig.ParseCIDR(IPV6ServiceRange)
				framework.ExpectNoError(err)
				for _, cidr := range cidrs {
					ipv6AddressesByCIDR = append(ipv6AddressesByCIDR, cidr.String())
				}

				return []config.AddressPool{
					{
						Name:      "l2-test",
						Protocol:  config.Layer2,
						Addresses: append(ipv4AddressesByCIDR, ipv6AddressesByCIDR...),
					},
				}
			}),
		)
	})

	table.DescribeTable("different services sharing the same ip should advertise from the same node", func(ipRange *string) {
		configData := config.File{
			Pools: []config.AddressPool{
				{
					Name:     "l2-services-same-ip-test",
					Protocol: config.Layer2,
					Addresses: []string{
						IPV4ServiceRange,
						IPV6ServiceRange,
					},
				},
			},
		}
		err := ConfigUpdater.Update(configData)
		framework.ExpectNoError(err)
		namespace := f.Namespace.Name

		jig1 := e2eservice.NewTestJig(cs, namespace, "svca")

		ip, err := config.GetIPFromRangeByIndex(*ipRange, 0)
		framework.ExpectNoError(err)
		svc1, err := jig1.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort)
			svc.Spec.Ports[0].Port = int32(service.TestServicePort)
			svc.Annotations = map[string]string{"metallb.universe.tf/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})

		framework.ExpectNoError(err)

		jig2 := e2eservice.NewTestJig(cs, namespace, "svcb")
		svc2, err := jig2.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort + 1)
			svc.Spec.Ports[0].Port = int32(service.TestServicePort + 1)
			svc.Annotations = map[string]string{"metallb.universe.tf/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})
		framework.ExpectNoError(err)
		defer func() {
			err := cs.CoreV1().Services(svc1.Namespace).Delete(context.TODO(), svc1.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
			err = cs.CoreV1().Services(svc2.Namespace).Delete(context.TODO(), svc2.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}()

		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		framework.ExpectNoError(err)
		_, err = jig1.Run(
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort), fmt.Sprintf("--udp-port=%d", service.TestServicePort)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(service.TestServicePort)
				rc.Spec.Template.Spec.NodeName = nodes.Items[0].Name
			})
		framework.ExpectNoError(err)
		_, err = jig2.Run(
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort+1), fmt.Sprintf("--udp-port=%d", service.TestServicePort+1)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(service.TestServicePort + 1)
				rc.Spec.Template.Spec.NodeName = nodes.Items[1].Name
			})
		framework.ExpectNoError(err)

		gomega.Eventually(func() error {
			events, err := cs.CoreV1().Events(namespace).List(context.Background(), metav1.ListOptions{FieldSelector: "reason=nodeAssigned"})
			if err != nil {
				return err
			}

			var service1Announce, service2Announce string
			for _, e := range events.Items {
				if e.InvolvedObject.Name == svc1.Name {
					service1Announce = e.Message
				}
				if e.InvolvedObject.Name == svc2.Name {
					service2Announce = e.Message
				}
			}
			if service1Announce == "" {
				return errors.New("service1 not announced")
			}
			if service2Announce == "" {
				return errors.New("service2 not announced")
			}
			if service1Announce != service2Announce {
				return fmt.Errorf("Service announced from different nodes %s %s", service1Announce, service2Announce)
			}
			return nil
		}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

	},
		table.Entry("IPV4", &IPV4ServiceRange),
		table.Entry("IPV6", &IPV6ServiceRange))

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods map[string]*corev1.Pod

		ginkgo.BeforeEach(func() {
			var err error
			controllerPod, err = metallb.ControllerPod(cs)
			framework.ExpectNoError(err)

			speakers, err := metallb.SpeakerPods(cs)
			framework.ExpectNoError(err)

			speakerPods = map[string]*corev1.Pod{}
			for _, item := range speakers {
				i := item
				speakerPods[i.Spec.NodeName] = i
			}
		})

		table.DescribeTable("should be exposed by the controller", func(ipFamily string) {
			poolName := "l2-metrics-test"

			configData := config.File{
				Pools: []config.AddressPool{
					{
						Name:     poolName,
						Protocol: config.Layer2,
						Addresses: []string{
							IPV4ServiceRange,
							IPV6ServiceRange,
						},
					},
				},
			}
			poolCount, err := config.PoolCount(configData.Pools[0])
			framework.ExpectNoError(err)

			err = ConfigUpdater.Update(configData)
			framework.ExpectNoError(err)

			ginkgo.By("checking the metrics when no service is added")
			gomega.Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(int(poolCount), "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

			ginkgo.By("creating a service")
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", testservice.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				if !pkgerr.IsNotFound(err) {
					framework.ExpectNoError(err)
				}
			}()

			ginkgo.By("checking the metrics when a service is added")
			gomega.Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				err = metrics.ValidateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			err = mac.RequestAddressResolution(ingressIP, executor.Host)
			framework.ExpectNoError(err)

			ginkgo.By("checking connectivity to its external VIP")
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err = wget.Do(address, executor.Host)
			framework.ExpectNoError(err)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			advNode, err := advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
			framework.ExpectNoError(err)
			advSpeaker, ok := speakerPods[advNode.Name]
			framework.ExpectEqual(ok, true, fmt.Sprintf("could not find speaker pod on announcing node %s", advNode.Name))
			delete(speakerPods, advSpeaker.Spec.NodeName)

			gomega.Eventually(func() error {
				speakerMetrics, err := metrics.ForPod(controllerPod, advSpeaker, metallb.Namespace)
				if err != nil {
					return err
				}

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": advSpeaker.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(1, "metallb_layer2_requests_received", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(1, "metallb_layer2_responses_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(1, "metallb_layer2_gratuitous_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				return nil
			}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

			// Negative - validate that the other speakers don't publish layer2 metrics
			for _, p := range speakerPods {
				speakerMetrics, err := metrics.ForPod(controllerPod, p, metallb.Namespace)
				framework.ExpectNoError(err)

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": p.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
				framework.ExpectError(err, fmt.Sprintf("metallb_speaker_announced present in node: %s", p.Spec.NodeName))
			}

			ginkgo.By("validating the speaker doesn't publish layer2 metrics after deleting the service")
			err = cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)

			gomega.Eventually(func() error {
				speakerMetrics, err := metrics.ForPod(controllerPod, advSpeaker, metallb.Namespace)
				if err != nil {
					return err
				}

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": advSpeaker.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
				if err == nil {
					return fmt.Errorf("metallb_speaker_announced present in node: %s", advSpeaker.Spec.NodeName)
				}

				return nil
			}, 1*time.Minute, 1*time.Second).Should(gomega.BeNil())
		},
			table.Entry("IPV4 - Checking service", "ipv4"),
			table.Entry("IPV6 - Checking service", "ipv6"))
	})

	table.DescribeTable("validate requesting a specific address pool for Loadbalancer service", func(ipRange *string) {
		var services []*corev1.Service
		var servicesIngressIP []string
		var pools []config.AddressPool

		namespace := f.Namespace.Name

		for i := 0; i < 2; i++ {
			ginkgo.By(fmt.Sprintf("configure addresspool number %d", i+1))
			ip, err := config.GetIPFromRangeByIndex(*ipRange, i)
			framework.ExpectNoError(err)
			addressesRange := fmt.Sprintf("%s-%s", ip, ip)
			pool := config.AddressPool{
				Name:     fmt.Sprintf("test-addresspool%d", i+1),
				Protocol: config.Layer2,
				Addresses: []string{
					addressesRange,
				},
			}
			pools = append(pools, pool)

			configData := config.File{
				Pools: pools,
			}
			err = ConfigUpdater.Update(configData)
			framework.ExpectNoError(err)

			ginkgo.By(fmt.Sprintf("configure service number %d", i+1))
			svc, _ := testservice.CreateWithBackend(cs, namespace, fmt.Sprintf("test-service%d", i+1),
				func(svc *corev1.Service) {
					svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
					svc.Annotations = map[string]string{"metallb.universe.tf/address-pool": fmt.Sprintf("test-addresspool%d", i+1)}
				})

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = config.ValidateIPInRange([]config.AddressPool{pool}, ingressIP)
			framework.ExpectNoError(err)

			services = append(services, svc)
			servicesIngressIP = append(servicesIngressIP, ingressIP)

			for j := 0; j <= i; j++ {
				ginkgo.By(fmt.Sprintf("validate service %d IP didn't change", j+1))
				ip := e2eservice.GetIngressPoint(&services[j].Status.LoadBalancer.Ingress[0])
				framework.ExpectEqual(ip, servicesIngressIP[j])

				ginkgo.By(fmt.Sprintf("checking connectivity of service %d to its external VIP", j+1))
				port := strconv.Itoa(int(services[j].Spec.Ports[0].Port))
				hostport := net.JoinHostPort(ip, port)
				address := fmt.Sprintf("http://%s/", hostport)
				err = wget.Do(address, executor.Host)
				framework.ExpectNoError(err)
			}
		}
	},
		table.Entry("IPV4", &IPV4ServiceRange),
		table.Entry("IPV6", &IPV6ServiceRange))
})

// TODO: The tests find the announcing node in multiple ways (MAC/Events).
// We should have a test that verifies that they all return the same node.
func advertisingNodeFromMAC(nodes []corev1.Node, ip string, exc executor.Executor) (*corev1.Node, error) {
	err := mac.UpdateNodeCache(nodes, exc)
	if err != nil {
		return nil, err
	}

	macAddr, err := mac.ForIP(ip, exc)
	if err != nil {
		return nil, err
	}

	advNode, err := mac.MatchNode(nodes, macAddr, exc)
	if err != nil {
		return nil, err
	}

	return advNode, err
}
