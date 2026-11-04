// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"testing"
)

func TestDefaultHealthProbePort(t *testing.T) {
	if DefaultHealthProbePort <= 0 {
		t.Errorf("DefaultHealthProbePort must be positive, got %d", DefaultHealthProbePort)
	}
}

func TestMetricsBindAddress(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{name: "disabled", port: 0, want: "0"},
		{name: "enabled", port: 9120, want: "0.0.0.0:9120"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := metricsBindAddress(test.port); got != test.want {
				t.Fatalf("metricsBindAddress(%d) = %q, want %q", test.port, got, test.want)
			}
		})
	}
}
