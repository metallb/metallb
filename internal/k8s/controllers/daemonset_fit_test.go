// SPDX-License-Identifier:Apache-2.0

package controllers

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeContinuesRunningDaemonPod(t *testing.T) {
	node := func(name string, labels map[string]string, taints ...corev1.Taint) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
			Spec:       corev1.NodeSpec{Taints: taints},
		}
	}
	ds := func(spec corev1.PodSpec) *appsv1.DaemonSet {
		if len(spec.Containers) == 0 {
			spec.Containers = []corev1.Container{{Name: "speaker", Image: "speaker"}}
		}
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "speaker", Namespace: "metallb-system"},
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{Spec: spec},
			},
		}
	}

	selected := node("worker-a", map[string]string{"bgp": "enabled", "kubernetes.io/hostname": "worker-a"})
	unselected := node("worker-b", map[string]string{"kubernetes.io/hostname": "worker-b"})
	speaker := ds(corev1.PodSpec{
		NodeSelector: map[string]string{"bgp": "enabled"},
		HostNetwork:  true,
		Tolerations: []corev1.Toleration{{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
	})

	if !nodeContinuesRunningDaemonPod(selected, speaker) {
		t.Fatal("node matching the selector should keep its speaker")
	}
	if nodeContinuesRunningDaemonPod(unselected, speaker) {
		t.Fatal("node missing the selector label should not keep its speaker")
	}

	controlPlane := node("cp", map[string]string{"bgp": "enabled"}, corev1.Taint{
		Key:    "node-role.kubernetes.io/control-plane",
		Effect: corev1.TaintEffectNoSchedule,
	})
	if !nodeContinuesRunningDaemonPod(controlPlane, speaker) {
		t.Fatal("tolerated NoSchedule taint should not evict a running speaker")
	}

	notReady := node("worker-a", map[string]string{"bgp": "enabled"}, corev1.Taint{
		Key:    corev1.TaintNodeNotReady,
		Effect: corev1.TaintEffectNoExecute,
	})
	if !nodeContinuesRunningDaemonPod(notReady, speaker) {
		t.Fatal("default DaemonSet toleration should keep the pod on a NotReady node")
	}

	maintenance := node("worker-a", map[string]string{"bgp": "enabled"}, corev1.Taint{
		Key:    "maintenance",
		Effect: corev1.TaintEffectNoExecute,
	})
	if nodeContinuesRunningDaemonPod(maintenance, speaker) {
		t.Fatal("untolerated NoExecute taint should evict the speaker")
	}

	noScheduleOnly := node("worker-a", map[string]string{"bgp": "enabled"}, corev1.Taint{
		Key:    "dedicated",
		Effect: corev1.TaintEffectNoSchedule,
	})
	if !nodeContinuesRunningDaemonPod(noScheduleOnly, speaker) {
		t.Fatal("untolerated NoSchedule taint should not evict a speaker that is already running")
	}

	affinity := ds(corev1.PodSpec{
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "rack",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"frontend", "edge"},
						}},
					}, {
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "kubernetes.io/hostname",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"worker-b"},
						}},
					}},
				},
			},
		},
	})
	if !nodeContinuesRunningDaemonPod(node("worker-b", map[string]string{"kubernetes.io/hostname": "worker-b"}), affinity) {
		t.Fatal("second affinity term should match worker-b")
	}
	if nodeContinuesRunningDaemonPod(node("worker-c", map[string]string{"kubernetes.io/hostname": "worker-c", "rack": "backend"}), affinity) {
		t.Fatal("node matching neither affinity term should not keep a speaker")
	}

	pinned := ds(corev1.PodSpec{NodeName: "worker-a"})
	if nodeContinuesRunningDaemonPod(unselected, pinned) {
		t.Fatal("pod template pinned to another node should not match")
	}
	if !nodeContinuesRunningDaemonPod(selected, pinned) {
		t.Fatal("pod template pinned to this node should match")
	}
}
