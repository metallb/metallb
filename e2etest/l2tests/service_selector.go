// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/pkg/status"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	pkgerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("L2-ServiceSelector", func() {
	var cs clientset.Interface
	testNamespace := ""

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		err := k8s.DeleteNamespace(cs, testNamespace)
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
	})

	ginkgo.Context("Pool and Service Selectors together", func() {
		ginkgo.It("should only advertise services matching both pool and service selectors", func() {
			ginkgo.By("Setting up two pools with labels and advertisement with both selectors")

			poolAv4, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())
			poolAv6, err := config.GetIPFromRangeByIndex(IPV6ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())
			poolBv4, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 1)
			Expect(err).NotTo(HaveOccurred())
			poolBv6, err := config.GetIPFromRangeByIndex(IPV6ServiceRange, 1)
			Expect(err).NotTo(HaveOccurred())

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "pool-a",
							Labels: map[string]string{"pool": "a"},
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolAv4 + "/32", poolAv6 + "/128"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "pool-b",
							Labels: map[string]string{"pool": "b"},
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolBv4 + "/32", poolBv6 + "/128"},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-pool-a-expose-true",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							IPAddressPoolSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"pool": "a"},
								},
							},
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"expose": "true"},
								},
							},
						},
					},
				},
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating service with expose=true requesting pool-a - should be advertised")
			svcPoolA, _ := service.CreateWithBackend(cs, testNamespace, "svc-pool-a", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"expose": "true"}),
				service.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-a"}))
			defer func() {
				err := cs.CoreV1().Services(svcPoolA.Namespace).Delete(context.TODO(), svcPoolA.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolA)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred(), "Service matching both selectors should be advertised")

			ginkgo.By("Creating service with expose=true requesting pool-b - pool doesn't match advertisement")
			svcPoolB, _ := service.CreateWithBackend(cs, testNamespace, "svc-pool-b", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"expose": "true"}),
				service.WithAnnotations(map[string]string{"metallb.io/address-pool": "pool-b"}))
			defer func() {
				err := cs.CoreV1().Services(svcPoolB.Namespace).Delete(context.TODO(), svcPoolB.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Checking that svc-pool-b is NOT advertised (pool selector mismatch)")
			Consistently(func() bool {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolB)
				return pkgerr.IsNotFound(err)
			}, 5*time.Second, 1*time.Second).Should(BeTrue(), "Service with pool selector mismatch should NOT be advertised")

			ginkgo.By("Updating svc-pool-a labels to expose=false - service selector no longer matches")
			svcPoolA, err = cs.CoreV1().Services(svcPoolA.Namespace).Get(context.TODO(), svcPoolA.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svcPoolA.Labels["expose"] = "false"
			svcPoolA, err = cs.CoreV1().Services(svcPoolA.Namespace).Update(context.TODO(), svcPoolA, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Checking that svc-pool-a is no longer advertised (service selector mismatch)")
			Eventually(func() bool {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolA)
				return pkgerr.IsNotFound(err)
			}, 30*time.Second, 1*time.Second).Should(BeTrue(), "Service with service selector mismatch should stop being advertised")

			Consistently(func() bool {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolA)
				return pkgerr.IsNotFound(err)
			}, 5*time.Second, 1*time.Second).Should(BeTrue(), "Service with service selector mismatch should remain not advertised")

			ginkgo.By("Updating svc-pool-b labels to expose=false - now both selectors don't match")
			svcPoolB, err = cs.CoreV1().Services(svcPoolB.Namespace).Get(context.TODO(), svcPoolB.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svcPoolB.Labels["expose"] = "false"
			svcPoolB, err = cs.CoreV1().Services(svcPoolB.Namespace).Update(context.TODO(), svcPoolB, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Checking that svc-pool-b is still NOT advertised (both selectors mismatch)")
			Consistently(func() bool {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolB)
				return pkgerr.IsNotFound(err)
			}, 5*time.Second, 1*time.Second).Should(BeTrue(), "Service with both selectors mismatch should NOT be advertised")

			ginkgo.By("Updating svc-pool-a labels back to expose=true - should be advertised again")
			svcPoolA, err = cs.CoreV1().Services(svcPoolA.Namespace).Get(context.TODO(), svcPoolA.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			svcPoolA.Labels["expose"] = "true"
			svcPoolA, err = cs.CoreV1().Services(svcPoolA.Namespace).Update(context.TODO(), svcPoolA, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := status.L2ForService(ConfigUpdater.Client(), svcPoolA)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred(), "Service matching both selectors should be advertised again")
		})
	})

	ginkgo.Context("Service selector filters election candidate nodes", func() {
		ginkgo.It("should announce from exactly one node when both advertisements match the service", func() {
			ginkgo.By("Getting the list of nodes")
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("Not enough nodes, need at least 2")
			}

			node0 := allNodes.Items[0]
			node1 := allNodes.Items[1]

			poolV4, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())
			poolV6, err := config.GetIPFromRangeByIndex(IPV6ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Setting up a pool with two L2 advertisements targeting different nodes but the same service")
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolV4 + "/32", poolV6 + "/128"},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-for-node0",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							NodeSelectors: k8s.SelectorsForNodes([]corev1.Node{node0}),
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"app": "test"},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-for-node1",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							NodeSelectors: k8s.SelectorsForNodes([]corev1.Node{node1}),
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"app": "test"},
								},
							},
						},
					},
				},
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating a service matching both advertisements' service selector")
			svc, _ := service.CreateWithBackend(cs, testNamespace, "svc-test", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"app": "test"}))
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Checking that exactly one of the two candidate nodes announces the service")
			Eventually(func() string {
				l2Status, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if err != nil {
					return fmt.Sprintf("error getting L2 status: %v", err)
				}
				return l2Status.Status.Node
			}, 30*time.Second, 1*time.Second).Should(BeElementOf(node0.Name, node1.Name),
				"Service should be announced by exactly one of the two candidate nodes")
		})

		ginkgo.It("should not announce when no advertisement matches both service and node", func() {
			poolV4, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())
			poolV6, err := config.GetIPFromRangeByIndex(IPV6ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Setting up a pool with two L2 advertisements: one matches all nodes but wrong service, one matches our service but non-existing node")
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "l2-test",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{poolV4 + "/32", poolV6 + "/128"},
						},
					},
				},
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-all-nodes-wrong-svc",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"app": "other"},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "adv-right-svc-ghost-node",
						},
						Spec: metallbv1beta1.L2AdvertisementSpec{
							NodeSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{
										"kubernetes.io/hostname": "non-existing-node",
									},
								},
							},
							ServiceSelectors: []metav1.LabelSelector{
								{
									MatchLabels: map[string]string{"app": "web"},
								},
							},
						},
					},
				},
			}

			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating a service matching only the ghost-node advertisement's service selector")
			svc, _ := service.CreateWithBackend(cs, testNamespace, "svc-web", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"app": "web"}))
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("Checking that no node announces the service")
			Consistently(func() error {
				l2Status, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if pkgerr.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("unexpected error getting L2 status: %v", err)
				}
				return fmt.Errorf("service is being announced by node %q but should not be", l2Status.Status.Node)
			}, 10*time.Second, 1*time.Second).Should(Succeed(),
				"Service should not be announced when no advertisement matches both service and node")

		})
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
				service.WithLabels(map[string]string{"app": "nginx"}))
			defer func() {
				err := cs.CoreV1().Services(svc1.Namespace).Delete(context.TODO(), svc1.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err := status.L2ForService(ConfigUpdater.Client(), svc1)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred(), "Service matching first selector should be advertised")

			ginkgo.By("Creating a service matching the second selector")
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "apache-svc", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"app": "apache"}))
			defer func() {
				err := cs.CoreV1().Services(svc2.Namespace).Delete(context.TODO(), svc2.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() error {
				_, err := status.L2ForService(ConfigUpdater.Client(), svc2)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred(), "Service matching second selector should be advertised")

			ginkgo.By("Creating a service matching neither selector")
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "other-svc", service.TrafficPolicyCluster,
				service.WithLabels(map[string]string{"app": "other"}))
			defer func() {
				err := cs.CoreV1().Services(svc3.Namespace).Delete(context.TODO(), svc3.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			Consistently(func() bool {
				_, err := status.L2ForService(ConfigUpdater.Client(), svc3)
				return pkgerr.IsNotFound(err)
			}, 5*time.Second, 1*time.Second).Should(BeTrue(), "Service not matching any selector should not be advertised")
		})
	})
})
