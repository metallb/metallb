// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/config"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/service"
	internalconfig "go.universe.tf/metallb/internal/config"

	testservice "go.universe.tf/metallb/e2etest/pkg/service"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("IP Assignment", func() {
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

	f = framework.NewDefaultFramework("assignment")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet

		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("IPV4 Assignment", func() {
		table.DescribeTable("should remove the ip from a service assign it to a free one when", func(modify func(svc *v1.Service) error) {
			ip, err := config.GetIPFromRangeByIndex(IPV4ServiceRange, 0)
			framework.ExpectNoError(err)

			resources := internalconfig.ClusterResources{
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
			svc, err := jig.CreateLoadBalancerService(10*time.Second, service.TrafficPolicyCluster)
			framework.ExpectNoError(err)

			ginkgo.By("Creating another service")
			svc1, err := jig.CreateTCPService(func(svc *v1.Service) {
				svc.Spec.Type = v1.ServiceTypeLoadBalancer
				svc.Name = "singleip1"
			})
			framework.ExpectNoError(err)
			defer func() {
				testservice.Delete(cs, svc1)
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
			table.Entry("changing the service type to clusterIP",
				func(svc *v1.Service) error {
					svc.Spec.Type = v1.ServiceTypeClusterIP
					_, err := cs.CoreV1().Services(svc.Namespace).Update(context.Background(), svc, metav1.UpdateOptions{})
					return err
				}),
			table.Entry("deleting the service",
				func(svc *v1.Service) error {
					err := cs.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
					return err
				}))
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
			defaultPool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-pool-3"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
				},
			}
			resources := internalconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespacePoolWithHigherPriority, defaultPool},
			}

			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc1, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-1")
			svc2, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-2")
			svc3, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-pool-3")
			defer func() {
				testservice.Delete(cs, svc1)
				testservice.Delete(cs, svc2)
				testservice.Delete(cs, svc3)
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
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{defaultPool}, e2eservice.GetIngressPoint(
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
			namespaceLabelPoolWithHigherPriority := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-2"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.10.0/32",
					},
					AllocateTo: &metallbv1beta1.ServiceAllocation{Priority: 10, NamespaceSelectors: []metav1.LabelSelector{{MatchLabels: f.Namespace.Labels}}},
				},
			}
			defaultPool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-label-pool-3"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
				},
			}

			resources := internalconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, namespaceLabelPoolWithHigherPriority, defaultPool},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svc1, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-1")
			svc2, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-2")
			svc3, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-ns-label-pool-3")
			defer func() {
				testservice.Delete(cs, svc1)
				testservice.Delete(cs, svc2)
				testservice.Delete(cs, svc3)
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
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{defaultPool}, e2eservice.GetIngressPoint(
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
			defaultPool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-svc-label-pool-3"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
				},
			}

			resources := internalconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{svcLabelPoolWithLowerPriority, svcLabelPoolWithHigherPriority, defaultPool},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svcTweakWithLabel := func(svc *corev1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-test-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				testservice.Delete(cs, svc1)
				testservice.Delete(cs, svc2)
				testservice.Delete(cs, svc3)
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
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{defaultPool}, e2eservice.GetIngressPoint(
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
			defaultPool := metallbv1beta1.IPAddressPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-ns-svc-label-pool-3"},
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses: []string{
						"192.168.20.0/32",
					},
				},
			}

			resources := internalconfig.ClusterResources{
				Pools: []metallbv1beta1.IPAddressPool{namespacePoolWithLowerPriority, svcLabelPoolWithHigherPriority, defaultPool},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			svcTweakWithLabel := func(svc *corev1.Service) {
				if svc.Labels == nil {
					svc.Labels = make(map[string]string)
				}
				svc.Labels["test"] = "e2e"
			}
			svc1, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-1", svcTweakWithLabel)
			svc2, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-2", svcTweakWithLabel)
			svc3, _ := testservice.CreateWithBackend(cs, f.Namespace.Name, "svc-ns-svc-label-pool-3", svcTweakWithLabel)
			defer func() {
				testservice.Delete(cs, svc1)
				testservice.Delete(cs, svc2)
				testservice.Delete(cs, svc3)
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
			err = config.ValidateIPInRange([]metallbv1beta1.IPAddressPool{defaultPool}, e2eservice.GetIngressPoint(
				&svc3.Status.LoadBalancer.Ingress[0]))
			framework.ExpectNoError(err)
		})
	})
})
