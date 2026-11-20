// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.universe.tf/e2etest/pkg/config"
	"go.universe.tf/e2etest/pkg/k8s"
	"go.universe.tf/e2etest/pkg/k8sclient"
	"go.universe.tf/e2etest/pkg/metallb"
	"go.universe.tf/e2etest/pkg/service"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// outOfServiceTaint is what an administrator applies to a uncontrolled shut down node so that kubernetes
// force deletes the pods running on it. MetalLB must stop announcing from such a node.
var outOfServiceTaint = corev1.Taint{
	Key:    corev1.TaintNodeOutOfService,
	Value:  "nodeshutdown",
	Effect: corev1.TaintEffectNoExecute,
}

var outOfServiceSpeakerToleration = corev1.Toleration{
	Key:      corev1.TaintNodeOutOfService,
	Operator: corev1.TolerationOpEqual,
	Value:    "nodeshutdown",
	Effect:   corev1.TaintEffectNoExecute,
}

type memberlistEnvVar struct {
	index int
	env   corev1.EnvVar
}

type speakerDaemonSetState struct {
	name                        string
	memberlistEnv               []memberlistEnvVar
	addedOutOfServiceToleration bool
}

func snapshotSpeakerDaemonSets(cs clientset.Interface) ([]speakerDaemonSetState, error) {
	speakers, err := metallb.SpeakerPods(cs)
	if err != nil {
		return nil, err
	}

	names := map[string]struct{}{}
	for _, pod := range speakers {
		found := false
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "DaemonSet" {
				names[owner.Name] = struct{}{}
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("speaker pod %s has no DaemonSet owner", pod.Name)
		}
	}

	daemonSetNames := make([]string, 0, len(names))
	for name := range names {
		daemonSetNames = append(daemonSetNames, name)
	}
	sort.Strings(daemonSetNames)

	states := make([]speakerDaemonSetState, 0, len(daemonSetNames))
	for _, name := range daemonSetNames {
		ds, err := cs.AppsV1().DaemonSets(metallb.Namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		container, err := speakerContainer(ds)
		if err != nil {
			return nil, err
		}

		state := speakerDaemonSetState{name: name}
		for i, env := range container.Env {
			if isMemberlistEnv(env.Name) {
				state.memberlistEnv = append(state.memberlistEnv, memberlistEnvVar{index: i, env: env})
			}
		}
		state.addedOutOfServiceToleration = !hasOutOfServiceSpeakerToleration(ds.Spec.Template.Spec.Tolerations)
		states = append(states, state)
	}
	return states, nil
}

func setMemberlistDisabled(cs clientset.Interface, states []speakerDaemonSetState, disabled bool) error {
	for _, state := range states {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			ds, err := cs.AppsV1().DaemonSets(metallb.Namespace).Get(context.Background(), state.name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			container, err := speakerContainer(ds)
			if err != nil {
				return err
			}

			if disabled {
				container.Env = removeMemberlistEnv(container.Env)
				if !hasOutOfServiceSpeakerToleration(ds.Spec.Template.Spec.Tolerations) {
					// Keep the speaker pod on a node carrying the out-of-service taint.
					ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, outOfServiceSpeakerToleration)
				}
			} else {
				container.Env = restoreMemberlistEnv(container.Env, state.memberlistEnv)
				if state.addedOutOfServiceToleration {
					ds.Spec.Template.Spec.Tolerations = removeOutOfServiceSpeakerToleration(ds.Spec.Template.Spec.Tolerations)
				}
			}
			_, err = cs.AppsV1().DaemonSets(metallb.Namespace).Update(context.Background(), ds, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			return fmt.Errorf("updating speaker DaemonSet %s: %w", state.name, err)
		}
	}
	return waitForSpeakerDaemonSets(cs, states)
}

func waitForSpeakerDaemonSets(cs clientset.Interface, states []speakerDaemonSetState) error {
	return wait.PollUntilContextTimeout(context.Background(), time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		for _, state := range states {
			ds, err := cs.AppsV1().DaemonSets(metallb.Namespace).Get(ctx, state.name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ds.Status.ObservedGeneration < ds.Generation ||
				ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled ||
				ds.Status.NumberAvailable < ds.Status.DesiredNumberScheduled {
				return false, nil
			}
		}
		return true, nil
	})
}

func speakerContainer(ds *appsv1.DaemonSet) (*corev1.Container, error) {
	for i := range ds.Spec.Template.Spec.Containers {
		if ds.Spec.Template.Spec.Containers[i].Name == "speaker" {
			return &ds.Spec.Template.Spec.Containers[i], nil
		}
	}
	return nil, fmt.Errorf("speaker DaemonSet %s has no speaker container", ds.Name)
}

func isMemberlistEnv(name string) bool {
	return name == "METALLB_ML_BIND_ADDR" || name == "METALLB_ML_LABELS"
}

func removeMemberlistEnv(env []corev1.EnvVar) []corev1.EnvVar {
	remaining := env[:0]
	for _, variable := range env {
		if !isMemberlistEnv(variable.Name) {
			remaining = append(remaining, variable)
		}
	}
	return remaining
}

func restoreMemberlistEnv(env []corev1.EnvVar, original []memberlistEnvVar) []corev1.EnvVar {
	env = removeMemberlistEnv(env)
	for _, variable := range original {
		index := variable.index
		if index > len(env) {
			index = len(env)
		}
		env = append(env, corev1.EnvVar{})
		copy(env[index+1:], env[index:])
		env[index] = variable.env
	}
	return env
}

func hasOutOfServiceSpeakerToleration(tolerations []corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if toleration.Key == outOfServiceSpeakerToleration.Key &&
			toleration.Operator == outOfServiceSpeakerToleration.Operator &&
			toleration.Value == outOfServiceSpeakerToleration.Value &&
			toleration.Effect == outOfServiceSpeakerToleration.Effect {
			return true
		}
	}
	return false
}

func removeOutOfServiceSpeakerToleration(tolerations []corev1.Toleration) []corev1.Toleration {
	remaining := tolerations[:0]
	removed := false
	for _, toleration := range tolerations {
		if !removed &&
			toleration.Key == outOfServiceSpeakerToleration.Key &&
			toleration.Operator == outOfServiceSpeakerToleration.Operator &&
			toleration.Value == outOfServiceSpeakerToleration.Value &&
			toleration.Effect == outOfServiceSpeakerToleration.Effect {
			removed = true
			continue
		}
		remaining = append(remaining, toleration)
	}
	return remaining
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

		ginkgo.It("should not be announced from a node tainted with out-of-service=nodeshutdown NoExecute", func() {
			allNodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			if len(allNodes.Items) < 2 {
				ginkgo.Skip("requires at least two nodes")
			}

			states, err := snapshotSpeakerDaemonSets(cs)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				err := setMemberlistDisabled(cs, states, false)
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("disabling memberlist and waiting for speakers to restart")
			err = setMemberlistDisabled(cs, states, true)
			Expect(err).NotTo(HaveOccurred())

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

			added, err := k8s.AddTaintToNode(cs, nodeToTaint.Name, outOfServiceTaint)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if !added {
					return
				}
				err := k8s.RemoveTaintFromNode(cs, nodeToTaint.Name, outOfServiceTaint)
				Expect(err).NotTo(HaveOccurred())
			}()

			ginkgo.By("validating the speaker of the tainted node remains ready")
			Eventually(func() (bool, error) {
				return metallb.HasSpeakerInNode(cs, nodeToTaint.Name)
			}, 2*time.Minute, time.Second).Should(BeTrue())

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
