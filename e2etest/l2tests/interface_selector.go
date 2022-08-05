// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	internalconfig "go.universe.tf/metallb/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"

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

		ginkgo.It("Validate the LB IP's mac", func() {
			resources := internalconfig.ClusterResources{
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
				resources := internalconfig.ClusterResources{
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
	})
})
