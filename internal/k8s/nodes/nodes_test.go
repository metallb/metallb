// SPDX-License-Identifier:Apache-2.0
package nodes

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsNodeExcludedFromBalancers2(t *testing.T) {
	tests := []struct {
		name    string
		node    *corev1.Node
		pattern *corev1.Node
		want    bool
	}{
		{
			name: "exclude when labels match",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			pattern: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: true,
		},
		{
			name: "exclude when annotation match",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			pattern: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: true,
		},
		// {
		// 	name:    "exclude when condition match",
		// 	node:    &corev1.Node{},
		// 	pattern: &corev1.Node{},
		// 	want:    true,
		// },
		{
			name: "exclude not when annotation missmatch",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{},
			},
			pattern: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesExcludePattern(tt.node, tt.pattern)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%q: unexpected advertisement state (-want +got)\n%s", tt.name, diff)
			}
		})
	}
}
