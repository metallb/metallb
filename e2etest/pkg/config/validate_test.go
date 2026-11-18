// SPDX-License-Identifier:Apache-2.0

package config

import (
	"math"
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func TestGetIPFromRangeByIndex(t *testing.T) {
	tests := []struct {
		name    string
		rangeIP string
		index   int
		want    string
		wantErr bool
	}{
		{name: "first address in range", rangeIP: "192.0.2.1-192.0.2.4", index: 0, want: "192.0.2.1"},
		{name: "last address in range", rangeIP: "192.0.2.1-192.0.2.4", index: 3, want: "192.0.2.4"},
		{name: "IPv6 address", rangeIP: "2001:db8::/126", index: 3, want: "2001:db8::3"},
		{name: "mapped IPv4 range", rangeIP: "::ffff:192.0.2.0/126", index: 3, want: "192.0.2.3"},
		{name: "negative index", rangeIP: "192.0.2.0/24", index: -1, wantErr: true},
		{name: "index outside range", rangeIP: "192.0.2.0/30", index: 4, wantErr: true},
		{name: "invalid range", rangeIP: "not-a-range", index: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetIPFromRangeByIndex(tt.rangeIP, tt.index)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetIPFromRangeByIndex(%q, %d) error = %v, wantErr %t", tt.rangeIP, tt.index, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("GetIPFromRangeByIndex(%q, %d) = %q, want %q", tt.rangeIP, tt.index, got, tt.want)
			}
		})
	}
}

func TestPoolCount(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		avoid     bool
		wantTotal int64
		wantV4    int64
		wantV6    int64
		wantErr   bool
	}{
		{name: "IPv4 CIDR", addresses: []string{"192.0.2.0/30"}, wantTotal: 4, wantV4: 4},
		{name: "avoid buggy IPv4 addresses", addresses: []string{"192.0.2.0/24"}, avoid: true, wantTotal: 254, wantV4: 254},
		{name: "avoid duplicate buggy endpoint", addresses: []string{"192.0.2.255/32"}, avoid: true},
		{name: "IPv6 CIDR", addresses: []string{"2001:db8::/126"}, wantTotal: 4, wantV6: 4},
		{name: "mapped IPv4 CIDR", addresses: []string{"::ffff:192.0.2.0/120"}, wantTotal: 256, wantV4: 256},
		{name: "mixed families", addresses: []string{"192.0.2.0/30", "2001:db8::/126"}, wantTotal: 8, wantV4: 4, wantV6: 4},
		{name: "large IPv6 range saturates", addresses: []string{"2001:db8::/64"}, wantTotal: math.MaxInt64, wantV6: math.MaxInt64},
		{name: "invalid range", addresses: []string{"invalid"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := metallbv1beta1.IPAddressPool{
				Spec: metallbv1beta1.IPAddressPoolSpec{
					Addresses:     tt.addresses,
					AvoidBuggyIPs: tt.avoid,
				},
			}
			gotTotal, gotV4, gotV6, err := PoolCount(pool)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PoolCount() error = %v, wantErr %t", err, tt.wantErr)
			}
			if err == nil && (gotTotal != tt.wantTotal || gotV4 != tt.wantV4 || gotV6 != tt.wantV6) {
				t.Errorf("PoolCount() = (%d, %d, %d), want (%d, %d, %d)", gotTotal, gotV4, gotV6, tt.wantTotal, tt.wantV4, tt.wantV6)
			}
		})
	}
}
