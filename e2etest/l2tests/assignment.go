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
	"go.universe.tf/e2etest/pkg/metallb"

	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const secondNamespace = "test-namespace"

var (
	firstNsLabels = map[string]string{
		"first-ns": "true",
	}
	secondNsLabels = map[string]string{
		"second-ns": "true",
	}
)

var _ = ginkgo.Describe("IP Assignment", func() {
	var cs clientset.Interface

	testNamespace := ""

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		err = k8s.DeleteNamespace(cs, secondNamespace)
		Expect(err).NotTo(HaveOccurred())
		err = k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		cs = k8sclient.New()
		var err error
		testNamespace, err = k8s.CreateTestNamespace(cs, "assignement")
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")
		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Updating the first namespace labels")
		Eventually(func() error {
			err := k8s.ApplyLabelsToNamespace(cs, testNamespace, firstNsLabels)
			return err
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		ginkgo.By("Creating a second namespace")

		err = k8s.CreateNamespace(cs, secondNamespace, secondNsLabels, func(ns *v1.Namespace) {
			// we also need to set the pod security policy for the namespace
			ns.Labels[admissionapi.EnforceLevelLabel] = string(admissionapi.LevelPrivileged)
		})
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("IPV4 Assignment", func() {
		ginkgo.DescribeTable("should remove the ip from a service assign it to a free one when", func(modify func(svc *v1.Service) error) {
			ip, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			Expect(err).NotTo(HaveOccurred())

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "singleip-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								fmt.Sprintf("%s/32", ip),
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			jig := jigservice.NewTestJig(cs, testNamespace, "singleip")
			svc, err := jig.CreateLoadBalancerService(context.TODO(), service.TrafficPolicyCluster)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating another service")
			svc1, err := jig.CreateTCPService(context.TODO(), func(svc *v1.Service) {
				svc.Spec.Type = v1.ServiceTypeLoadBalancer
				svc.Name = "singleip1"
			})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				service.Delete(cs, svc1)
			}()

			Consistently(func() int {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(s.Status.LoadBalancer.Ingress)
			}, 5*time.Second, 1*time.Second).Should(BeZero())

			err = modify(svc)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Changing the service type so the ip is free to be used again")
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Checking the second service gets the ip assigned")

			Eventually(func() string {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if len(s.Status.LoadBalancer.Ingress) == 0 {
					return ""
				}
				return s.Status.LoadBalancer.Ingress[0].IP
			}, time.Minute, 1*time.Second).Should(Equal(ip))
		},
			ginkgo.Entry("changing the service type to clusterIP",
				func(svc *v1.Service) error {
					svc.Spec.Type = v1.ServiceTypeClusterIP
					_, err := cs.CoreV1().Services(svc.Namespace).Update(context.Background(), svc, metav1.UpdateOptions{})
					return err
				}),
			ginkgo.Entry("deleting the service",
				func(svc *v1.Service) error {
					err := cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
					return err
				}))

		ginkgo.It("should preseve the same external ip after controller restart", func() {
			const numOfRestarts = 5
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "assignment-controller-reset-test-pool",
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{"192.168.10.100/32", "192.168.20.200/32"},
						},
					},
				},
			}
			ginkgo.By("Updating the configuration with the initial pool")
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("creating 4 LB services")
			jig := jigservice.NewTestJig(cs, testNamespace, "service-a")
			serviceA, err := jig.CreateLoadBalancerService(context.TODO(), nil)
			Expect(err).NotTo(HaveOccurred())
			defer service.Delete(cs, serviceA)
			service.ValidateDesiredLB(serviceA)

			jig = jigservice.NewTestJig(cs, testNamespace, "service-b")
			serviceB, err := jig.CreateLoadBalancerService(context.TODO(), nil)
			Expect(err).NotTo(HaveOccurred())
			defer service.Delete(cs, serviceB)
			service.ValidateDesiredLB(serviceB)

			jig = jigservice.NewTestJig(cs, testNamespace, "service-c")
			serviceC, err := jig.CreateLoadBalancerServiceWaitForClusterIPOnly(nil)
			Expect(err).NotTo(HaveOccurred())
			defer service.Delete(cs, serviceC)

			jig = jigservice.NewTestJig(cs, testNamespace, "service-d")
			serviceD, err := jig.CreateLoadBalancerServiceWaitForClusterIPOnly(nil)
			Expect(err).NotTo(HaveOccurred())
			defer service.Delete(cs, serviceD)

			restartAndAssert := func() {
				metallb.RestartController(cs)
				Consistently(func() error {
					serviceA, err = cs.CoreV1().Services(serviceA.Namespace).Get(context.TODO(), serviceA.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())

					err = service.ValidateAssignedWith(serviceA, "192.168.10.100")
					if err != nil {
						return err
					}
					serviceB, err = cs.CoreV1().Services(serviceB.Namespace).Get(context.TODO(), serviceB.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())

					err = service.ValidateAssignedWith(serviceB, "192.168.20.200")
					if err != nil {
						return err
					}

					return nil
				}, 10*time.Second, 2*time.Second).ShouldNot(HaveOccurred())
			}

			ginkgo.By("restarting the controller and validating that the service keeps the same ip")
			for i := 0; i < numOfRestarts; i++ {
				restartAndAssert()
			}
		})
	})

	ginkgo.Context("IPV4 removing pools", func() {
		var pool1 metallbv1beta1.IPAddressPool
		var pool2 metallbv1beta1.IPAddressPool

		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.BeforeEach(func() {
			pool1 = metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{testNamespace}},
				},
			}
			pool2 = metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{pool1, pool2},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("removes all pools", func() {
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-1")
			defer func() {
				service.Delete(cs, svc1)
			}()

			ginkgo.By("Validating LoadBalancer IP is allocated from pool1")
			err := config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("deleting all pools")
			err = ConfigUpdater.Client().DeleteAllOf(context.Background(), &metallbv1beta1.IPAddressPool{}, client.InNamespace(ConfigUpdater.Namespace()))
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("validate LoadBalancer IP is removed from the svc")
			Eventually(func() int {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(s.Status.LoadBalancer.Ingress)
			}, time.Minute, 1*time.Second).Should(Equal(0))
		})

		ginkgo.It("reallocates svc after deleting a pool", func() {
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-1")
			defer func() {
				service.Delete(cs, svc1)
			}()

			ginkgo.By("validate LoadBalancer IP is allocated from pool1")
			err := config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("deleting pool 1")
			p := &metallbv1beta1.IPAddressPool{}
			err = ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Namespace: ConfigUpdater.Namespace(), Name: pool1.Name}, p)
			Expect(err).NotTo(HaveOccurred())
			err = ConfigUpdater.Client().Delete(context.Background(), p)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("validate LoadBalancer IP is re-allocated from pool2")
			Eventually(func() error {
				svc1, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool2}, jigservice.GetIngressPoint(
					&svc1.Status.LoadBalancer.Ingress[0]))
				return err
			}, time.Minute, 1*time.Second).ShouldNot(HaveOccurred())
		})
	})

	ginkgo.Context("IPV4 - Validate service allocation in address pools", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("with namespace", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{testNamespace}},
				},
			}
			namespacePoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10, Namespaces: []string{testNamespace}},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", testNamespace)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespacePoolWithHigherPriority, namespacePoolNoPriority},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-1")
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-2")
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-3")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithHigherPriority}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, jigservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, jigservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("with namespace and namespace labels", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{testNamespace}},
				},
			}
			testNs, err := cs.CoreV1().Namespaces().Get(context.Background(), testNamespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			testNs.Labels["foo1"] = "bar1"
			testNs.Labels["foo2"] = "bar2"
			_, err = cs.CoreV1().Namespaces().Update(context.Background(), testNs, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			namespaceLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{
						Priority:           10,
						NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}}},
					},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", testNamespace)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespaceLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err = ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-label-pool-1")
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-label-pool-2")
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-label-pool-3")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespaceLabelPoolWithHigherPriority}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, jigservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, jigservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("with service label", func() {
			svcLabelPoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{
						Priority:         20,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}},
					},
				},
			}
			svcLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{
						Priority:         10,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}},
					},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", testNamespace)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{svcLabelPoolWithLowerPriority, svcLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svcTweakWithLabel := func(svc *v1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithHigherPriority}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithLowerPriority}, jigservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, jigservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("with namespace and service label", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-svc-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{testNamespace}},
				},
			}
			svcLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-svc-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{
						Priority:         10,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}},
					},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", testNamespace)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, svcLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svcTweakWithLabel := func(svc *v1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-ns-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "svc-ns-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "svc-ns-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithHigherPriority}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, jigservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, jigservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("with namespace with labels", func() {
			firstNamespacePool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "first-ns-labels-ip-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10, NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: firstNsLabels}}},
				},
			}
			secondNamespacePool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "second-ns-labels-ip-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.30.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10, NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: secondNsLabels}}},
				},
			}

			noNamespacePool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "no-ns-labels-ip-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.40.0/32",
					},
				},
			}

			newLabels := make(map[string]string)
			for key, value := range secondNsLabels {
				newLabels[key] = value
			}
			newLabels["newLabel"] = "true"

			secondNamespacePoolHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "second-ns-labels-higher-priority-ip-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.50.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 5, NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: newLabels}}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{firstNamespacePool, secondNamespacePool, secondNamespacePoolHigherPriority, noNamespacePool},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			svc1, _ := service.CreateWithBackend(cs, secondNamespace, "second-ns-service")
			svc2, _ := service.CreateWithBackend(cs, testNamespace, "default-ns-service")
			svc3, _ := service.CreateWithBackend(cs, testNamespace, "default-ns-service2")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{secondNamespacePool}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{firstNamespacePool}, jigservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{noNamespacePool}, jigservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("updating second namespace labels to match higher priority pool")
			ns, err := cs.CoreV1().Namespaces().Get(context.Background(), secondNamespace, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			ns.Labels = newLabels
			_, err = cs.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("creating a second svc that should get an ip from the higher priority pool")
			svc4, _ := service.CreateWithBackend(cs, secondNamespace, "second-ns-service2")
			defer func() {
				service.Delete(cs, svc4)
			}()

			ginkgo.By("validate LoadBalancer IP is allocated from higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{secondNamespacePoolHigherPriority}, jigservice.GetIngressPoint(
				&svc4.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	ginkgo.Context("PREFER-DUALSTACK - only 1 stack available", func() {
		const v4PoolAddresses = "192.168.10.100/32"
		const v6PoolAddresses = "fc00:f853:0ccd:e799::/124"
		const v4PoolAddresses2 = "192.168.11.100/32"
		const v6PoolAddresses2 = "fc00:f853:0ccd:e800::/124"
		IPFamilyPolicyPreferDualStack := v1.IPFamilyPolicyPreferDualStack
		ginkgo.It("Should select dual-stack pool if available", func() {
			poolv4 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-ipv4-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v4PoolAddresses,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			poolv6 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-ipv6-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v6PoolAddresses,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			poolDual := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-dualstack-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v4PoolAddresses2,
						v6PoolAddresses2,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{poolv4, poolv6, poolDual},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("Creating the service with PreferDualStack policy")
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-1", func(svc *v1.Service) {
				svc.Spec.IPFamilyPolicy = &IPFamilyPolicyPreferDualStack
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Validating that 2 IPs are assigned to the service")
			Eventually(func() int {
				svc1, err = cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(svc1.Status.LoadBalancer.Ingress)
			}, 5*time.Minute, 1*time.Second).Should(Equal(2))
			ginkgo.By("validate LoadBalancer IPs are allocated from poolDual")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{poolDual}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{poolDual}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[1]))
			Expect(err).NotTo(HaveOccurred())
		})
		ginkgo.It("Should prefer primary ip family among single-stack pools", func() {
			poolv4 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-ipv4-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v4PoolAddresses,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			poolv6 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-ipv6-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v6PoolAddresses,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{poolv4, poolv6},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("Creating the service with PreferDualStack policy")
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-2", func(svc *v1.Service) {
				svc.Spec.IPFamilyPolicy = &IPFamilyPolicyPreferDualStack
				svc.Spec.IPFamilies = []v1.IPFamily{v1.IPv6Protocol, v1.IPv4Protocol}
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Validating that 1 IP is assigned to the service")
			Eventually(func() int {
				svc1, err = cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(svc1.Status.LoadBalancer.Ingress)
			}, 5*time.Minute, 1*time.Second).Should(Equal(1))
			ginkgo.By("validate LoadBalancer IP is allocated from ipv6 pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{poolv6}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
		})
		ginkgo.It("Additional ip should be assigned when the assigned 1-stack pool becomes dual-stack", func() {
			pool1 := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-dualstack-pool"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						v4PoolAddresses,
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{testNamespace}},
				},
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{pool1},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Creating the service with PreferDualStack policy")
			svc1, _ := service.CreateWithBackend(cs, testNamespace, "svc-test-ns-pool-3", func(svc *v1.Service) {
				svc.Spec.IPFamilyPolicy = &IPFamilyPolicyPreferDualStack
			})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("Validating that 1 IP is assigned to the service")
			Eventually(func() int {
				svc1, err = cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(svc1.Status.LoadBalancer.Ingress)
			}, 5*time.Minute, 1*time.Second).Should(Equal(1))
			ginkgo.By("validate LoadBalancer IP is allocated from pool1")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			Expect(err).NotTo(HaveOccurred())
			ginkgo.By("Updating pool1 to include both v4 and v6 addresses")
			pool1.Spec.Addresses = []string{v4PoolAddresses, v6PoolAddresses}
			err = ConfigUpdater.Update(config.Resources{Pools: []metallbv1beta1.IPAddressPool{pool1}})
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(2 * time.Minute)

			ginkgo.By("Validating that an additional IP is assigned to the service")
			Eventually(func() int {
				svc1, err = cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return len(svc1.Status.LoadBalancer.Ingress)
			}, 5*time.Minute, 1*time.Second).Should(Equal(2))
			ginkgo.By("validate second LoadBalancer IP is allocated from pool1")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, jigservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[1]))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
