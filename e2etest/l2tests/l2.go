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

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/k8sclient"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	pkgerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/iprange"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/metrics"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/pkg/status"
	"go.universe.tf/e2etest/pkg/udp"
	"go.universe.tf/e2etest/pkg/wget"
)

var (
	ConfigUpdater       config.Updater
	Reporter            *k8sreporter.KubernetesReporter
	IPV4ServiceRange    string
	IPV6ServiceRange    string
	PrometheusNamespace string
)

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	testNamespace := ""

	emptyL2Advertisement := metallbv1beta1.L2Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err = k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2")
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("type=Loadbalancer", func() {
		ginkgo.BeforeEach(func() {
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								IPV4ServiceRange,
								IPV6ServiceRange},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Cluster", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("checking connectivity to its external VIP")

			Eventually(func() error {
				return service.ValidateL2(svc)
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		ginkgo.It("should process the status as L2ServiceStatus along with service", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			ginkgo.By("checking correct serviceL2Status object is populated")

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			var l2Status *metallbv1beta1.ServiceL2Status
			Eventually(func() error {
				s, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if err != nil {
					return err
				}
				l2Status = s
				return nil
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("validating the node in ServiceL2Status is the one who is announcing for the service")
			var speakingNode string
			Eventually(func() string {
				node, err := k8s.GetSvcNode(cs, svc.Namespace, svc.Name, allNodes)
				if err != nil {
					return err.Error()
				}
				speakingNode = node.Name
				return node.Name
			}, time.Minute, time.Second).Should(Equal(l2Status.Status.Node))
			Consistently(func() string {
				var s *metallbv1beta1.ServiceL2Status
				if s, err = status.L2ForService(ConfigUpdater.Client(), svc); err != nil {
					return err.Error()
				}
				return s.Status.Node
			}, 5*time.Second).Should(Equal(speakingNode))

			service.Delete(cs, svc)
			ginkgo.By("validating the status object is deleted after the service is deleted")
			Eventually(func() bool {
				_, err = status.L2ForService(ConfigUpdater.Client(), svc)
				return pkgerr.IsNotFound(err)
			}, 2*time.Minute, 1*time.Second).Should(Equal(true))
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Local", func() {
			svc, jig := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyLocal)
			err := jig.Scale(context.TODO(), 5)
			Expect(err).NotTo(HaveOccurred())

			epNodes, err := jig.ListNodesWithEndpoint(context.TODO()) // Only nodes with an endpoint could be advertising the IP
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := jigservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)

			ginkgo.By(fmt.Sprintf("checking connectivity to its external VIP %s", hostport))
			err = wget.Do(address, executor.Host)
			Expect(err).NotTo(HaveOccurred())

			// Give the speakers enough time to settle and for the announcer to complete its gratuitous.
			Eventually(func() error {
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
					pod, err := cs.CoreV1().Pods(testNamespace).Get(context.TODO(), name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					if pod.Spec.NodeName != advNode.Name {
						return fmt.Errorf("traffic arrived to a pod on node %s which is not the announcing node %s", pod.Spec.NodeName, advNode.Name)
					}
				}

				return nil
			}, 5*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
		})

		ginkgo.It("IPV4 Should work with mixed protocol services", func() {

			tcpPort := service.TestServicePort
			udpPort := service.TestServicePort + 1
			namespace := testNamespace

			ginkgo.By("Creating a mixed protocol TCP / UDP service")
			jig1 := jigservice.NewTestJig(cs, namespace, "svca")
			svc1, err := jig1.CreateLoadBalancerService(context.TODO(), func(svc *corev1.Service) {
				svc.Spec.Ports[0].TargetPort = intstr.FromInt(tcpPort)
				svc.Spec.Ports[0].Port = int32(tcpPort)
				svc.Spec.Ports[0].Name = "tcp"
				svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
					Protocol:   corev1.ProtocolUDP,
					TargetPort: intstr.FromInt(udpPort),
					Port:       int32(udpPort),
					Name:       "udp",
				})
			})

			Expect(err).NotTo(HaveOccurred())

			defer func() {
				err := cs.CoreV1().Services(svc1.Namespace).Delete(context.TODO(), svc1.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Expect(err).NotTo(HaveOccurred())
			_, err = jig1.Run(context.TODO(),
				func(rc *corev1.ReplicationController) {
					rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", tcpPort), fmt.Sprintf("--udp-port=%d", udpPort)}
					rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(tcpPort)
				})
			Expect(err).NotTo(HaveOccurred())

			ingressIP := jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0])
			hostport := net.JoinHostPort(ingressIP, strconv.Itoa(udpPort))

			ginkgo.By(fmt.Sprintf("checking connectivity to its external VIP %s", hostport))
			Eventually(func() error {
				return udp.Check(hostport)
			}, 2*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("checking connectivity to its external VIP %s", hostport))
			hostport = net.JoinHostPort(ingressIP, strconv.Itoa(tcpPort))
			address := fmt.Sprintf("http://%s/", hostport)
			err = wget.Do(address, executor.Host)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("should not be announced from a node with a NetworkUnavailable condition", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("getting the advertising node")
			var nodeToSet *corev1.Node

			Eventually(func() error {
				var err error
				nodeToSet, err = nodeForService(svc, allNodes.Items)
				if err != nil {
					return err
				}
				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			err = k8s.SetNodeCondition(cs, nodeToSet.Name, corev1.NodeNetworkUnavailable, corev1.ConditionTrue)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				err = k8s.SetNodeCondition(cs, nodeToSet.Name, corev1.NodeNetworkUnavailable, corev1.ConditionFalse)
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("validating the service is announced from a different node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).ShouldNot(Equal(nodeToSet.Name))

			ginkgo.By("setting the NetworkUnavailable condition back to false")
			err = k8s.SetNodeCondition(cs, nodeToSet.Name, corev1.NodeNetworkUnavailable, corev1.ConditionFalse)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("validating the service is announced back again from the previous node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).Should(Equal(nodeToSet.Name))
		})

		ginkgo.It("It should be work when adding NodeExcludeBalancers label to a node", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("getting the advertising node")
			var nodeToSet *corev1.Node

			Eventually(func() error {
				var err error
				nodeToSet, err = nodeForService(svc, allNodes.Items)
				if err != nil {
					return err
				}
				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("add the NodeExcludeBalancers label of the node")

			k8s.AddLabelToNode(nodeToSet.Name, corev1.LabelNodeExcludeBalancers, "", cs)
			defer func() {
				ginkgo.By("removing the NodeExcludeBalancers label of the node")
				k8s.RemoveLabelFromNode(nodeToSet.Name, corev1.LabelNodeExcludeBalancers, cs)
			}()

			ginkgo.By("validating the service is announced from a different node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).ShouldNot(Equal(nodeToSet.Name))

			ginkgo.By("removing the NodeExcludeBalancers label of the node")
			k8s.RemoveLabelFromNode(nodeToSet.Name, corev1.LabelNodeExcludeBalancers, cs)

			ginkgo.By("validating the service is announced back again from the previous node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).Should(Equal(nodeToSet.Name))
		})
	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {

		ginkgo.DescribeTable("set different AddressPools ranges modes", func(getAddressPools func() []metallbv1beta1.IPAddressPool) {
			resources := config.Resources{
				Pools:  getAddressPools(),
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ingressIP := jigservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			err = config.ValidateIPInRange(getAddressPools(), ingressIP)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("checking connectivity to its external VIP")

			Eventually(func() error {
				return service.ValidateL2(svc)
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		},
			ginkgo.Entry("AddressPool defined by address range", func() []metallbv1beta1.IPAddressPool {
				return []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								IPV4ServiceRange,
								IPV6ServiceRange},
						},
					},
				}
			}),
			ginkgo.Entry("AddressPool defined by network prefix",

				func() []metallbv1beta1.IPAddressPool {
					var ipv4AddressesByCIDR []string
					var ipv6AddressesByCIDR []string

					cidrs, err := iprange.Parse(IPV4ServiceRange)
					Expect(err).NotTo(HaveOccurred())
					for _, cidr := range cidrs {
						ipv4AddressesByCIDR = append(ipv4AddressesByCIDR, cidr.String())
					}

					cidrs, err = iprange.Parse(IPV6ServiceRange)
					Expect(err).NotTo(HaveOccurred())
					for _, cidr := range cidrs {
						ipv6AddressesByCIDR = append(ipv6AddressesByCIDR, cidr.String())
					}
					return []metallbv1beta1.IPAddressPool{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "l2-test",
							},
							Spec: metallbv1beta1.IPAddressPoolSpec{
								Addresses: append(ipv4AddressesByCIDR, ipv6AddressesByCIDR...),
							},
						},
					}
				}),
		)
	})

	ginkgo.DescribeTable("different services sharing the same ip should advertise from the same node", func(ipRange *string) {
		resources := config.Resources{
			Pools: []metallbv1beta1.IPAddressPool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "l2-services-same-ip-test",
					},
					Spec: metallbv1beta1.IPAddressPoolSpec{
						Addresses: []string{
							IPV4ServiceRange,
							IPV6ServiceRange},
					},
				},
			},
			L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
		}

		err := ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())
		namespace := testNamespace

		jig1 := jigservice.NewTestJig(cs, namespace, "svca")

		ip, err := config.GetIPFromRangeByIndex(*ipRange, 0)
		Expect(err).NotTo(HaveOccurred())
		svc1, err := jig1.CreateLoadBalancerService(context.TODO(), func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort)
			svc.Spec.Ports[0].Port = int32(service.TestServicePort)
			svc.Annotations = map[string]string{"metallb.io/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})

		Expect(err).NotTo(HaveOccurred())

		jig2 := jigservice.NewTestJig(cs, namespace, "svcb")
		svc2, err := jig2.CreateLoadBalancerService(context.TODO(), func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort + 1)
			svc.Spec.Ports[0].Port = int32(service.TestServicePort + 1)
			svc.Annotations = map[string]string{"metallb.io/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			err := cs.CoreV1().Services(svc1.Namespace).Delete(context.TODO(), svc1.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			err = cs.CoreV1().Services(svc2.Namespace).Delete(context.TODO(), svc2.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}()

		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = jig1.Run(context.TODO(),
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort), fmt.Sprintf("--udp-port=%d", service.TestServicePort)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(service.TestServicePort)
				rc.Spec.Template.Spec.NodeName = nodes.Items[0].Name
			})
		Expect(err).NotTo(HaveOccurred())
		_, err = jig2.Run(context.TODO(),
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort+1), fmt.Sprintf("--udp-port=%d", service.TestServicePort+1)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(service.TestServicePort + 1)
				rc.Spec.Template.Spec.NodeName = nodes.Items[1].Name
			})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
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
				return fmt.Errorf("service announced from different nodes %s %s", service1Announce, service2Announce)
			}
			return nil
		}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

	},
		ginkgo.Entry("IPV4", &IPV4ServiceRange),
		ginkgo.Entry("IPV6", &IPV6ServiceRange))

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods map[string]*corev1.Pod
		var promPod *corev1.Pod

		ginkgo.BeforeEach(func() {
			var err error
			controllerPod, err = metallb.ControllerPod(cs)
			Expect(err).NotTo(HaveOccurred())

			speakers, err := metallb.SpeakerPods(cs)
			Expect(err).NotTo(HaveOccurred())

			speakerPods = map[string]*corev1.Pod{}
			for _, item := range speakers {
				i := item
				speakerPods[i.Spec.NodeName] = i
			}

			promPod, err = metrics.PrometheusPod(cs, PrometheusNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.DescribeTable("should be exposed by the controller", func(ipFamily string) {
			poolName := "l2-metrics-test"
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: poolName,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								IPV4ServiceRange,
								IPV6ServiceRange},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
			}

			total, ipv4, ipv6, err := config.PoolCount(resources.Pools[0])
			Expect(err).NotTo(HaveOccurred())

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("checking the metrics when no service is added")
			Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(promPod, controllerPod, metallb.Namespace)
				if err != nil {
					return err
				}
				cases := []struct {
					name   string
					expect int64
				}{
					{
						"addresses_in_use_total", 0,
					},
					{
						"addresses_total", total,
					},
					{
						"ipv4_addresses_in_use_total", 0,
					},
					{
						"ipv4_addresses_total", ipv4,
					},
					{
						"ipv6_addresses_in_use_total", 0,
					},
					{
						"ipv6_addresses_total", ipv6,
					},
				}
				for _, c := range cases {
					err = metrics.ValidateGaugeValue(int(c.expect), "metallb_allocator_"+c.name, map[string]string{"pool": poolName}, controllerMetrics)
					if err != nil {
						return err
					}
					err = metrics.ValidateOnPrometheus(promPod, fmt.Sprintf(`metallb_allocator_%s{pool="%s"} == %d`, c.name, poolName, c.expect), metrics.There)
					if err != nil {
						return err
					}
				}
				return nil
			}, 2*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("creating a service")
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				if !pkgerr.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}()

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
			}, 2*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

			ingressIP := jigservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			Eventually(func() error {
				return mac.RequestAddressResolution(ingressIP, executor.Host)
			}, 2*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))

			ginkgo.By("checking connectivity to its external VIP")

			Eventually(func() error {
				return service.ValidateL2(svc)
			}, 2*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			var advNode *corev1.Node
			var advSpeaker *corev1.Pod
			Eventually(func() error {
				var ok bool

				advNode, err = advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
				if err != nil {
					return err
				}

				advSpeaker, ok = speakerPods[advNode.Name]
				if !ok {
					return fmt.Errorf("could not find speaker pod on announcing node %s", advNode.Name)
				}

				speakerMetrics, err := metrics.ForPod(promPod, advSpeaker, metallb.Namespace)
				if err != nil {
					return err
				}

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": advSpeaker.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", testNamespace, svc.Name)}, speakerMetrics)
				if err != nil {
					return err
				}

				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_speaker_announced{node="%s",protocol="layer2",service="%s/%s"} == 1`,
						advSpeaker.Spec.NodeName, testNamespace, svc.Name), metrics.There)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), "metallb_layer2_requests_received", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_layer2_requests_received{ip="%s"} >= 1`, ingressIP), metrics.There)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), "metallb_layer2_responses_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_layer2_responses_sent{ip="%s"} >= 1`, ingressIP), metrics.There)
				if err != nil {
					return err
				}

				err = metrics.ValidateCounterValue(metrics.GreaterOrEqualThan(1), "metallb_layer2_gratuitous_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}
				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_layer2_gratuitous_sent{ip="%s"} >= 1`, ingressIP), metrics.There)
				if err != nil {
					return err
				}

				return nil
			}, 2*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

			// Negative - validate that the other speakers don't publish layer2 metrics
			delete(speakerPods, advSpeaker.Spec.NodeName)

			for _, p := range speakerPods {
				speakerMetrics, err := metrics.ForPod(promPod, p, metallb.Namespace)
				Expect(err).NotTo(HaveOccurred())

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": p.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", testNamespace, svc.Name)}, speakerMetrics)
				Expect(err).To(HaveOccurred(), fmt.Sprintf("metallb_speaker_announced present in node: %s", p.Spec.NodeName))

				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_speaker_announced{node="%s",protocol="layer2",service="%s/%s"} == 1`, p.Spec.NodeName, testNamespace, svc.Name), metrics.NotThere)
				Expect(err).NotTo(HaveOccurred())
			}

			ginkgo.By("validating the speaker doesn't publish layer2 metrics after deleting the service")
			err = cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				speakerMetrics, err := metrics.ForPod(promPod, advSpeaker, metallb.Namespace)
				if err != nil {
					return err
				}

				err = metrics.ValidateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": advSpeaker.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", testNamespace, svc.Name)}, speakerMetrics)
				if err == nil {
					return fmt.Errorf("metallb_speaker_announced present in node: %s", advSpeaker.Spec.NodeName)
				}
				err = metrics.ValidateOnPrometheus(promPod,
					fmt.Sprintf(`metallb_speaker_announced{node="%s",protocol="layer2",service="%s/%s"} == 1`, advSpeaker.Spec.NodeName, testNamespace, svc.Name), metrics.NotThere)
				if err != nil {
					return err
				}

				return nil
			}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())
		},
			ginkgo.Entry("IPV4 - Checking service", "ipv4"),
			ginkgo.Entry("IPV6 - Checking service", "ipv6"))
	})

	ginkgo.DescribeTable("validate requesting a specific address pool for Loadbalancer service", func(ipRange *string, autoAssign bool) {
		var services []*corev1.Service
		var servicesIngressIP []string
		var pools []metallbv1beta1.IPAddressPool

		namespace := testNamespace

		for i := 0; i < 2; i++ {
			ginkgo.By(fmt.Sprintf("configure addresspool number %d", i+1))
			ip, err := config.GetIPFromRangeByIndex(*ipRange, i)
			Expect(err).NotTo(HaveOccurred())
			addressesRange := fmt.Sprintf("%s-%s", ip, ip)
			pool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test-addresspool%d", i+1),
				},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses:  []string{addressesRange},
					AutoAssign: &autoAssign,
				},
			}
			pools = append(pools, pool)

			resources := config.Resources{
				Pools:  pools,
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("configure service number %d", i+1))
			svc, _ := service.CreateWithBackend(cs, namespace, fmt.Sprintf("test-service%d", i+1),
				func(svc *corev1.Service) {
					svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
					svc.Annotations = map[string]string{"metallb.io/address-pool": fmt.Sprintf("test-addresspool%d", i+1)}
				})

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool}, ingressIP)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("validate annotating a service with the pool used to provide its IP")
			Expect(svc.Annotations["metallb.io/ip-allocated-from-pool"]).To(Equal(pool.Name))

			services = append(services, svc)
			servicesIngressIP = append(servicesIngressIP, ingressIP)

			for j := 0; j <= i; j++ {

				ginkgo.By(fmt.Sprintf("validate service %d IP didn't change", j+1))
				ip := jigservice.GetIngressPoint(&services[j].Status.LoadBalancer.Ingress[0])
				Expect(ip).To(Equal(servicesIngressIP[j]))

				ginkgo.By(fmt.Sprintf("checking connectivity of service %d to its external VIP", j+1))
				Eventually(func() error {
					return service.ValidateL2(services[j])
				}, 2*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))
			}
		}
	},
		ginkgo.Entry("IPV4", &IPV4ServiceRange, true),
		ginkgo.Entry("IPV6", &IPV6ServiceRange, true),
		ginkgo.Entry("IPV4", &IPV4ServiceRange, false),
		ginkgo.Entry("IPV6", &IPV6ServiceRange, false))
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
