// SPDX-License-Identifier:Apache-2.0

package config

import (
	"testing"
	"time"

	"go.universe.tf/metallb/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBGPPeerTimerBounds(t *testing.T) {
	duration := func(value time.Duration) *metav1.Duration {
		return &metav1.Duration{Duration: value}
	}
	tests := []struct {
		name      string
		hold      *metav1.Duration
		keepalive *metav1.Duration
		wantError bool
	}{
		{name: "maximum hold time", hold: duration(65535 * time.Second)},
		{name: "minimum hold time", hold: duration(3 * time.Second)},
		{name: "hold time below minimum", hold: duration(time.Second), wantError: true},
		{name: "maximum keepalive with explicit hold time", hold: duration(65535 * time.Second), keepalive: duration(65535 * time.Second)},
		{name: "hold time exceeds OPEN field", hold: duration(65536 * time.Second), wantError: true},
		{name: "derived maximum hold time", keepalive: duration(21845 * time.Second)},
		{name: "derived hold time exceeds OPEN field", keepalive: duration(21846 * time.Second), wantError: true},
		{name: "negative hold time below one second", hold: duration(-time.Millisecond), wantError: true},
		{name: "negative keepalive time", hold: duration(180 * time.Second), keepalive: duration(-time.Second), wantError: true},
		{name: "zero timers", hold: duration(0), keepalive: duration(0)},
		{name: "default timers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := For(ClusterResources{Peers: []v1beta2.BGPPeer{{
				Spec: v1beta2.BGPPeerSpec{
					MyASN: 42, ASN: 43, Address: "192.0.2.1",
					HoldTime: tt.hold, KeepaliveTime: tt.keepalive,
				},
			}}}, DontValidate, ForOptions{})
			if (err != nil) != tt.wantError {
				t.Fatalf("For() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
