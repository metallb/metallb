// SPDX-License-Identifier:Apache-2.0

package ipfamily

import (
	"net/netip"
	"testing"
)

func TestIPFamilyForAddresses(t *testing.T) {
	tests := []struct {
		desc    string
		ips     []string
		family  Family
		wantErr bool
	}{
		{
			desc:   "ipv4 address",
			ips:    []string{"1.1.1.1"},
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ips:    []string{"100::1"},
			family: IPv6,
		},
		{
			desc:    "one invalid address",
			ips:     []string{"!.1.1.1"},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:   "ipv4 and ipv6 addresse",
			ips:    []string{"1.2.3.4", "100::1"},
			family: DualStack,
		},
		{
			desc:    "dual stack with same address family",
			ips:     []string{"1.2.3.4", "5.6.7.8"},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "dual stack with empty address",
			ips:     []string{"", ""},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "dual stack with one invalid address",
			ips:     []string{"!.1.1.1", "100::1"},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "more than 2 addresses",
			ips:     []string{"1.1.1.1", "100::1", "2.2.2.2"},
			family:  Unknown,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family, err := ForAddresses(test.ips)
			if test.wantErr && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
			if family != test.family {
				t.Fatalf("Incorrect IPFamily returned %s expected %s", family, test.family)
			}
		})
	}
}

func TestIPFamilyForAddressesIPs(t *testing.T) {
	tests := []struct {
		desc    string
		ips     []netip.Addr
		family  Family
		wantErr bool
	}{
		{
			desc:   "ipv4 address",
			ips:    []netip.Addr{netip.MustParseAddr("1.2.4.0")},
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ips:    []netip.Addr{netip.MustParseAddr("100::1")},
			family: IPv6,
		},
		{
			desc:   "ipv4 and ipv6 addresse",
			ips:    []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("100::1")},
			family: DualStack,
		},
		{
			desc:    "dual stack with same address family",
			ips:     []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("5.6.7.8")},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "dual stack with empty address",
			ips:     []netip.Addr{{}, {}},
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "more than 2 addresses",
			ips:     []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("100::1"), netip.MustParseAddr("2.2.2.2")},
			family:  Unknown,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family, err := ForAddressesIPs(test.ips)
			if test.wantErr && err == nil {
				t.Fatalf("Expected error for %s", test.desc)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("Not expected error %s for %s", err, test.desc)
			}
			if family != test.family {
				t.Fatalf("Incorrect IPFamily returned %s expected %s", family, test.family)
			}
		})
	}
}

func TestIPFamilyForCIDR(t *testing.T) {
	tests := []struct {
		desc   string
		cidr   netip.Prefix
		family Family
	}{
		{
			desc:   "ipv4 cidr",
			cidr:   netip.MustParsePrefix("1.2.3.4/30"),
			family: IPv4,
		},
		{
			desc:   "ipv6 cidr",
			cidr:   netip.MustParsePrefix("100::/96"),
			family: IPv6,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family := ForCIDR(test.cidr)
			if family != test.family {
				t.Fatalf("Incorrect IPFamily returned %s expected %s", family, test.family)
			}
		})
	}
}

func TestIPFamilyForAddress(t *testing.T) {
	tests := []struct {
		desc   string
		ip     netip.Addr
		family Family
	}{
		{
			desc:   "ipv4 address",
			ip:     netip.MustParseAddr("1.2.3.4"),
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ip:     netip.MustParseAddr("100::"),
			family: IPv6,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family := ForAddress(test.ip)
			if family != test.family {
				t.Fatalf("Incorrect IPFamily returned %s expected %s", family, test.family)
			}
		})
	}
}
