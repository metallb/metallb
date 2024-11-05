// SPDX-License-Identifier:Apache-2.0

package epslices

import (
	"testing"

	v1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/ptr"
)

func TestIsConditionReadyOrServing(t *testing.T) {
	tests := []struct {
		name                   string
		conditions             v1.EndpointConditions
		ignoreServingEndpoints bool
		want                   bool
	}{
		{
			name: "Is Ready",
			conditions: v1.EndpointConditions{
				Ready: ptr.To(true),
			},
			ignoreServingEndpoints: false,
			want:                   true,
		},
		{
			name: "Is Serving",
			conditions: v1.EndpointConditions{
				Serving: ptr.To(true),
			},
			ignoreServingEndpoints: false,
			want:                   true,
		},
		{
			name: "Is Ready but not serving",
			conditions: v1.EndpointConditions{
				Ready:   ptr.To(true),
				Serving: ptr.To(false),
			},
			ignoreServingEndpoints: false,
			want:                   true,
		},
		{
			name:                   "Ready and Serving not set",
			conditions:             v1.EndpointConditions{},
			ignoreServingEndpoints: false,
			want:                   true,
		},
		{
			name: "Ready not set and not serving",
			conditions: v1.EndpointConditions{
				Serving: ptr.To(false),
			},
			ignoreServingEndpoints: false,
			want:                   true,
		},
		{
			name: "Not Ready",
			conditions: v1.EndpointConditions{
				Ready: ptr.To(false),
			},
			ignoreServingEndpoints: false,
			want:                   false,
		},
		{
			name: "Not Ready and is Serving but ignoreServingEndpoints is true",
			conditions: v1.EndpointConditions{
				Ready:   ptr.To(false),
				Serving: ptr.To(true),
			},
			ignoreServingEndpoints: true,
			want:                   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EndpointCanServe(tt.conditions, tt.ignoreServingEndpoints); got != tt.want {
				t.Errorf("EndpointCanServe() = %v, want %v", got, tt.want)
			}
		})
	}
}
