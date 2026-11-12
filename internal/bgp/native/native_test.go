// SPDX-License-Identifier:Apache-2.0

package native

import (
	"net/netip"
	"testing"
)

func TestNormalizeIPv4Transport(t *testing.T) {
	tests := []struct {
		name        string
		peerAddress netip.Addr
		sourceAddr  netip.Addr
		wantPeer    netip.Addr
		wantSource  netip.Addr
		wantErr     string
	}{
		{
			name:        "peer with automatically selected source",
			peerAddress: netip.MustParseAddr("192.0.2.1"),
			wantPeer:    netip.MustParseAddr("192.0.2.1"),
		},
		{
			name:        "peer and configured source",
			peerAddress: netip.MustParseAddr("192.0.2.1"),
			sourceAddr:  netip.MustParseAddr("192.0.2.2"),
			wantPeer:    netip.MustParseAddr("192.0.2.1"),
			wantSource:  netip.MustParseAddr("192.0.2.2"),
		},
		{
			name:        "peer and unspecified IPv4 source",
			peerAddress: netip.MustParseAddr("192.0.2.1"),
			sourceAddr:  netip.IPv4Unspecified(),
			wantPeer:    netip.MustParseAddr("192.0.2.1"),
			wantSource:  netip.IPv4Unspecified(),
		},
		{
			name:        "IPv4-mapped peer",
			peerAddress: netip.MustParseAddr("::ffff:192.0.2.1"),
			wantPeer:    netip.MustParseAddr("192.0.2.1"),
		},
		{
			name:        "IPv4-mapped source",
			peerAddress: netip.MustParseAddr("192.0.2.1"),
			sourceAddr:  netip.MustParseAddr("::ffff:192.0.2.2"),
			wantPeer:    netip.MustParseAddr("192.0.2.1"),
			wantSource:  netip.MustParseAddr("192.0.2.2"),
		},
		{
			name:        "IPv6 peer",
			peerAddress: netip.MustParseAddr("2001:db8::1"),
			wantErr:     `IPv6 peer address "2001:db8::1" is not supported by native BGP`,
		},
		{
			name:        "IPv6 source address",
			peerAddress: netip.MustParseAddr("192.0.2.1"),
			sourceAddr:  netip.MustParseAddr("2001:db8::2"),
			wantErr:     `IPv6 source address "2001:db8::2" is not supported by native BGP`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			peerAddr, sourceAddr, err := normalizeIPv4Transport(test.peerAddress, test.sourceAddr)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("normalizeIPv4Transport(%q, %q) unexpected error: %v", test.peerAddress, test.sourceAddr, err)
				}
				if peerAddr != test.wantPeer || sourceAddr != test.wantSource {
					t.Fatalf("normalizeIPv4Transport(%q, %q) = (%q, %q), want (%q, %q)", test.peerAddress, test.sourceAddr, peerAddr, sourceAddr, test.wantPeer, test.wantSource)
				}
				return
			}
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("normalizeIPv4Transport(%q, %q) error = %v, want %q", test.peerAddress, test.sourceAddr, err, test.wantErr)
			}
		})
	}
}
