// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/e2etest/pkg/k8s"
	"go.universe.tf/metallb/e2etest/pkg/service"
	internalconfig "go.universe.tf/metallb/internal/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eservice "k8s.io/kubernetes/test/e2e/framework/service"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("LoadBalancer class", func() {
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

	f = framework.NewDefaultFramework("lbclass")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.BeforeEach(func() {
		cs = f.ClientSet

		ginkgo.By("Clearing any previous configuration")
		err := ConfigUpdater.Clean()
		framework.ExpectNoError(err)
	})

	ginkgo.Context("A service with loadbalancer class", func() {
		ginkgo.It("should not get an ip", func() {
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
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "empty",
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			framework.ExpectNoError(err)

			jig := e2eservice.NewTestJig(cs, f.Namespace.Name, "lbclass")
			svc, err := jig.CreateLoadBalancerService(10*time.Second, service.WithLoadbalancerClass("foo"))
			gomega.Expect(err).Should(gomega.MatchError(gomega.ContainSubstring("timed out waiting for the condition")))
			gomega.Expect(svc).To(gomega.BeNil())
		})
	})
})
