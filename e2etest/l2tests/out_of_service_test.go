// SPDX-License-Identifier:Apache-2.0

package l2tests

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestRestoreMemberlistEnvPreservesOtherVariables(t *testing.T) {
	original := []corev1.EnvVar{
		{Name: "FIRST", Value: "first"},
		{
			Name: "METALLB_ML_BIND_ADDR",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
		{Name: "OTHER", Value: "other"},
		{Name: "METALLB_ML_LABELS", Value: "app=metallb,component=speaker"},
	}
	snapshot := []memberlistEnvVar{
		{index: 1, env: original[1]},
		{index: 3, env: original[3]},
	}

	disabled := append([]corev1.EnvVar(nil), original...)
	disabled = append(removeMemberlistEnv(disabled), corev1.EnvVar{Name: "ADDED", Value: "added"})
	got := restoreMemberlistEnv(disabled, snapshot)
	want := append(original, corev1.EnvVar{Name: "ADDED", Value: "added"})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("restored environment mismatch (-want +got):\n%s", diff)
	}
}

func TestRemoveOutOfServiceSpeakerTolerationRemovesOnlyOneExactEntry(t *testing.T) {
	otherToleration := corev1.Toleration{
		Key:      corev1.TaintNodeOutOfService,
		Operator: corev1.TolerationOpEqual,
		Value:    "other-value",
		Effect:   corev1.TaintEffectNoExecute,
	}
	original := []corev1.Toleration{
		otherToleration,
		outOfServiceSpeakerToleration,
		outOfServiceSpeakerToleration,
	}

	got := removeOutOfServiceSpeakerToleration(original)
	want := []corev1.Toleration{otherToleration, outOfServiceSpeakerToleration}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("remaining tolerations mismatch (-want +got):\n%s", diff)
	}
}
