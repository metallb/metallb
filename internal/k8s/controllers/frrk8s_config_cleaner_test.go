// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"context"
	"time"

	"github.com/go-kit/log"
	frrv1beta1 "github.com/metallb/frr-k8s/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	frrk8s "go.universe.tf/metallb/internal/bgp/frrk8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	cleanerDaemonSetName = "speaker"
	gateNamespace        = "frr-gate-test"
	selectedNode         = "cleaner-selected"
	unselectedNode       = "cleaner-unselected"
	gateNode             = "gate-node"
)

var _ = Describe("FRR-K8s configuration cleaner", func() {
	BeforeEach(func() {
		ensureNamespace(gateNamespace)
		ensureSpeakerDaemonSet(map[string]string{"bgp": "enabled"})
	})

	It("deletes configurations for nodes the speaker DaemonSet no longer selects", func() {
		ensureNode(selectedNode, map[string]string{"bgp": "enabled"})
		ensureNode(unselectedNode, map[string]string{})
		DeferCleanup(func() {
			deleteNode(selectedNode)
			deleteNode(unselectedNode)
		})

		selected := newFRRConfiguration(testNamespace, selectedNode)
		unselected := newFRRConfiguration(testNamespace, unselectedNode)
		foreign := &frrv1beta1.FRRConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "user-bgp", Namespace: testNamespace},
			Spec: frrv1beta1.FRRConfigurationSpec{
				NodeSelector: metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelHostname: unselectedNode}},
				BGP:          frrv1beta1.BGPConfig{Routers: []frrv1beta1.Router{{ASN: 64512}}},
			},
		}
		Expect(k8sClient.Create(ctx, selected)).To(Succeed())
		Expect(k8sClient.Create(ctx, unselected)).To(Succeed())
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() {
			deleteFRRConfiguration(testNamespace, selected.Name)
			deleteFRRConfiguration(testNamespace, unselected.Name)
			deleteFRRConfiguration(testNamespace, foreign.Name)
		})

		Eventually(frrConfigExists(testNamespace, unselected.Name), 5*time.Second, 200*time.Millisecond).Should(BeFalse())
		Consistently(frrConfigExists(testNamespace, selected.Name), 2*time.Second, 200*time.Millisecond).Should(BeTrue())
		Consistently(frrConfigExists(testNamespace, foreign.Name), 2*time.Second, 200*time.Millisecond).Should(BeTrue())

		setNodeLabels(selectedNode, map[string]string{corev1.LabelHostname: selectedNode})
		Eventually(frrConfigExists(testNamespace, selected.Name), 5*time.Second, 200*time.Millisecond).Should(BeFalse())
		Expect(frrConfigExists(testNamespace, foreign.Name)()).To(BeTrue())

		ensureNode(unselectedNode, map[string]string{"bgp": "enabled"})
		recreated := newFRRConfiguration(testNamespace, unselectedNode)
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		Eventually(frrConfigExists(testNamespace, recreated.Name), 5*time.Second, 200*time.Millisecond).Should(BeTrue())
		deleteNode(unselectedNode)
		Eventually(frrConfigExists(testNamespace, recreated.Name), 5*time.Second, 200*time.Millisecond).Should(BeFalse())
	})

	It("withdraws and restores this speaker's configuration as the node selection changes", func() {
		ensureNode(gateNode, map[string]string{"bgp": "enabled"})
		DeferCleanup(func() {
			deleteNode(gateNode)
		})

		cfg := newFRRConfiguration(gateNamespace, gateNode)
		reconciler := &FRRK8sReconciler{
			Client:             k8sClient,
			Logger:             log.NewNopLogger(),
			NodeName:           gateNode,
			FRRK8sNamespace:    gateNamespace,
			DaemonSetName:      cleanerDaemonSetName,
			DaemonSetNamespace: speakerNamespace,
		}
		reconciler.desiredConfiguration = cfg.DeepCopy()

		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(frrConfigExists(gateNamespace, cfg.Name)()).To(BeTrue())

		setNodeLabels(gateNode, map[string]string{corev1.LabelHostname: gateNode})
		_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(frrConfigExists(gateNamespace, cfg.Name)()).To(BeFalse())

		setNodeLabels(gateNode, map[string]string{"bgp": "enabled", corev1.LabelHostname: gateNode})
		_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(frrConfigExists(gateNamespace, cfg.Name)()).To(BeTrue())

		deleteNode(gateNode)
		_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: cfg.Name, Namespace: cfg.Namespace}})
		Expect(err).ToNot(HaveOccurred())
		Expect(frrConfigExists(gateNamespace, cfg.Name)()).To(BeFalse())
	})
})

func ensureNamespace(name string) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Create(ctx, ns)
	if apierrors.IsAlreadyExists(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

func ensureSpeakerDaemonSet(nodeSelector map[string]string) {
	key := types.NamespacedName{Name: cleanerDaemonSetName, Namespace: speakerNamespace}
	current := &appsv1.DaemonSet{}
	err := k8sClient.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		ds := speakerDaemonSet(nodeSelector)
		Expect(k8sClient.Create(ctx, ds)).To(Succeed())
		return
	}
	Expect(err).ToNot(HaveOccurred())
	original := current.Spec.Template.Spec.NodeSelector
	current.Spec.Template.Spec.NodeSelector = nodeSelector
	Expect(k8sClient.Update(ctx, current)).To(Succeed())
	DeferCleanup(func() {
		latest := &appsv1.DaemonSet{}
		if err := k8sClient.Get(ctx, key, latest); err != nil {
			return
		}
		latest.Spec.Template.Spec.NodeSelector = original
		_ = k8sClient.Update(ctx, latest)
	})
}

func speakerDaemonSet(nodeSelector map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: cleanerDaemonSetName, Namespace: speakerNamespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "speaker-test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "speaker-test"}},
				Spec: corev1.PodSpec{
					NodeSelector: nodeSelector,
					HostNetwork:  true,
					Containers:   []corev1.Container{{Name: "speaker", Image: "speaker"}},
					Tolerations: []corev1.Toleration{{
						Key:      "node-role.kubernetes.io/control-plane",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					}},
				},
			},
		},
	}
}

func ensureNode(name string, labels map[string]string) {
	if labels == nil {
		labels = map[string]string{}
	}
	labels[corev1.LabelHostname] = name
	node := &corev1.Node{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)
	if apierrors.IsNotFound(err) {
		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
		Expect(k8sClient.Create(ctx, node)).To(Succeed())
		return
	}
	Expect(err).ToNot(HaveOccurred())
	node.Labels = labels
	Expect(k8sClient.Update(ctx, node)).To(Succeed())
}

func setNodeLabels(name string, labels map[string]string) {
	Eventually(func() error {
		node := &corev1.Node{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, node); err != nil {
			return err
		}
		node.Labels = labels
		return k8sClient.Update(ctx, node)
	}, 5*time.Second, 200*time.Millisecond).Should(Succeed())
}

func deleteNode(name string) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	err := k8sClient.Delete(ctx, node)
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred())
}

func newFRRConfiguration(namespace, nodeName string) *frrv1beta1.FRRConfiguration {
	return &frrv1beta1.FRRConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frrk8s.ConfigName(nodeName),
			Namespace: namespace,
		},
		Spec: frrv1beta1.FRRConfigurationSpec{
			NodeSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{corev1.LabelHostname: nodeName},
			},
			BGP: frrv1beta1.BGPConfig{Routers: []frrv1beta1.Router{{ASN: 64512}}},
		},
	}
}

func frrConfigExists(namespace, name string) func() bool {
	return func() bool {
		cfg := &frrv1beta1.FRRConfiguration{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cfg)
		if apierrors.IsNotFound(err) {
			return false
		}
		Expect(err).ToNot(HaveOccurred())
		return true
	}
}

func deleteFRRConfiguration(namespace, name string) {
	cfg := &frrv1beta1.FRRConfiguration{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	err := k8sClient.Delete(context.Background(), cfg)
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred())
}
