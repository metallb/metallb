// SPDX-License-Identifier:Apache-2.0

package bgptests

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"

	frrconfig "go.universe.tf/e2etest/pkg/frr/config"
	"go.universe.tf/e2etest/pkg/ipfamily"
	testservice "go.universe.tf/e2etest/pkg/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

var _ = ginkgo.Describe("FRR BGP Exclude Node", func() {
	var (
		ann           = [2]string{"ocp-mc", "rebooting"}
		allNodes      *corev1.NodeList
		cs            clientset.Interface
		svc           *corev1.Service
		testNamespace string
	)
	wait := func() {
		Eventually(func() error {
			pods, err := metallb.SpeakerPods(cs)
			if err != nil {
				return err
			}

			for _, p := range pods {
				if !k8s.PodIsReady(p) {
					return fmt.Errorf("speaker pods are not ready")
				}
			}

			return nil
		}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred(), "no downtime until speakers are ready")
	}

	ginkgo.BeforeEach(func() {
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred(), "cleaning k8s api CRs failed")

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			Expect(err).NotTo(HaveOccurred(),
				fmt.Sprintf("cleaning frr config at %s failed", c.Name))
		}

		data := `metadata:
      annotations:
        %s: %s
      labels:
        foo: "bar"`

		cm := map[string]string{"excludeNodePattern.yaml": fmt.Sprintf(data, ann[0], ann[1])}

		cs = k8sclient.New()
		err = k8s.CreateConfigmap(cs, "excludenodepatterns", metallb.Namespace, cm)
		Expect(err).NotTo(HaveOccurred())

		// Restart pods is required to read the configMap
		err = metallb.RestartSpeakerPods(cs)
		Expect(err).NotTo(HaveOccurred())
		wait()

		testNamespace, err = k8s.CreateTestNamespace(cs, "bgp")
		Expect(err).NotTo(HaveOccurred())

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, svc = setupBGPService(cs, testNamespace, ipfamily.IPv4, []string{v4PoolAddresses}, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
		})
		testservice.ValidateDesiredLB(svc)
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			// TODO: dumpBGPInfo on frr-k8s fails because we restart speaker pods
			//dumpBGPInfo(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, cs, testNamespace)
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		testservice.Delete(cs, svc)
		err := k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		err = k8s.RemoveConfigmap(cs, "excludenodepatterns", metallb.Namespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.It("service should be announced only from expected nodes", func() {
		ginkgo.By("checking that all nodes advertise prefix")
		checkServiceOnlyOnNodes(svc, allNodes.Items, ipfamily.IPv4)

		ginkgo.By("adding annotation")
		k8s.AddAnnotationToNode(allNodes.Items[0].Name, ann[0], ann[1], cs)

		ginkgo.By("checking that specific nodes advertise prefix after annotation")
		checkServiceOnlyOnNodes(svc, allNodes.Items[1:], ipfamily.IPv4)
		checkServiceNotOnNodes(svc, allNodes.Items[:1], ipfamily.IPv4)

		defer func() {
			ginkgo.By("removing the annotation of the node")
			k8s.RemoveAnnotationFromNode(allNodes.Items[0].Name, ann[0], cs)

			ginkgo.By("checking that all nodes advertise prefix as before")
			checkServiceOnlyOnNodes(svc, allNodes.Items, ipfamily.IPv4)
		}()
	})
})
