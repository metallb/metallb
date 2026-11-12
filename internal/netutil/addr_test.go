// SPDX-License-Identifier:Apache-2.0

package netutil

import (
	"net"
	"net/netip"
	"testing"
)

func TestAddrFromNetAddr(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want netip.Addr
		ok   bool
	}{
		{
			name: "IPNet IPv4",
			addr: &net.IPNet{IP: net.ParseIP("192.0.2.1")},
			want: netip.MustParseAddr("192.0.2.1"),
			ok:   true,
		},
		{
			name: "IPAddr IPv6 with zone",
			addr: &net.IPAddr{IP: net.ParseIP("fe80::1"), Zone: "eth0"},
			want: netip.MustParseAddr("fe80::1%eth0"),
			ok:   true,
		},
		{
			name: "TCPAddr IPv4-mapped IPv6",
			addr: &net.TCPAddr{IP: net.ParseIP("::ffff:192.0.2.2")},
			want: netip.MustParseAddr("192.0.2.2"),
			ok:   true,
		},
		{
			name: "UDPAddr IPv6",
			addr: &net.UDPAddr{IP: net.ParseIP("2001:db8::1")},
			want: netip.MustParseAddr("2001:db8::1"),
			ok:   true,
		},
		{
			name: "invalid IP",
			addr: &net.IPAddr{},
		},
		{
			name: "malformed IP",
			addr: &net.IPAddr{IP: net.IP{192, 0, 2}},
		},
		{
			name: "nil net.Addr",
			addr: nil,
		},
		{
			name: "nil IPNet",
			addr: (*net.IPNet)(nil),
		},
		{
			name: "nil IPAddr",
			addr: (*net.IPAddr)(nil),
		},
		{
			name: "nil TCPAddr",
			addr: (*net.TCPAddr)(nil),
		},
		{
			name: "nil UDPAddr",
			addr: (*net.UDPAddr)(nil),
		},
		{
			name: "unsupported address type",
			addr: &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := AddrFromNetAddr(tt.addr)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("AddrFromNetAddr(%v) = (%v, %t), want (%v, %t)", tt.addr, got, ok, tt.want, tt.ok)
			}
		})
	}
}
