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
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("L2 Cordon Node", func() {
	emptyL2Advertisement := metallbv1beta1.L2Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}

	var (
		cs            clientset.Interface
		testNamespace string
	)

	ginkgo.BeforeEach(func() {
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred(), "cleaning k8s api CRs failed")
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2test-cordon-node")
		Expect(err).NotTo(HaveOccurred())

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
			L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Advertisement},
		}

		err = ConfigUpdater.Update(resources)
		Expect(err).NotTo(HaveOccurred())

	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err := k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Describe("When create an IPV4 service and then cordon node", func() {
		var (
			svc       *corev1.Service
			allNodes  *corev1.NodeList
			nodeToCordon *corev1.Node
		)

		ginkgo.BeforeEach(func() {
			svc, _ = service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)

			var err error
			allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				var err error
				nodeToCordon, err = nodeForService(svc, allNodes.Items)
				if err != nil {
					return err
				}
				return nil
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			ginkgo.By(fmt.Sprintf("cordon node %s", nodeToCordon.Name))
			err = k8s.CordonNode(cs, nodeToCordon)
			Expect(err).NotTo(HaveOccurred(), "k8s api call to cordon node failed")
			Eventually(func() bool {
				ret, err := k8s.IsNodeCordoned(cs, nodeToCordon)
				Expect(err).NotTo(HaveOccurred())
				return ret
			}, 30*time.Second, time.Second).Should(BeTrue(), "node.spec.Unschedulable not true")

		})

		ginkgo.AfterEach(func() {
			ginkgo.By(fmt.Sprintf("uncordon node %s", nodeToCordon.Name))
			err := k8s.UnCordonNode(cs, nodeToCordon)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).Should(Equal(nodeToCordon.Name))

			err = cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("service should be announced only from expected nodes", func() {
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, time.Minute, time.Second).ShouldNot(Equal(nodeToCordon.Name))
		})
	})
})
