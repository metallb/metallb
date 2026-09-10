// SPDX-License-Identifier:Apache-2.0

package routes

import (
	"net"
	"testing"
)

func TestParseIPToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Reviewer example: multiple hextets after ::
		{name: "compressed with trailing hextets", input: " 2001:db8:1:2::5:6 dev eth0", want: "2001:db8:1:2::5:6"},
		// Original dualstack failure case
		{name: "kind cluster ULA", input: " fc00:f853:ccd:e793::1 dev eth0", want: "fc00:f853:ccd:e793::1"},
		// Middle compression with multiple trailing groups
		{name: "middle compression", input: " 2001:db8::8a2e:370:7334 dev eth0", want: "2001:db8::8a2e:370:7334"},
		// Full 8-group notation
		{name: "full notation", input: " 2001:0db8:85a3:0000:0000:8a2e:0370:7334 dev eth0", want: "2001:db8:85a3::8a2e:370:7334"},
		// Loopback and unspecified
		{name: "loopback", input: " ::1 dev lo", want: "::1"},
		{name: "unspecified", input: " :: dev lo", want: "::"},
		// Link-local with zone identifier
		{name: "link-local with zone", input: " fe80::1%eth0 dev eth0", want: "fe80::1"},
		// IPv4-mapped IPv6
		{name: "ipv4-mapped", input: " ::ffff:192.0.2.1 dev eth0", want: "192.0.2.1"},
		// Plain IPv4
		{name: "ipv4", input: " 192.168.1.1 dev eth0", want: "192.168.1.1"},
		// Route nexthop line fragment
		{name: "nexthop line", input: " fc00:f853:ccd:e793::1 dev eth0 weight 1", want: "fc00:f853:ccd:e793::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIPToken(tt.input)
			if got == nil {
				t.Fatalf("ParseIPToken(%q) = nil, want %s", tt.input, tt.want)
			}
			wantIP := net.ParseIP(tt.want)
			if !got.Equal(wantIP) {
				t.Fatalf("ParseIPToken(%q) = %s, want %s", tt.input, got, wantIP)
			}
		})
	}
}

func TestParseIPTokenInvalid(t *testing.T) {
	invalid := []string{
		"",
		"   ",
		"not-an-ip dev eth0",
		"999.999.999.999 dev eth0",
		"gggg::1 dev eth0",
	}
	for _, input := range invalid {
		if got := ParseIPToken(input); got != nil {
			t.Errorf("ParseIPToken(%q) = %s, want nil", input, got)
		}
	}
}
