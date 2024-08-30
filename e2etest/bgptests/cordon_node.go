// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"

	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	"go.universe.tf/e2etest/pkg/ipfamily"
	testservice "go.universe.tf/e2etest/pkg/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("BGP Cordon Node", func() {
	var (
		cs            clientset.Interface
		testNamespace string
	)

	ginkgo.BeforeEach(func() {
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred(), "cleaning k8s api CRs failed")

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			Expect(err).NotTo(HaveOccurred(),
				fmt.Sprintf("cleaning frr config at %s failed", c.Name))
		}

		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "bgp")
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			dumpBGPInfo(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, cs, testNamespace)
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err := k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Describe("When create an IPV4 service and then cordon node", func() {
		var (
			svc      *corev1.Service
			allNodes *corev1.NodeList
		)

		ginkgo.BeforeEach(func() {
			_, svc = setupBGPService(cs, testNamespace, ipfamily.IPv4, []string{v4PoolAddresses}, FRRContainers, func(svc *corev1.Service) {
				testservice.TrafficPolicyCluster(svc)
			})
			testservice.ValidateDesiredLB(svc)

			var err error
			allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}

			err = k8s.CordonNode(cs, &allNodes.Items[0])
			Expect(err).NotTo(HaveOccurred(), "node was not cordoned")
		})

		ginkgo.AfterEach(func() {

			err := k8s.UnCordonNode(cs, &allNodes.Items[0])
			Expect(err).NotTo(HaveOccurred())

			for _, c := range FRRContainers {
				validateService(svc, allNodes.Items, c)
			}

			testservice.Delete(cs, svc)
		})

		ginkgo.It("service should be announced only from expected nodes", func() {
			checkServiceOnlyOnNodes(svc, allNodes.Items[1:], ipfamily.IPv4)
			checkServiceNotOnNodes(svc, allNodes.Items[:1], ipfamily.IPv4)
		})

	})
})

// when create service, when uncordon node, cordoned node should eventually not be next NextHops
// when uncordon node, when create service, cordoned node should not be ever as NextHops
// when create service, when cordon uncordon, service should come back working
