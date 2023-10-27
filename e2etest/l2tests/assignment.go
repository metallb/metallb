// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/metallb"

	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
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

	var f *framework.Framework
	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		// Clean previous configuration.
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
		err = k8s.DeleteNamespace(cs, secondNamespace)
		framework.ExpectNoError(err)
	})

	f = framework.NewDefaultFramework("assignment")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet

		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)

		ginkgo.By("Updating the first namespace labels")
		gomega.Eventually(func() error {
			err := k8s.ApplyLabelsToNamespace(cs, f.Namespace.Name, firstNsLabels)
			return err
		}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

		ginkgo.By("Creating a second namespace")

		err = k8s.CreateNamespace(cs, secondNamespace, secondNsLabels, func(ns *v1.Namespace) {
			// we also need to set the pod security policy for the namespace
			ns.Labels[admissionapi.EnforceLevelLabel] = string(admissionapi.LevelPrivileged)
		})
		framework.ExpectNoError(err)

	})

	ginkgo.Context("IPV4 Assignment", func() {
		ginkgo.DescribeTable("should remove the ip from a service assign it to a free one when", func(modify func(svc *v1.Service) error) {
			ip, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			framework.ExpectNoError(err)

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
			framework.ExpectNoError(err)

			jig := e2eservice.NewTestJig(cs, f.Namespace.Name, "singleip")
			svc, err := jig.CreateLoadBalancerService(context.TODO(), 10*time.Second, service.TrafficPolicyCluster)
			framework.ExpectNoError(err)

			ginkgo.By("Creating another service")
			svc1, err := jig.CreateTCPService(context.TODO(), func(svc *v1.Service) {
				svc.Spec.Type = v1.ServiceTypeLoadBalancer
				svc.Name = "singleip1"
			})
			framework.ExpectNoError(err)
			defer func() {
				service.Delete(cs, svc1)
			}()

			gomega.Consistently(func() int {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				return len(s.Status.LoadBalancer.Ingress)
			}, 5*time.Second, 1*time.Second).Should(gomega.BeZero())

			err = modify(svc)
			framework.ExpectNoError(err)

			ginkgo.By("Changing the service type so the ip is free to be used again")
			framework.ExpectNoError(err)

			ginkgo.By("Checking the second service gets the ip assigned")

			gomega.Eventually(func() string {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				if len(s.Status.LoadBalancer.Ingress) == 0 {
					return ""
				}
				return s.Status.LoadBalancer.Ingress[0].IP
			}, time.Minute, 1*time.Second).Should(gomega.Equal(ip))
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
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("creating 4 LB services")
			jig := e2eservice.NewTestJig(cs, f.Namespace.Name, "service-a")
			serviceA, err := jig.CreateLoadBalancerService(context.TODO(), 30*time.Second, nil)
			framework.ExpectNoError(err)
			defer service.Delete(cs, serviceA)
			service.ValidateDesiredLB(serviceA)

			jig = e2eservice.NewTestJig(cs, f.Namespace.Name, "service-b")
			serviceB, err := jig.CreateLoadBalancerService(context.TODO(), 30*time.Second, nil)
			framework.ExpectNoError(err)
			defer service.Delete(cs, serviceB)
			service.ValidateDesiredLB(serviceB)

			jig = e2eservice.NewTestJig(cs, f.Namespace.Name, "service-c")
			serviceC, err := jig.CreateLoadBalancerServiceWaitForClusterIPOnly(nil)
			framework.ExpectNoError(err)
			defer service.Delete(cs, serviceC)

			jig = e2eservice.NewTestJig(cs, f.Namespace.Name, "service-d")
			serviceD, err := jig.CreateLoadBalancerServiceWaitForClusterIPOnly(nil)
			framework.ExpectNoError(err)
			defer service.Delete(cs, serviceD)

			restartAndAssert := func() {
				metallb.RestartController(cs)
				gomega.Consistently(func() error {
					serviceA, err = cs.CoreV1().Services(serviceA.Namespace).Get(context.TODO(), serviceA.Name, metav1.GetOptions{})
					framework.ExpectNoError(err)

					err = service.ValidateAssignedWith(serviceA, "192.168.10.100")
					if err != nil {
						return err
					}
					serviceB, err = cs.CoreV1().Services(serviceB.Namespace).Get(context.TODO(), serviceB.Name, metav1.GetOptions{})
					framework.ExpectNoError(err)

					err = service.ValidateAssignedWith(serviceB, "192.168.20.200")
					if err != nil {
						return err
					}

					return nil
				}, 10*time.Second, 2*time.Second).ShouldNot(gomega.HaveOccurred())
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
			framework.ExpectNoError(err)
		})

		ginkgo.BeforeEach(func() {
			pool1 = metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{f.Namespace.Name}},
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
			framework.ExpectNoError(err)
		})

		ginkgo.It("removes all pools", func() {
			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-1")
			defer func() {
				service.Delete(cs, svc1)
			}()

			ginkgo.By("validate LoadBalancer IP is allocated from pool1")
			err := config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)

			ginkgo.By("deleting all pools")
			err = ConfigUpdater.Client().DeleteAllOf(context.Background(), &metallbv1beta1.IPAddressPool{}, client.InNamespace(ConfigUpdater.Namespace()))
			framework.ExpectNoError(err)

			ginkgo.By("validate LoadBalancer IP is removed from the svc")
			gomega.Eventually(func() int {
				s, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				return len(s.Status.LoadBalancer.Ingress)
			}, time.Minute, 1*time.Second).Should(gomega.Equal(0))
		})

		ginkgo.It("reallocates svc after deleting a pool", func() {
			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-1")
			defer func() {
				service.Delete(cs, svc1)
			}()

			ginkgo.By("validate LoadBalancer IP is allocated from pool1")
			err := config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool1}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)

			ginkgo.By("deleting pool 1")
			p := &metallbv1beta1.IPAddressPool{}
			err = ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Namespace: ConfigUpdater.Namespace(), Name: pool1.Name}, p)
			framework.ExpectNoError(err)
			err = ConfigUpdater.Client().Delete(context.Background(), p)
			framework.ExpectNoError(err)

			ginkgo.By("validate LoadBalancer IP is re-allocated from pool2")
			gomega.Eventually(func() error {
				svc1, err := cs.CoreV1().Services(svc1.Namespace).Get(context.Background(), svc1.Name, metav1.GetOptions{})
				framework.ExpectNoError(err)
				err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{pool2}, e2eservice.GetIngressPoint(
					&svc1.Status.LoadBalancer.Ingress[0]))
				return err
			}, time.Minute, 1*time.Second).ShouldNot(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("IPV4 - Validate service allocation in address pools", func() {
		ginkgo.AfterEach(func() {
			// Clean previous configuration.
			err := ConfigUpdater.Clean()
			framework.ExpectNoError(err)
		})

		ginkgo.It("with namespace", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{f.Namespace.Name}},
				},
			}
			namespacePoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10, Namespaces: []string{f.Namespace.Name}},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", f.Namespace.Name)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{f.Namespace.Name}},
				},
			}
			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespacePoolWithHigherPriority, namespacePoolNoPriority},
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-1")
			svc2, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-2")
			svc3, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-3")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithHigherPriority}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, e2eservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
		})

		ginkgo.It("with namespace and namespace labels", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{f.Namespace.Name}},
				},
			}
			testNs, err := cs.CoreV1().Namespaces().Get(context.Background(), f.Namespace.Name, metav1.GetOptions{})
			framework.ExpectNoError(err)
			testNs.Labels["foo1"] = "bar1"
			testNs.Labels["foo2"] = "bar2"
			_, err = cs.CoreV1().Namespaces().Update(context.Background(), testNs, metav1.UpdateOptions{})
			framework.ExpectNoError(err)
			namespaceLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10,
						NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"foo1": "bar1", "foo2": "bar2"}}}},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", f.Namespace.Name)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{f.Namespace.Name}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespaceLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err = ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-1")
			svc2, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-2")
			svc3, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-3")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespaceLabelPoolWithHigherPriority}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, e2eservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
		})

		ginkgo.It("with service label", func() {
			svcLabelPoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}}},
				},
			}
			svcLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}}},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", f.Namespace.Name)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{f.Namespace.Name}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{svcLabelPoolWithLowerPriority, svcLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svcTweakWithLabel := func(svc *v1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithHigherPriority}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithLowerPriority}, e2eservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
		})

		ginkgo.It("with namespace and service label", func() {
			namespacePoolWithLowerPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-svc-label-pool-1"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.5.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 20, Namespaces: []string{f.Namespace.Name}},
				},
			}
			svcLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-svc-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10,
						ServiceSelectors: []metav1.LabelSelector{{MatchLabels: map[string]string{"test": "e2e"}}}},
				},
			}
			namespacePoolNoPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-%s-ip-pool", f.Namespace.Name)},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Namespaces: []string{f.Namespace.Name}},
				},
			}

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, svcLabelPoolWithHigherPriority, namespacePoolNoPriority},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svcTweakWithLabel := func(svc *v1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := service.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{svcLabelPoolWithHigherPriority}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority}, e2eservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{namespacePoolNoPriority}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
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
			framework.ExpectNoError(err)

			svc1, _ := service.CreateWithBackend(cs, secondNamespace, "second-ns-service")
			svc2, _ := service.CreateWithBackend(cs, f.Namespace.Name, "default-ns-service")
			svc3, _ := service.CreateWithBackend(cs, f.Namespace.Name, "default-ns-service2")
			defer func() {
				service.Delete(cs, svc1)
				service.Delete(cs, svc2)
				service.Delete(cs, svc3)
			}()

			// The createWithBackend method always wait for service to acquire an ingress IP, so
			// just validate service ingress ip address are assigned from appropriate ip
			// address pool.
			ginkgo.By("validate LoadBalancer IP is allocated from 1st higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{secondNamespacePool}, e2eservice.GetIngressPoint(
				&svc1.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from 2nd higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{firstNamespacePool}, e2eservice.GetIngressPoint(
				&svc2.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
			ginkgo.By("validate LoadBalancer IP is allocated from default address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{noNamespacePool}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)

			ginkgo.By("updating second namespace labels to match higher priority pool")
			ns, err := cs.CoreV1().Namespaces().Get(context.Background(), secondNamespace, metav1.GetOptions{})
			framework.ExpectNoError(err)
			ns.Labels = newLabels
			_, err = cs.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
			framework.ExpectNoError(err)

			ginkgo.By("creating a second svc that should get an ip from the higher priority pool")
			svc4, _ := service.CreateWithBackend(cs, secondNamespace, "second-ns-service2")
			defer func() {
				service.Delete(cs, svc4)
			}()

			ginkgo.By("validate LoadBalancer IP is allocated from higher priority address pool")
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{secondNamespacePoolHigherPriority}, e2eservice.GetIngressPoint(
				&svc4.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
		})
	})
})
