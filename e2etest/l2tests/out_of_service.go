// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// outOfServiceTaint is what an administrator applies to a uncontrolled shut down node so that kubernetes
// force deletes the pods running on it. MetalLB must stop announcing from such a node.
var outOfServiceTaint = corev1.Taint{
	Key:    corev1.TaintNodeOutOfService,
	Value:  "nodeshutdown",
	Effect: corev1.TaintEffectNoExecute,
}

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	testNamespace := ""

	emptyL2Adv := metallbv1beta1.L2Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty",
		},
	}

	ginkgo.AfterEach(func() {
		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())

		if ginkgo.CurrentSpecReport().Failed() {
			k8s.DumpInfo(Reporter, ginkgo.CurrentSpecReport().LeafNodeText)
		}
		err = k8s.DeleteNamespace(cs, testNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.BeforeEach(func() {
		ginkgo.By("Clearing any previous configuration")

		err := ConfigUpdater.Clean()
		Expect(err).NotTo(HaveOccurred())
		cs = k8sclient.New()
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2oos")
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("out-of-service taint", func() {
		ginkgo.BeforeEach(func() {
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
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2Adv},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("should not be announced from a node tainted with out-of-service=nodeshutdown:NoExecute", func() {
			keepSpeaker := os.Getenv("METALLB_E2E_OUT_OF_SERVICE_KEEP_SPEAKER") == "true"
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("requires at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("getting the advertising node")
			var nodeToTaint *corev1.Node
			Eventually(func() error {
				var err error
				nodeToTaint, err = nodeForService(svc, allNodes.Items)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())
			if keepSpeaker {
				pod, err := metallb.SpeakerPodInNode(cs, nodeToTaint.Name)
				Expect(err).NotTo(HaveOccurred())
				bindAddrConfigured, labelsConfigured := false, false
				for _, container := range pod.Spec.Containers {
					for _, env := range container.Env {
						configured := env.Value != "" || env.ValueFrom != nil
						switch env.Name {
						case "METALLB_ML_BIND_ADDR":
							bindAddrConfigured = configured
						case "METALLB_ML_LABELS":
							labelsConfigured = configured
						}
					}
				}
				Expect(bindAddrConfigured && labelsConfigured).To(BeFalse(), "memberlist must be disabled for this test")
			}

			added, err := k8s.AddTaintToNode(cs, nodeToTaint.Name, outOfServiceTaint)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if !added {
					return
				}
				err := k8s.RemoveTaintFromNode(cs, nodeToTaint.Name, outOfServiceTaint)
				Expect(err).NotTo(HaveOccurred())
			}()

			if keepSpeaker {
				ginkgo.By("validating the speaker of the tainted node remains ready")
				Eventually(func() (bool, error) {
					return metallb.HasSpeakerInNode(cs, nodeToTaint.Name)
				}, 2*time.Minute, time.Second).Should(BeTrue())
			} else {
				ginkgo.By("validating the speaker of the tainted node gets evicted")
				Eventually(func() (bool, error) {
					return metallb.HasSpeakerInNode(cs, nodeToTaint.Name)
				}, 2*time.Minute, time.Second).Should(BeFalse())
			}

			ginkgo.By("validating the service is announced from a different node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, 2*time.Minute, time.Second).ShouldNot(Equal(nodeToTaint.Name))

			ginkgo.By("validating the service is still reachable")
			Eventually(func() error {
				return service.ValidateL2(svc)
			}, time.Minute, time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("removing the out-of-service taint")
			if added {
				err = k8s.RemoveTaintFromNode(cs, nodeToTaint.Name, outOfServiceTaint)
				Expect(err).NotTo(HaveOccurred())
			}

			ginkgo.By("validating the speaker is ready on the node")
			Eventually(func() (bool, error) {
				return metallb.HasSpeakerInNode(cs, nodeToTaint.Name)
			}, 2*time.Minute, time.Second).Should(BeTrue())

			ginkgo.By("validating the service is announced back again from the previous node")
			Eventually(func() string {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return ""
				}
				return node.Name
			}, 2*time.Minute, time.Second).Should(Equal(nodeToTaint.Name))
		})

		ginkgo.DescribeTable("should keep announcing from a node tainted with", func(taint corev1.Taint) {
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("requires at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("getting the advertising node")
			var nodeToTaint *corev1.Node
			Eventually(func() error {
				var err error
				nodeToTaint, err = nodeForService(svc, allNodes.Items)
				return err
			}, 30*time.Second, 1*time.Second).ShouldNot(HaveOccurred())

			added, err := k8s.AddTaintToNode(cs, nodeToTaint.Name, taint)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if !added {
					return
				}
				err := k8s.RemoveTaintFromNode(cs, nodeToTaint.Name, taint)
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("validating the service keeps being announced from the same node")
			Consistently(func() error {
				node, err := nodeForService(svc, allNodes.Items)
				if err != nil {
					return err
				}
				if node.Name != nodeToTaint.Name {
					return fmt.Errorf("service announced from %s, want %s", node.Name, nodeToTaint.Name)
				}
				return nil
			}, 30*time.Second, 5*time.Second).ShouldNot(HaveOccurred())
		},
			// Both entries use a non evicting effect, so the speaker stays up and only MetalLB's
			// handling of the taint decides whether the announcement moves.
			ginkgo.Entry("out-of-service and a value other than nodeshutdown", corev1.Taint{
				Key:    corev1.TaintNodeOutOfService,
				Value:  "notnodeshutdown",
				Effect: corev1.TaintEffectNoSchedule,
			}),
			ginkgo.Entry("out-of-service=nodeshutdown and an effect other than NoExecute", corev1.Taint{
				Key:    corev1.TaintNodeOutOfService,
				Value:  "nodeshutdown",
				Effect: corev1.TaintEffectNoSchedule,
			}),
		)
	})
})
