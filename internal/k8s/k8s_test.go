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
