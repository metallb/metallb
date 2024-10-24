// SPDX-License-Identifier:Apache-2.0

package ipfamily

import (
	"net"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestIPFamilyForAddresses(t *testing.T) {
	tests := []struct {
		desc    string
		ips     []string
		policy  v1.IPFamilyPolicy
		family  Family
		wantErr bool
	}{
		{
			desc:   "ipv4 address",
			ips:    []string{"1.1.1.1"},
			policy: v1.IPFamilyPolicySingleStack,
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ips:    []string{"100::1"},
			policy: v1.IPFamilyPolicySingleStack,
			family: IPv6,
		},
		{
			desc:    "one invalid address",
			ips:     []string{"!.1.1.1"},
			policy:  v1.IPFamilyPolicySingleStack,
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:   "require dualstack with ipv4 and ipv6 addresses",
			ips:    []string{"1.2.3.4", "100::1"},
			policy: v1.IPFamilyPolicyRequireDualStack,
			family: RequireDualStack,
		},
		{
			desc:   "prefer dualstack with ipv4 and ipv6 addresses",
			ips:    []string{"1.2.3.4", "100::1"},
			policy: v1.IPFamilyPolicyPreferDualStack,
			family: PreferDualStack,
		},
		{
			desc:   "prefer dualstack with both ipv4",
			ips:    []string{"1.2.3.4", "5.6.7.8"},
			policy: v1.IPFamilyPolicyPreferDualStack,
			family: PreferDualStack,
		},
		{
			desc:   "prefer dualstack with both ipv6",
			ips:    []string{"100::1", "100::2"},
			policy: v1.IPFamilyPolicyPreferDualStack,
			family: PreferDualStack,
		},
		{
			desc:    "prefer dualstack with empty address",
			ips:     []string{"", ""},
			policy:  v1.IPFamilyPolicyPreferDualStack,
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "require dualstack with one invalid address",
			ips:     []string{"!.1.1.1", "100::1"},
			policy:  v1.IPFamilyPolicyRequireDualStack,
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "more than 2 addresses",
			policy:  v1.IPFamilyPolicyRequireDualStack,
			ips:     []string{"1.1.1.1", "100::1", "2.2.2.2"},
			family:  Unknown,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family, err := ForAddresses(test.ips, test.policy)
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
		ips     []net.IP
		policy  v1.IPFamilyPolicy
		family  Family
		wantErr bool
	}{
		{
			desc:   "ipv4 address",
			ips:    []net.IP{net.ParseIP("1.2.4.0")},
			policy: v1.IPFamilyPolicySingleStack,
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ips:    []net.IP{net.ParseIP("100::1")},
			policy: v1.IPFamilyPolicySingleStack,
			family: IPv6,
		},
		{
			desc:   "ipv4 and ipv6 addresse",
			ips:    []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("100::1")},
			policy: v1.IPFamilyPolicyRequireDualStack,
			family: RequireDualStack,
		},
		{
			desc:   "dual stack with same address family",
			ips:    []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("5.6.7.8")},
			policy: v1.IPFamilyPolicyPreferDualStack,
			family: PreferDualStack,
		},
		{
			desc:    "dual stack with empty address",
			ips:     []net.IP{net.ParseIP(""), net.ParseIP("")},
			policy:  v1.IPFamilyPolicyRequireDualStack,
			family:  Unknown,
			wantErr: true,
		},
		{
			desc:    "more than 2 addresses",
			ips:     []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("100::1"), net.ParseIP("2.2.2.2")},
			policy:  v1.IPFamilyPolicyRequireDualStack,
			family:  Unknown,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			family, err := ForAddressesIPs(test.ips, test.policy)
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
		cidr   *net.IPNet
		family Family
	}{
		{
			desc:   "ipv4 cidr",
			cidr:   ipnet("1.2.3.4/30"),
			family: IPv4,
		},
		{
			desc:   "ipv6 cidr",
			cidr:   ipnet("100::/96"),
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
		ip     net.IP
		family Family
	}{
		{
			desc:   "ipv4 address",
			ip:     net.ParseIP("1.2.3.4"),
			family: IPv4,
		},
		{
			desc:   "ipv6 address",
			ip:     net.ParseIP("100::"),
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

func ipnet(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}
