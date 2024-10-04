// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	"go.universe.tf/e2etest/pkg/k8sclient"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/executor"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/mac"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/pkg/status"
)

const agnostImage = "registry.k8s.io/e2e-test-images/agnhost:2.45"

var (
	NodeNics  []string
	LocalNics []string
)

var _ = ginkgo.Describe("L2-interface selector", func() {
	var cs clientset.Interface

	testNamespace := ""

	ginkgo.AfterEach(func() {
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
		if len(NodeNics) < 2 {
			ginkgo.Fail("Cluster nodes don't have multi-interfaces to test L2-interface selector")
		}
		if len(NodeNics) != len(LocalNics) {
			ginkgo.Fail("Local interfaces can't correspond to cluster node's interfaces")
		}
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2interface")
		Expect(err).NotTo(HaveOccurred())
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
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("Validate L2ServiceStatus interface", func() {
			ginkgo.By("use the 1st interface for announcing")
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
			Expect(err).NotTo(HaveOccurred())

			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err = status.L2ForService(ConfigUpdater.Client(), svc)
				return err
			}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

			Consistently(func() string {
				var s *metallbv1beta1.ServiceL2Status
				if s, err = status.L2ForService(ConfigUpdater.Client(), svc); err != nil {
					return err.Error()
				}
				if len(s.Status.Interfaces) == 0 {
					return fmt.Errorf("expect 1 Interface, got %d", len(s.Status.Interfaces)).Error()
				}
				return s.Status.Interfaces[0].Name
			}, 5*time.Second).Should(Equal(NodeNics[0]))
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
			Expect(err).NotTo(HaveOccurred())

			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			var svcNode *corev1.Node
			Eventually(func() error {
				svcNode, err = k8s.GetSvcNode(cs, svc.Namespace, svc.Name, allNodes)
				return err
			}, 1*time.Minute, 1*time.Second).Should(Not(HaveOccurred()))
			speakerPod, err := metallb.SpeakerPodInNode(cs, svcNode.Name)
			Expect(err).NotTo(HaveOccurred())
			selectorMac, err := mac.GetIfaceMac(NodeNics[0], executor.ForPodDebug(speakerPod.Namespace, speakerPod.Name, "speaker", agnostImage))
			Expect(err).NotTo(HaveOccurred())

			ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = mac.FlushIPNeigh(ingressIP, executor.Host)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
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
			}, 1*time.Minute, 1*time.Second).Should(Equal(selectorMac.String()))
		})

		ginkgo.It("Modify L2 interface", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])

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
				Expect(err).NotTo(HaveOccurred())
				for j := range LocalNics {
					if j == i {
						continue
					}
					Eventually(func() error {
						return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[j], executor.Host)
					}, 10*time.Second, 1*time.Second).Should(HaveOccurred())
				}
				Eventually(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).Should(Not(HaveOccurred()))
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
			Expect(err).NotTo(HaveOccurred())

			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
			err = mac.FlushIPNeigh(ingressIP, executor.Host)
			Expect(err).NotTo(HaveOccurred())

			// check arp respond
			for i := range LocalNics {
				Consistently(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).Should(HaveOccurred())
			}

			// check announceFailed event
			Eventually(func() error {
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
			}, 1*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})

		ginkgo.It("Address pool connected with two L2 advertisements", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
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
			Expect(err).NotTo(HaveOccurred())
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			for i := range LocalNics {
				Eventually(func() error {
					return mac.RequestAddressResolutionFromIface(ingressIP, LocalNics[i], executor.Host)
				}, 10*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
			}
		})

		ginkgo.It("node selector", func() {
			svc, _ := service.CreateWithBackend(cs, testNamespace, "lb-service")
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})

			Expect(err).NotTo(HaveOccurred())
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
				Expect(err).NotTo(HaveOccurred())

				ingressIP := jigservice.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
				err := mac.FlushIPNeigh(ingressIP, executor.Host)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() string {
					node, err := nodeForService(svc, allNodes.Items)
					if err != nil {
						return ""
					}
					return node.Name
				}, 1*time.Minute, 1*time.Second).Should(Equal(node.Name))
			}
		})
	})
})
