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

	jigservice "go.universe.tf/e2etest/pkg/jigservice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("LoadBalancer class", func() {
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
		cs = k8sclient.New()
		var err error
		testNamespace, err = k8s.CreateTestNamespace(cs, "lbclass")
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("Clearing any previous configuration")
		err = ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("A service with loadbalancer class", func() {
		ginkgo.It("should not get an ip", func() {
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
				L2Advs: []metallbv1beta1.L2Advertisement{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "empty",
						},
					},
				},
			}
			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())

			jig := jigservice.NewTestJig(cs, testNamespace, "lbclass")
			svc, err := jig.CreateLoadBalancerServiceWithTimeout(context.TODO(), 10*time.Second, service.WithLoadbalancerClass("foo"))

			Expect(err).Should(MatchError(ContainSubstring("timed out waiting for service \"lbclass\" to have a load balancer")))
			Expect(svc).To(BeNil())
		})
	})
})
