// SPDX-License-Identifier:Apache-2.0

package nodes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsNetworkUnavailable(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "no conditions",
			node: &corev1.Node{},
			want: false,
		},
		{
			name: "network unavailable true",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "network unavailable false",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
		{
			name: "network unavailable unknown",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionUnknown},
					},
				},
			},
			want: false,
		},
		{
			name: "other condition true only",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
					},
				},
			},
			want: false,
		},
		{
			name: "network unavailable true among other conditions",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue},
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNetworkUnavailable(tt.node); got != tt.want {
				t.Errorf("IsNetworkUnavailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNodeExcludedFromBalancers(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "no labels",
			node: &corev1.Node{},
			want: false,
		},
		{
			name: "unrelated labels",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
			},
			want: false,
		},
		{
			name: "exclude label with empty value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{corev1.LabelNodeExcludeBalancers: ""},
				},
			},
			want: true,
		},
		{
			name: "exclude label with value",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1.LabelNodeExcludeBalancers: "true",
						"foo":                            "bar",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNodeExcludedFromBalancers(tt.node); got != tt.want {
				t.Errorf("IsNodeExcludedFromBalancers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNodeOutOfService(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "nil taints",
			node: &corev1.Node{},
			want: false,
		},
		{
			name: "empty taints",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{Taints: []corev1.Taint{}},
			},
			want: false,
		},
		{
			name: "unrelated taint",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoExecute},
					},
				},
			},
			want: false,
		},
		{
			name: "out of service taint",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeOutOfService, Value: "nodeshutdown", Effect: corev1.TaintEffectNoExecute},
					},
				},
			},
			want: true,
		},
		{
			name: "out of service taint with a different value",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeOutOfService, Value: "notnodeshutdown", Effect: corev1.TaintEffectNoExecute},
					},
				},
			},
			want: false,
		},
		{
			name: "out of service taint with no value",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeOutOfService, Effect: corev1.TaintEffectNoExecute},
					},
				},
			},
			want: false,
		},
		{
			name: "out of service taint with NoSchedule effect",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeOutOfService, Value: "nodeshutdown", Effect: corev1.TaintEffectNoSchedule},
					},
				},
			},
			want: false,
		},
		{
			name: "out of service taint with PreferNoSchedule effect",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeOutOfService, Value: "nodeshutdown", Effect: corev1.TaintEffectPreferNoSchedule},
					},
				},
			},
			want: false,
		},
		{
			name: "out of service taint among others",
			node: &corev1.Node{
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
						{Key: corev1.TaintNodeOutOfService, Value: "nodeshutdown", Effect: corev1.TaintEffectNoExecute},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNodeOutOfService(tt.node); got != tt.want {
				t.Errorf("IsNodeOutOfService() = %v, want %v", got, tt.want)
			}
		})
	}
}
