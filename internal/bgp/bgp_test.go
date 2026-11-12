// SPDX-License-Identifier:Apache-2.0

package bgp

import (
	"net/netip"
	"testing"
)

func TestAdvertisementEqualPrefixes(t *testing.T) {
	for _, tt := range []struct {
		name, a, b string
		want       bool
	}{
		{"same IPv4 prefix", "192.0.2.0/24", "192.0.2.0/24", true},
		{"same IPv6 prefix", "2001:db8::/64", "2001:db8::/64", true},
		{"different address", "192.0.2.0/24", "192.0.3.0/24", false},
		{"different length", "192.0.2.0/24", "192.0.2.0/25", false},
		{"host bits remain significant", "192.0.2.1/24", "192.0.2.0/24", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &Advertisement{Prefix: netip.MustParsePrefix(tt.a)}
			b := &Advertisement{Prefix: netip.MustParsePrefix(tt.b)}
			if got := a.Equal(b); got != tt.want {
				t.Fatalf("Equal() = %t, want %t", got, tt.want)
			}
		})
	}
}
