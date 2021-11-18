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
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/mikioh/ipaddr"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/mac"
	"go.universe.tf/metallb/e2etest/pkg/metrics"
	internalconfig "go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
)

var _ = ginkgo.Describe("L2", func() {
	f := framework.NewDefaultFramework("l2")
	var loadBalancerCreateTimeout time.Duration
	var cs clientset.Interface

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet
		loadBalancerCreateTimeout = e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
		configData := configFile{
			Pools: []addressPool{
				{
					Name:     "l2-test",
					Protocol: Layer2,
					Addresses: []string{
						ipv4ServiceRange,
						ipv6ServiceRange,
					},
				},
			},
		}
		err := updateConfigMap(cs, configData)
		framework.ExpectNoError(err)
	})

	ginkgo.AfterEach(func() {
		// Clean previous configuration.
		err := updateConfigMap(cs, configFile{})
		framework.ExpectNoError(err)

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			DescribeSvc(f.Namespace.Name)
		}
	})

	ginkgo.Context("type=Loadbalancer", func() {
		ginkgo.It("should work for ExternalTrafficPolicy=Cluster", func() {
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)

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
			err := wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work for ExternalTrafficPolicy=Local", func() {
			svc, jig := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeLocal)
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

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err = wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)

			advNode, err := advertisingNodeFromMAC(epNodes, ingressIP, executor.Host)
			framework.ExpectNoError(err)

			for i := 0; i < 5; i++ {
				name, err := getEndpointHostName(hostport, executor.Host)
				framework.ExpectNoError(err)

				pod, err := cs.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				framework.ExpectEqual(pod.Spec.NodeName == advNode.Name, true, "traffic arrived to a pod not from the announcing node")
			}

		})

	})

	ginkgo.Context("validate different AddressPools for type=Loadbalancer", func() {

		table.DescribeTable("set different AddressPools ranges modes", func(getAddressPools func() []addressPool) {
			configData := configFile{
				Pools: getAddressPools(),
			}
			err := updateConfigMap(cs, configData)
			framework.ExpectNoError(err)

			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			ginkgo.By("validate LoadBalancer IP is in the AddressPool range")
			err = validateIPInRange(getAddressPools(), ingressIP)
			framework.ExpectNoError(err)

			ginkgo.By("checking connectivity to its external VIP")

			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)
			err = wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)
		},
			table.Entry("AddressPool defined by address range", func() []addressPool {
				return []addressPool{
					{
						Name:     "l2-test",
						Protocol: Layer2,
						Addresses: []string{
							ipv4ServiceRange,
							ipv6ServiceRange,
						},
					},
				}
			}),
			table.Entry("AddressPool defined by network prefix", func() []addressPool {
				var ipv4AddressesByCIDR []string
				var ipv6AddressesByCIDR []string

				cidrs, err := internalconfig.ParseCIDR(ipv4ServiceRange)
				framework.ExpectNoError(err)
				for _, cidr := range cidrs {
					ipv4AddressesByCIDR = append(ipv4AddressesByCIDR, cidr.String())
				}

				cidrs, err = internalconfig.ParseCIDR(ipv6ServiceRange)
				framework.ExpectNoError(err)
				for _, cidr := range cidrs {
					ipv6AddressesByCIDR = append(ipv6AddressesByCIDR, cidr.String())
				}

				return []addressPool{
					{
						Name:      "l2-test",
						Protocol:  Layer2,
						Addresses: append(ipv4AddressesByCIDR, ipv6AddressesByCIDR...),
					},
				}
			}),
		)
	})

	table.DescribeTable("different services sharing the same ip should advertise from the same node", func(ipRange *string) {
		namespace := f.Namespace.Name

		jig1 := e2eservice.NewTestJig(cs, namespace, "svca")

		ip := firstIPFromRange(*ipRange)
		svc1, err := jig1.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(servicePodPort)
			svc.Spec.Ports[0].Port = int32(servicePodPort)
			svc.Annotations = map[string]string{"metallb.universe.tf/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})

		framework.ExpectNoError(err)

		jig2 := e2eservice.NewTestJig(cs, namespace, "svcb")
		svc2, err := jig2.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(servicePodPort + 1)
			svc.Spec.Ports[0].Port = int32(servicePodPort + 1)
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
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", servicePodPort), fmt.Sprintf("--udp-port=%d", servicePodPort)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(servicePodPort)
				rc.Spec.Template.Spec.NodeName = nodes.Items[0].Name
			})
		framework.ExpectNoError(err)
		_, err = jig2.Run(
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", servicePodPort+1), fmt.Sprintf("--udp-port=%d", servicePodPort+1)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(servicePodPort + 1)
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
		table.Entry("IPV4", &ipv4ServiceRange),
		table.Entry("IPV6", &ipv6ServiceRange))

	ginkgo.Context("metrics", func() {
		var controllerPod *corev1.Pod
		var speakerPods map[string]*corev1.Pod

		ginkgo.BeforeEach(func() {
			pods, err := cs.CoreV1().Pods(testNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=controller",
			})
			framework.ExpectNoError(err)
			framework.ExpectEqual(len(pods.Items), 1, "More than one controller found")
			controllerPod = &pods.Items[0]

			speakers, err := cs.CoreV1().Pods(testNameSpace).List(context.Background(), metav1.ListOptions{
				LabelSelector: "component=speaker",
			})
			framework.ExpectNoError(err)
			speakerPods = map[string]*corev1.Pod{}
			for _, item := range speakers.Items {
				i := item
				speakerPods[i.Spec.NodeName] = &i
			}
		})

		table.DescribeTable("should be exposed by the controller", func(ipFamily string) {
			poolName := "l2-metrics-test"

			configData := configFile{
				Pools: []addressPool{
					{
						Name:     poolName,
						Protocol: Layer2,
						Addresses: []string{
							ipv4ServiceRange,
							ipv6ServiceRange,
						},
					},
				},
			}
			poolCount, err := poolCount(configData.Pools[0])
			framework.ExpectNoError(err)

			err = updateConfigMap(cs, configData)
			framework.ExpectNoError(err)

			ginkgo.By("checking the metrics when no service is added")
			gomega.Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, testNameSpace)
				if err != nil {
					return err
				}
				err = validateGaugeValue(0, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				err = validateGaugeValue(int(poolCount), "metallb_allocator_addresses_total", map[string]string{"pool": poolName}, controllerMetrics)
				if err != nil {
					return err
				}
				return nil
			}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

			ginkgo.By("creating a service")
			svc, _ := createServiceWithBackend(cs, f.Namespace.Name, corev1.ServiceExternalTrafficPolicyTypeCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ginkgo.By("checking the metrics when a service is added")
			gomega.Eventually(func() error {
				controllerMetrics, err := metrics.ForPod(controllerPod, controllerPod, testNameSpace)
				if err != nil {
					return err
				}
				err = validateGaugeValue(1, "metallb_allocator_addresses_in_use_total", map[string]string{"pool": poolName}, controllerMetrics)
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
			err = wgetRetry(address, executor.Host)
			framework.ExpectNoError(err)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			advNode, err := advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
			framework.ExpectNoError(err)
			advSpeaker, ok := speakerPods[advNode.Name]
			framework.ExpectEqual(ok, true, fmt.Sprintf("could not find speaker pod on announcing node %s", advNode.Name))
			delete(speakerPods, advSpeaker.Spec.NodeName)

			gomega.Eventually(func() error {
				speakerMetrics, err := metrics.ForPod(controllerPod, advSpeaker, testNameSpace)
				if err != nil {
					return err
				}

				err = validateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": advSpeaker.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
				if err != nil {
					return err
				}

				err = validateCounterValue(1, "metallb_layer2_requests_received", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				err = validateCounterValue(1, "metallb_layer2_responses_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				err = validateCounterValue(1, "metallb_layer2_gratuitous_sent", map[string]string{"ip": ingressIP}, speakerMetrics)
				if err != nil {
					return err
				}

				return nil
			}, 2*time.Minute, 1*time.Second).Should(gomega.BeNil())

			// Negative - validate that the other speakers don't publish layer2 metrics
			for _, p := range speakerPods {
				speakerMetrics, err := metrics.ForPod(controllerPod, p, testNameSpace)
				framework.ExpectNoError(err)

				err = validateGaugeValue(1, "metallb_speaker_announced", map[string]string{"node": p.Spec.NodeName, "protocol": "layer2", "service": fmt.Sprintf("%s/%s", f.Namespace.Name, svc.Name)}, speakerMetrics)
				framework.ExpectError(err, fmt.Sprintf("metallb_speaker_announced present in node: %s", p.Spec.NodeName))
			}

		},
			table.Entry("IPV4 - Checking service", "ipv4"),
			table.Entry("IPV6 - Checking service", "ipv6"))
	})
})

func firstIPFromRange(ipRange string) string {
	cidr, err := internalconfig.ParseCIDR(ipRange)
	framework.ExpectNoError(err)
	c := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr[0])})
	return c.First().IP.String()
}

// Relies on the endpoint being an agnhost netexec pod.
func getEndpointHostName(ep string, exec executor.Executor) (string, error) {
	res, err := exec.Exec("wget", "-O-", "-q", fmt.Sprintf("http://%s/hostname", ep))
	if err != nil {
		return "", err
	}

	return res, nil
}

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

// taken from internal: poolCount returns the number of addresses in the pool.
// TODO: find a better place for this func.
func poolCount(p addressPool) (int64, error) {
	var total int64
	for _, r := range p.Addresses {
		cidrs, err := internalconfig.ParseCIDR(r)
		if err != nil {
			return 0, err
		}
		for _, cidr := range cidrs {
			o, b := cidr.Mask.Size()
			if b-o >= 62 {
				// An enormous ipv6 range is allocated which will never run out.
				// Just return max to avoid any math errors.
				return math.MaxInt64, nil
			}
			sz := int64(math.Pow(2, float64(b-o)))

			cur := ipaddr.NewCursor([]ipaddr.Prefix{*ipaddr.NewPrefix(cidr)})
			firstIP := cur.First().IP
			lastIP := cur.Last().IP

			if p.AvoidBuggyIPs {
				if o <= 24 {
					// A pair of buggy IPs occur for each /24 present in the range.
					buggies := int64(math.Pow(2, float64(24-o))) * 2
					sz -= buggies
				} else {
					// Ranges smaller than /24 contain 1 buggy IP if they
					// start/end on a /24 boundary, otherwise they contain
					// none.
					if ipConfusesBuggyFirmwares(firstIP) {
						sz--
					}
					if ipConfusesBuggyFirmwares(lastIP) {
						sz--
					}
				}
			}

			total += sz
		}
	}
	return total, nil
}

// taken from internal: ipConfusesBuggyFirmwares returns true if ip is an IPv4 address ending in 0 or 255.
// TODO: find a better place for this func.
func ipConfusesBuggyFirmwares(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return ip[3] == 0 || ip[3] == 255
}
