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
	"net"
	"strconv"
	"time"

	"github.com/mikioh/ipaddr"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/mac"
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

			err = mac.UpdateNodeCache(epNodes, executor.Host)
			framework.ExpectNoError(err)

			macAddr, err := mac.ForIP(ingressIP, executor.Host)
			framework.ExpectNoError(err)

			advNode, err := mac.MatchNode(epNodes, macAddr, executor.Host)
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
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(82)
			svc.Spec.Ports[0].Port = 82
			svc.Annotations = map[string]string{"metallb.universe.tf/allow-shared-ip": "foo"}
			svc.Spec.LoadBalancerIP = ip
		})

		framework.ExpectNoError(err)

		jig2 := e2eservice.NewTestJig(cs, namespace, "svcb")
		svc2, err := jig2.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
			svc.Spec.Ports[0].TargetPort = intstr.FromInt(83)
			svc.Spec.Ports[0].Port = 83
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
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", 82), fmt.Sprintf("--udp-port=%d", 82)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(82)
				rc.Spec.Template.Spec.NodeName = nodes.Items[0].Name
			})
		framework.ExpectNoError(err)
		_, err = jig2.Run(
			func(rc *corev1.ReplicationController) {
				rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", 83), fmt.Sprintf("--udp-port=%d", 83)}
				rc.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port = intstr.FromInt(83)
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
