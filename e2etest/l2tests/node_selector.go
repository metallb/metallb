// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	var nodeToLabel *corev1.Node
	testNamespace := ""

	ginkgo.AfterEach(func() {
		if nodeToLabel != nil {
			k8s.RemoveLabelFromNode(nodeToLabel.Name, "bgp-node-selector-test", cs)
		}

		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		err = k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		cs = k8sclient.New()
		var err error
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2sel")
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")

		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("Node Selector", func() {
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
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("should work selecting one node", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			for _, node := range allNodes.Items {
				l2Advertisement := metallbv1beta1.L2Advertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: "with-selector",
					},
					Spec: metallbv1beta1.L2AdvertisementSpec{
						NodeSelectors: k8s.SelectorsForNodes([]corev1.Node{node}),
					},
				}

				ginkgo.By(fmt.Sprintf("Assigning the advertisement to node %s", node.Name))
				resources := config.Resources{
					L2Advs: []metallbv1beta1.L2Advertisement{l2Advertisement},
				}

				err := ConfigUpdater.Update(resources)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() string {
					node, err := nodeForService(svc, allNodes.Items)
					if err != nil {
						return ""
					}
					return node.Name
				}, 30*time.Second, 1*time.Second).Should(Equal(node.Name))
			}
		})

		ginkgo.It("should work with multiple node selectors", func() {
			// ETP = local, pin the endpoint to node0, have two l2 advertisements, one for
			// all and one for node1, check node0 is advertised.
			jig := jigservice.NewTestJig(cs, testNamespace, "svca")
			svc, err := jig.CreateLoadBalancerService(context.TODO(), func(svc *corev1.Service) {
				svc.Spec.Ports[0].TargetPort = intstr.FromInt(service.TestServicePort)
				svc.Spec.Ports[0].Port = int32(service.TestServicePort)
				svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
			})
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("Not enough nodes")
			}
			_, err = jig.Run(context.TODO(),
				func(rc *corev1.ReplicationController) {
					rc.Spec.Template.Spec.Containers[0].Args = []string{"netexec", fmt.Sprintf("--http-port=%d", service.TestServicePort), fmt.Sprintf("--udp-port=%d", service.TestServicePort)}
					rc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.FromInt(service.TestServicePort)
					rc.Spec.Template.Spec.NodeName = allNodes.Items[0].Name
				})
			Expect(err).NotTo(HaveOccurred())

			l2Advertisements := []metallbv1beta1.L2Advertisement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "with-selector",
					},
					Spec: metallbv1beta1.L2AdvertisementSpec{
						NodeSelectors: k8s.SelectorsForNodes([]corev1.Node{allNodes.Items[1]}),
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-selector",
					},
				},
			}

			ginkgo.By("Creating the l2 advertisements")
			resources := config.Resources{
				L2Advs: l2Advertisements,
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("checking connectivity to its external VIP")

			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return err.Error()
				}
				return node.Name
			}, 2*time.Minute, time.Second).Should(Equal(allNodes.Items[0].Name))
		})

		ginkgo.It("should work when adding nodes", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ingressIP := jigservice.GetIngressPoint(
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
			resources := config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2Advertisement},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Validating service IP not advertised")
			Eventually(func() error {
				return mac.RequestAddressResolution(ingressIP, executor.Host)
			}, 2*time.Minute, time.Second).Should(HaveOccurred())

			nodeToLabel = &allNodes.Items[0]
			ginkgo.By(fmt.Sprintf("Adding advertisement label to node %s", nodeToLabel.Name))
			k8s.AddLabelToNode(nodeToLabel.Name, "l2-node-selector-test", "true", cs)

			ginkgo.By(fmt.Sprintf("Validating service IP advertised by %s", nodeToLabel.Name))

			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return err.Error()
				}
				return node.Name
			}, 2*time.Minute, time.Second).Should(Equal(nodeToLabel.Name))
		})
	})
})
