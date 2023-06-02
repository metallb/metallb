/*
Copyright 2016 The Kubernetes Authors.
Copyright 2021 The MetalLB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// https://github.com/kubernetes/kubernetes/blob/92aff21558831b829fbc8cbca3d52edc80c01aa3/test/e2e/network/loadbalancer.go#L878

package status

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/openshift-kni/k8sreporter"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

var (
	ConfigUpdater config.Updater
	Reporter      *k8sreporter.KubernetesReporter
)

var _ = ginkgo.Describe("IPAddressPool status", func() {
	var cs clientset.Interface
	var f *framework.Framework

	f = framework.NewDefaultFramework("status")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet

		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("Service updates", func() {
		ginkgo.It("performs changes on services and checks pool status", func() {
			pool1 := "pool1"
			pool2 := "pool2"

			resources := config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pool1,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"20.20.20.0/24",
								"2000::/121",
							},
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			ginkgo.By("Creating a LB service and checking status of pool 1")
			jig := e2eservice.NewTestJig(cs, f.Namespace.Name, "test-svc1")
			svc1, err := jig.CreateLoadBalancerService(10*time.Second, service.DualStack)
			framework.ExpectNoError(err)

			defer func() {
				service.Delete(cs, svc1)
			}()

			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool1, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  1,
				AssignedIPv6:  1,
				AvailableIPv4: 255,
				AvailableIPv6: 127,
			}))

			ginkgo.By("Creating a second LB service and checking status of pool 1")
			jig = e2eservice.NewTestJig(cs, f.Namespace.Name, "test-svc2")
			svc2, err := jig.CreateLoadBalancerService(10*time.Second, service.DualStack)
			framework.ExpectNoError(err)

			defer func() {
				service.Delete(cs, svc2)
			}()

			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool1, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  2,
				AssignedIPv6:  2,
				AvailableIPv4: 254,
				AvailableIPv6: 126,
			}))

			ginkgo.By("Moving the second LB service to pool2 and checking status of pool 1 and 2")
			resources = config.Resources{
				Pools: []metallbv1beta1.IPAddressPool{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pool1,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"20.20.20.0/24",
								"2000::/121",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: pool2,
						},
						Spec: metallbv1beta1.IPAddressPoolSpec{
							Addresses: []string{
								"30.30.30.0/24",
								"3000::/121",
							},
						},
					},
				},
			}
			err = ConfigUpdater.Update(resources)
			if apierrors.IsConflict(err) {
				err = ConfigUpdater.Update(resources)
			}
			framework.ExpectNoError(err)

			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool1, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  2,
				AssignedIPv6:  2,
				AvailableIPv4: 254,
				AvailableIPv6: 126,
			}))
			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool2, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  0,
				AssignedIPv6:  0,
				AvailableIPv4: 256,
				AvailableIPv6: 128,
			}))

			svc2, err = jig.UpdateService(service.WithSpecificPool(pool2))
			framework.ExpectNoError(err)

			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool1, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  1,
				AssignedIPv6:  1,
				AvailableIPv4: 255,
				AvailableIPv6: 127,
			}))
			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool2, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  1,
				AssignedIPv6:  1,
				AvailableIPv4: 255,
				AvailableIPv6: 127,
			}))

			ginkgo.By("changing the service type of second LB service and checking pool 2")
			err = jig.ChangeServiceType("NodePort", 10*time.Second)
			framework.ExpectNoError(err)

			gomega.Eventually(func() metallbv1beta1.IPAddressPoolStatus {
				p := metallbv1beta1.IPAddressPool{}
				err := ConfigUpdater.Client().Get(context.Background(), client.ObjectKey{Name: pool2, Namespace: ConfigUpdater.Namespace()}, &p)
				framework.ExpectNoError(err)
				return p.Status
			}, 5*time.Second, 1*time.Second).Should(gomega.Equal(metallbv1beta1.IPAddressPoolStatus{
				AssignedIPv4:  0,
				AssignedIPv6:  0,
				AvailableIPv4: 256,
				AvailableIPv6: 128,
			}))
		})
	})
})
