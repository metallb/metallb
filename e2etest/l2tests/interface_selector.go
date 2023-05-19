// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"

	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/executor"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/mac"

	"go.universe.tf/metallb/e2etest/pkg/metallb"
	"go.universe.tf/metallb/e2etest/pkg/service"
)

var (
	NodeNics  []string
	LocalNics []string
)

var _ = ginkgo.Describe("L2-interface selector", func() {
	var cs clientset.Interface

	var f *framework.Framework
	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	f = framework.NewDefaultFramework("l2")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		if len(NodeNics) < 2 {
			framework.Fail("Cluster nodes don't have multi-interfaces to test L2-interface selector")
		}
		if len(NodeNics) != len(LocalNics) {
			framework.Fail("Local interfaces can't correspond to cluster node's interfaces")
		}
		cs = f.ClientSet
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("Interface Selector", func() {
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
			framework.ExpectNoError(err)
		})

		ginkgo.It("Validate the LB IP's mac", func() {
			resources := config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "with-interfaces",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							Interfaces: []string{NodeNics[0]},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			framework.ExpectNoError(err)
			var svcNode *corev1.Node
			gomega.Eventually(func() error {
				svcNode, err = k8s.GetSvcNode(cs, svc.Namespace, svc.Name, allNodes)
				return err
			}, 1*time.Minute, 1*time.Second).Should(gomega.Not(gomega.HaveOccurred()))
			speakerPod, err := metallb.SpeakerPodInNode(cs, svcNode.Name)
			framework.ExpectNoError(err)
			selectorMac, err := mac.GetIfaceMac(NodeNics[0], executor.ForPod(speakerPod.Namespace, speakerPod.Name, "speaker"))
			framework.ExpectNoError(err)

			ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = mac.FlushIPNeigh(ingressIP, executor.Host)
			framework.ExpectNoError(err)

			gomega.Eventually(func() string {
				err := mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[0], executor.Host)
				if err != nil {
					return err.Error()
				}
				err = service.ValidateL2(svc)
				if err != nil {
					return err.Error()
				}
				ingressMac, err := mac.ForIP(ingressIP, executor.Host)
				if err != nil {
					return err.Error()
				}
				return ingressMac.String()
			}, 1*time.Minute, 1*time.Second).Should(gomega.Equal(selectorMac.String()))
		})

		ginkgo.It("Modify L2 interface", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])

			for i := range NodeNics {
				resources := config.Resources{
					L2Advs: []metallbv1beta1.L2Advertisement{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "with-interfaces",
							},
							Spec: metallbv1beta1.L2AdvertisementSpec{
								Interfaces: []string{NodeNics[i]},
							},
						},
					},
				}
				err := ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)
				for j := range LocalNics {
					if j == i {
						continue
					}
					gomega.Eventually(func() error {
						return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[j], executor.Host)
					}, 10*time.Second, 1*time.Second).Should(gomega.HaveOccurred())
				}
				gomega.Eventually(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).Should(gomega.Not(gomega.HaveOccurred()))
			}
		})

		ginkgo.It("Specify not existing interfaces", func() {
			resources := config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "with-interfaces",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							Interfaces: []string{"foo"},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = mac.FlushIPNeigh(ingressIP, executor.Host)
			framework.ExpectNoError(err)

			// check arp respond
			for i := range LocalNics {
				gomega.Consistently(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).Should(gomega.HaveOccurred())
			}

			// check announceFailed event
			gomega.Eventually(func() error {
				events, err := cs.CoreV1().Events(svc.Namespace).List(context.Background(), metav1.ListOptions{FieldSelector: "reason=announceFailed"})
				if err != nil {
					return err
				}

				for _, e := range events.Items {
					if e.InvolvedObject.Name == svc.Name {
						return nil
					}
				}
				return fmt.Errorf("service hasn't receive the \"announceFailed\" event")
			}, 1*time.Minute, 1*time.Second).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Address pool connected with two L2 advertisements", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			resources := config.Resources{}

			for i := range NodeNics {
				l2Adv := metallbv1beta1.L2Advertisement{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("with-interfaces-%d-l2adv", i),
					},
					Spec: metallbv1beta1.L2AdvertisementSpec{
						IPAddressPools: []string{"l2-test"},
						Interfaces:     []string{NodeNics[i]},
					},
				}
				resources.L2Advs = append(resources.L2Advs, l2Adv)
			}

			err := mac.FlushIPNeigh(ingressIP, executor.Host)
			framework.ExpectNoError(err)
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			for i := range LocalNics {
				gomega.Eventually(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).ShouldNot(gomega.HaveOccurred())
			}
		})

		ginkgo.It("node selector", func() {
			svc, _ := service.CreateWithBackend(cs, f.Namespace.Name, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})

			framework.ExpectNoError(err)
			for _, node := range allNodes.Items {
				resources := config.Resources{
					L2Advs: []metallbv1beta1.L2Advertisement{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "with-interfaces",
							},
							Spec: metallbv1beta1.L2AdvertisementSpec{
								Interfaces:    []string{NodeNics[0]},
								NodeSelectors: k8s.SelectorsForNodes([]corev1.Node{node}),
							},
						},
					},
				}

				err = ConfigUpdater.Update(resources)
				framework.ExpectNoError(err)

				ingressIP := e2eservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
				err := mac.FlushIPNeigh(ingressIP, executor.Host)
				framework.ExpectNoError(err)

				gomega.Eventually(func() string {
					node, err := nodeForService(svc, allNodes.Items)
					if err != nil {
						return ""
					}
					return node
				}, 1*time.Minute, 1*time.Second).Should(gomega.Equal(node.Name))
			}
		})
	})
})
