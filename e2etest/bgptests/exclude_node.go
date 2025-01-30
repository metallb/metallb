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

// configmap name is hardcoded to manifests
const excludeCM = "nodesexclusionpatterns"

var _ = ginkgo.Describe("FRR BGP ExcludeNode IPV4", func() {
	var (
		ann           = [2]string{"ocp-mc", "rebooting"}
		allNodes      *corev1.NodeList
		cs            clientset.Interface
		svc           *corev1.Service
		testNamespace string
	)

	ginkgo.BeforeEach(func() {
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred(), "cleaning k8s api CRs failed")
		cs = k8sclient.New()

		data := fmt.Sprintf("annotationToExclude:\n  %s: %s", ann[0], ann[1])
		cm := map[string]string{"pattern.yaml": data}
		ginkgo.By("adding the nodesexclusionpatterns configmap")
		err = k8s.CreateConfigmap(cs, excludeCM, metallb.Namespace, cm)
		Expect(err).NotTo(HaveOccurred())

		ginkgo.By("restarting speaker pods so excludenode configmap is read")
		restartSpeakerPods(cs)

		for _, c := range FRRContainers {
			err := c.UpdateBGPConfigFile(frrconfig.Empty)
			Expect(err).NotTo(HaveOccurred(),
				fmt.Sprintf("cleaning frr config at %s failed", c.Name))
		}

		testNamespace, err = k8s.CreateTestNamespace(cs, "bgp")
		Expect(err).NotTo(HaveOccurred())

		allNodes, err = cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, svc = setupBGPService(cs, testNamespace, ipfamily.IPv4, []string{v4PoolAddresses}, FRRContainers, func(svc *corev1.Service) {
			testservice.TrafficPolicyCluster(svc)
		})
	})

	ginkgo.AfterEach(func() {
		if ginkgo.CurrentSpecReport().Failed() {
			dumpBGPInfo(ReportPath, ginkgo.CurrentSpecReport().LeafNodeText, cs, testNamespace)
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}

		testservice.Delete(cs, svc)
		err := k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		err = k8s.RemoveConfigmap(cs, excludeCM, metallb.Namespace)
		Expect(err).NotTo(HaveOccurred())
		restartSpeakerPods(cs)
	})

	ginkgo.It("service should be announced only from expected nodes", func() {
		ginkgo.By("checking that all nodes advertise prefix")
		checkServiceOnlyOnNodes(svc, allNodes.Items, ipfamily.IPv4)

		ginkgo.By("adding annotation to the node")
		k8s.AddAnnotationToNode(allNodes.Items[0].Name, ann[0], ann[1], cs)
		defer func() {
			k8s.RemoveAnnotationFromNode(allNodes.Items[0].Name, ann[0], cs)
		}()

		ginkgo.By("checking that specific nodes advertise prefix after annotation")
		checkServiceOnlyOnNodes(svc, allNodes.Items[1:], ipfamily.IPv4)
		checkServiceNotOnNodes(svc, allNodes.Items[:1], ipfamily.IPv4)

		ginkgo.By("removing the annotation of the node")
		k8s.RemoveAnnotationFromNode(allNodes.Items[0].Name, ann[0], cs)

		ginkgo.By("checking that all nodes advertise prefix as before annotation")
		checkServiceOnlyOnNodes(svc, allNodes.Items, ipfamily.IPv4)
	})
})

func restartSpeakerPods(cs clientset.Interface) {
	defer ginkgo.GinkgoHelper()
	err := metallb.RestartSpeakerPods(cs)
	Expect(err).NotTo(HaveOccurred())
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
	}, 3*time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

	err = FRRProvider.Update()
	Expect(err).NotTo(HaveOccurred())
}
