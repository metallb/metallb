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
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	"go.universe.tf/e2etest/pkg/status"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

var _ = ginkgo.Describe("L2", func() {
	var cs clientset.Interface
	testNamespace := ""

	emptyL2 := metallbv1beta1.L2Advertisement{
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
		testNamespace, err = k8s.CreateTestNamespace(cs, "l2spk")
		Expect(err).NotTo(HaveOccurred())
	})

	ginkgo.Context("speaker failover", func() {
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
				L2Advs: []metallbv1beta1.L2Advertisement{emptyL2},
			}

			err := ConfigUpdater.Update(resources)
			Expect(err).NotTo(HaveOccurred())
		})

		ginkgo.It("should move L2 advertisement when the announcing node stops running a ready speaker", func() {
			nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(nodes.Items) < 2 {
				ginkgo.Skip("requires at least two nodes")
			}

			svc, _ := service.CreateWithBackend(cs, testNamespace, "external-local-lb", service.TrafficPolicyCluster)
			defer func() {
				err := cs.CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}()

			var announcing string
			Eventually(func() error {
				s, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if err != nil {
					return err
				}
				announcing = s.Status.Node
				return nil
			}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred())

			originalName := announcing

			ginkgo.By(fmt.Sprintf("adding a NoSchedule taint the speaker does not tolerate on node %s, then deleting its speaker pod", originalName))
			err = addSpeakerFailoverNoScheduleTaint(cs, originalName)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				err := removeSpeakerFailoverNoScheduleTaint(cs, originalName)
				Expect(err).NotTo(HaveOccurred())
			}()

			speakerPod, err := metallb.SpeakerPodInNode(cs, originalName)
			Expect(err).NotTo(HaveOccurred())
			err = cs.CoreV1().Pods(metallb.Namespace).Delete(context.TODO(), speakerPod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By("waiting until the node has no ready speaker (DaemonSet cannot replace the pod while the taint is present)")
			Eventually(func() error {
				has, err := metallb.HasSpeakerInNode(cs, originalName)
				if err != nil {
					return err
				}
				if has {
					return fmt.Errorf("speaker still running and ready on %s", originalName)
				}
				return nil
			}, 2*time.Minute, time.Second).Should(Succeed())

			ginkgo.By("expecting a different announcing node and working connectivity")
			Eventually(func() error {
				s, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if err != nil {
					return err
				}
				if s.Status.Node == originalName {
					return fmt.Errorf("announcement still on %s", originalName)
				}
				return nil
			}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred())

			ginkgo.By("removing the speaker-failover taint so the speaker can run on the original node again")
			err = removeSpeakerFailoverNoScheduleTaint(cs, originalName)
			Expect(err).NotTo(HaveOccurred())

			ginkgo.By(fmt.Sprintf("expecting L2 announcement to return to the original node %s", originalName))
			Eventually(func() error {
				s, err := status.L2ForService(ConfigUpdater.Client(), svc)
				if err != nil {
					return err
				}
				if s.Status.Node != originalName {
					return fmt.Errorf("announcement on %s, want %s", s.Status.Node, originalName)
				}
				return nil
			}, 2*time.Minute, time.Second).ShouldNot(HaveOccurred())
		})
	})
})

// Taint used only in this test: the MetalLB speaker DaemonSet does not tolerate it, so the
// speaker cannot be rescheduled onto the node after we delete its pod.
const (
	speakerFailoverTaintKey   = "metallb.e2e/speaker-failover-block"
	speakerFailoverTaintValue = "true"
)

func addSpeakerFailoverNoScheduleTaint(cs clientset.Interface, nodeName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := cs.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, t := range node.Spec.Taints {
			if t.Key == speakerFailoverTaintKey && t.Effect == corev1.TaintEffectNoSchedule {
				return nil
			}
		}
		taints := append([]corev1.Taint(nil), node.Spec.Taints...)
		taints = append(taints, corev1.Taint{
			Key:    speakerFailoverTaintKey,
			Value:  speakerFailoverTaintValue,
			Effect: corev1.TaintEffectNoSchedule,
		})
		node.Spec.Taints = taints
		_, err = cs.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		return err
	})
}

func removeSpeakerFailoverNoScheduleTaint(cs clientset.Interface, nodeName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := cs.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		var newTaints []corev1.Taint
		found := false
		for _, t := range node.Spec.Taints {
			if t.Key == speakerFailoverTaintKey && t.Effect == corev1.TaintEffectNoSchedule {
				found = true
				continue
			}
			newTaints = append(newTaints, t)
		}
		if !found {
			return nil
		}
		node.Spec.Taints = newTaints
		_, err = cs.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		return err
	})
}
