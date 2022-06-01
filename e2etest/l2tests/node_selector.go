// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/mac"
	"go.universe.tf/metallb/e2etest/pkg/service"
	"go.universe.tf/metallb/e2etest/pkg/wget"

	internalconfig "go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	var nodeToLabel *v1.Node

	var f *framework.Framework
	ginkgo.AfterEach(func() {
		if nodeToLabel != nil {
			k8s.RemoveLabelFromNode(nodeToLabel.Name, "bgp-node-selector-test", cs)
		}

		if ginkgo.CurrentGinkgoTestDescription().Failed {
			k8s.DumpInfo(Reporter, ginkgo.CurrentGinkgoTestDescription().TestText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	f = framework.NewDefaultFramework("l2")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet

		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("Node Selector", func() {
		ginkgo.BeforeEach(func() {
			resources := internalconfig.ClusterResources{
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
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)
		})

		ginkgo.It("should work selecting one node", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)
			for _, node := range allNodes.Items {
				l2Advertisement := metallbv1beta1.L2Advertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: "with-selector",
					},
					Spec: metallbv1beta1.L2AdvertisementSpec{
						NodeSelectors: k8s.SelectorsForNodes([]v1.Node{node}),
					},
				}

				ginkgo.By(fmt.Sprintf("Assigning the advertisement to node %s", node.Name))
				resources := internalconfig.ClusterResources{
					L2Advs: []metallbv1beta1.L2Advertisement{l2Advertisement},
				}

				err := ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
				hostport := net.JoinHostPort(ingressIP, port)
				address := fmt.Sprintf("http://%s/", hostport)

				gomega.Eventually(func() string {
					err := mac.RequestAddressResolution(ingressIP, executor.Host)
					if err != nil {
						return err.Error()
					}
					err = wget.Do(address, executor.Host)
					framework.ExpectNoError(err)
					advNode, err := advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
					if err != nil {
						return err.Error()
					}
					return advNode.Name
				}, 30*time.Second, 1*time.Second).Should(gomega.Equal(node.Name))
			}
		})

		ginkgo.It("should work with multiple node selectors", func() {
			// ETP = local, pin the endpoint to node0, have two l2 advertisements, one for
			// all and one for node1, check node0 is advertised.
			jig := e2eservice.NewTestJig(cs, f.Namespace.Name, "svca")
			loadBalancerCreateTimeout := e2eservice.GetServiceLoadBalancerCreationTimeout(cs)
			svc, err := jig.CreateLoadBalancerService(loadBalancerCreateTimeout, func(svc *corev1.Service) {
				svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort)
				svc.Spec.Ports[0].Port = int32(service.TestServicePort)
				svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
			})
			framework.ExpectNoError(err)

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("Not enough nodes")
			}
			_, err = jig.Run(
				func(rc *corev1.ReplicationController) {
					rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort), fmt.Sprintf("--udp-port=%d", service.TestServicePort)}
					rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(service.TestServicePort)
					rc.Spec.Template.Spec.NodeName = allNodes.Items[0].Name
				})
			framework.ExpectNoError(err)

			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			l2Advertisements := []metallbv1beta1.L2Advertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "with-selector",
					},
					Spec: metallbv1beta1.L2AdvertisementSpec{
						NodeSelectors: k8s.SelectorsForNodes([]v1.Node{allNodes.Items[1]}),
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-selector",
					},
				},
			}

			ginkgo.By("Creating the l2 advertisements")
			resources := internalconfig.ClusterResources{
				L2Advs: l2Advertisements,
			}

			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("checking connectivity to its external VIP")
			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)

			gomega.Eventually(func() string {
				err := mac.RequestAddressResolution(ingressIP, executor.Host)
				if err != nil {
					return err.Error()
				}
				err = wget.Do(address, executor.Host)
				framework.ExpectNoError(err)

				advNode, err := advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
				if err != nil {
					return err.Error()
				}
				return advNode.Name
			}, 2*time.Minute, time.Second).Should(gomega.Equal(allNodes.Items[0].Name))
		})

		ginkgo.It("should work when adding nodes", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(
				&svc.Status.LoadBalancer.Ingress[0])

			l2Advertisement := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "with-selector",
				},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					NodeSelectors: []metav1.LabelSelector{
						{
							MatchLabels: map[string]string{
								"l2-node-selector-test": "true",
							},
						},
					},
				},
			}

			ginkgo.By("Setting advertisement with node selector (no matching nodes)")
			resources := internalconfig.ClusterResources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2Advertisement},
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			port := strconv.Itoa(int(svc.Spec.Ports[0].Port))
			hostport := net.JoinHostPort(ingressIP, port)
			address := fmt.Sprintf("http://%s/", hostport)

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)

			ginkgo.By("Validating service IP not advertised")
			gomega.Eventually(func() error {
				return mac.RequestAddressResolution(ingressIP, executor.Host)
			}, 2*time.Minute, time.Second).Should(gomega.HaveOccurred())

			nodeToLabel = &allNodes.Items[0]
			ginkgo.By(fmt.Sprintf("Adding advertisement label to node %s", nodeToLabel.Name))
			k8s.AddLabelToNode(nodeToLabel.Name, "l2-node-selector-test", "true", cs)

			ginkgo.By(fmt.Sprintf("Validating service IP advertised by %s", nodeToLabel.Name))

			gomega.Eventually(func() string {
				err := mac.RequestAddressResolution(ingressIP, executor.Host)
				if err != nil {
					return err.Error()
				}
				err = wget.Do(address, executor.Host)
				framework.ExpectNoError(err)
				advNode, err := advertisingNodeFromMAC(allNodes.Items, ingressIP, executor.Host)
				if err != nil {
					return err.Error()
				}
				return advNode.Name
			}, 2*time.Minute, time.Second).Should(gomega.Equal(nodeToLabel.Name))
		})
	})
})
