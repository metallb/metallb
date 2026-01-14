// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("L2-ServiceSelector", func() {
	var cs clientset.Interface
	testNamespace := ""
	var allNodes []corev1.Node

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
		cs = k8sclient.New()
		var err error
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2svcsel")
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")

		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		allNodes = nodes.Items
	})

	ginkgo.Context("Service Selector", func() {
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

		ginkgo.It("should use OR logic for multiple selectors", func() {
			l2Advertisement := metallbv1beta1.L2Advertisement{
				ObjectMeta: metav1.ObjectMeta{
					Name: "with-multiple-selectors",
				},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					ServiceSelectors: []metav1.LabelSelector{
						{
							MatchLabels: map[string]string{"app": "nginx"},
						},
						{
							MatchLabels: map[string]string{"app": "apache"},
						},
					},
				},
			}

			resources := config.Resources{
				L2Advs: []metallbv1beta1.L2Advertisement{l2Advertisement},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating a service matching the first selector")
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "nginx-svc", service.TrafficPolicyCluster,
				func(s *corev1.Service) {
					s.Labels = map[string]string{"app": "nginx"}
				})
			defer func() {
				err := cs.CoreV1().Services(svc1.Namespace).Delete(context.TODO(), svc1.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err := nodeForService(svc1, allNodes)
				return err
			}, 30*time.Second, 1*time.Second).Should(BeNil(), "Service matching first selector should be advertised")

			ginkgo.By("Creating a service matching the second selector")
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "apache-svc", service.TrafficPolicyCluster,
				func(s *corev1.Service) {
					s.Labels = map[string]string{"app": "apache"}
				})
			defer func() {
				err := cs.CoreV1().Services(svc2.Namespace).Delete(context.TODO(), svc2.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err := nodeForService(svc2, allNodes)
				return err
			}, 30*time.Second, 1*time.Second).Should(BeNil(), "Service matching second selector should be advertised")

			ginkgo.By("Creating a service matching neither selector")
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "other-svc", service.TrafficPolicyCluster,
				func(s *corev1.Service) {
					s.Labels = map[string]string{"app": "other"}
				})
			defer func() {
				err := cs.CoreV1().Services(svc3.Namespace).Delete(context.TODO(), svc3.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Consistently(func() error {
				_, err := nodeForService(svc3, allNodes)
				return err
			}, 10*time.Second, 1*time.Second).ShouldNot(BeNil(), "Service not matching any selector should not be advertised")
		})
	})
})
