package ndp

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp/internal/ndptest"
)

func Test_chooseAddr(t *testing.T) {
	// Assumed zone for all tests.
	const zone = "eth0"

	var (
		ip4 = net.IPv4(192, 168, 1, 1).To4()
		ip6 = ndptest.MustIPv6("2001:db8::1000")

		gua = ndptest.MustIPv6("2001:db8::1")
		ula = ndptest.MustIPv6("fc00::1")
		lla = ndptest.MustIPv6("fe80::1")
	)

	addrs := []net.Addr{
		// Ignore non-IP addresses.
		&net.TCPAddr{IP: gua},

		&net.IPNet{IP: ip4},
		&net.IPNet{IP: ula},
		&net.IPNet{IP: lla},

		// The second GUA IPv6 address should only be found when
		// Addr specifies it explicitly.
		&net.IPNet{IP: gua},
		&net.IPNet{IP: ip6},
	}

	tests := []struct {
		name  string
		addrs []net.Addr
		addr  Addr
		ip    net.IP
		ok    bool
	}{
		{
			name: "empty",
		},
		{
			name: "IPv4 Addr",
			addr: Addr(ip4.String()),
		},
		{
			name: "no IPv6 addresses",
			addrs: []net.Addr{
				&net.IPNet{
					IP: ip4,
				},
			},
			addr: LinkLocal,
		},
		{
			name: "ok, unspecified",
			ip:   net.IPv6unspecified,
			addr: Unspecified,
			ok:   true,
		},
		{
			name:  "ok, GUA",
			addrs: addrs,
			ip:    gua,
			addr:  Global,
			ok:    true,
		},
		{
			name:  "ok, ULA",
			addrs: addrs,
			ip:    ula,
			addr:  UniqueLocal,
			ok:    true,
		},
		{
			name:  "ok, LLA",
			addrs: addrs,
			ip:    lla,
			addr:  LinkLocal,
			ok:    true,
		},
		{
			name:  "ok, arbitrary",
			addrs: addrs,
			ip:    ip6,
			addr:  Addr(ip6.String()),
			ok:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipa, err := chooseAddr(tt.addrs, zone, tt.addr)

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			ttipa := &net.IPAddr{
				IP:   tt.ip,
				Zone: zone,
			}

			if diff := cmp.Diff(ttipa, ipa); diff != "" {
				t.Fatalf("unexpected IPv6 address (-want +got):\n%s", diff)
			}
		})
	}
}
