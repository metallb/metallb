// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.universe.tf/metallb/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPreferredNodeScores(t *testing.T) {
	nodes := []corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "edge-a", Labels: map[string]string{"role": "edge", "zone": "primary"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "edge-b", Labels: map[string]string{"role": "edge", "zone": "secondary"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", Labels: map[string]string{"role": "worker", "zone": "primary"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-b", Labels: map[string]string{"role": "worker"}}},
	}

	tests := []struct {
		desc      string
		nodes     []corev1.Node
		eligible  map[string]bool
		preferred []v1beta1.PreferredNodeSelector
		want      map[string]int64
		wantErr   bool
	}{
		{
			desc:     "no preferences returns nil",
			eligible: map[string]bool{"edge-a": true, "worker-a": true},
		},
		{
			desc:     "single selector scores matching eligible nodes",
			eligible: map[string]bool{"edge-a": true, "edge-b": true, "worker-a": true, "worker-b": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 100, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"role": "edge"}}},
			},
			want: map[string]int64{"edge-a": 100, "edge-b": 100},
		},
		{
			desc:     "cumulative weights sum",
			eligible: map[string]bool{"edge-a": true, "edge-b": true, "worker-a": true, "worker-b": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 60, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"zone": "primary"}}},
				{Weight: 50, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"role": "edge"}}},
			},
			want: map[string]int64{"edge-a": 110, "edge-b": 50, "worker-a": 60},
		},
		{
			desc:     "preferences do not score ineligible nodes",
			eligible: map[string]bool{"edge-a": true, "edge-b": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 100, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"zone": "primary"}}},
			},
			want: map[string]int64{"edge-a": 100},
		},
		{
			desc:     "empty preference matches every eligible node",
			eligible: map[string]bool{"edge-a": true, "worker-b": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 10, Preference: metav1.LabelSelector{}},
			},
			want: map[string]int64{"edge-a": 10, "worker-b": 10},
		},
		{
			desc:     "no matches returns nil",
			eligible: map[string]bool{"edge-a": true, "edge-b": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 50, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"role": "gpu"}}},
			},
		},
		{
			desc: "selectors with distinct weights apply independently",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "edge-a", Labels: map[string]string{"role": "edge"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "worker-a", Labels: map[string]string{"role": "worker"}}},
			},
			eligible: map[string]bool{"edge-a": true, "worker-a": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 40, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"role": "edge"}}},
				{Weight: 70, Preference: metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}},
			},
			want: map[string]int64{"edge-a": 40, "worker-a": 70},
		},
		{
			desc: "invalid label selector operator",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: map[string]string{"a": "b"}}},
			},
			eligible: map[string]bool{"n": true},
			preferred: []v1beta1.PreferredNodeSelector{
				{Weight: 10, Preference: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "x",
					Operator: "BadOperator",
				}}}},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ns := tc.nodes
			if ns == nil {
				ns = nodes
			}
			got, err := preferredNodeScores(ns, tc.eligible, tc.preferred)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("preferredNodeScores returned error: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("preferredNodeScores mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
