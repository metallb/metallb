// SPDX-License-Identifier:Apache-2.0

package iprange

import (
	"fmt"
	"net/netip"
	"strings"

	"go4.org/netipx"
)

func Parse(cidr string) ([]netip.Prefix, error) {
	if !strings.Contains(cidr, "-") {
		n, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", cidr)
		}
		n, err = normalizePrefix(n)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		return []netip.Prefix{n.Masked()}, nil
	}

	fs := strings.SplitN(cidr, "-", 2)
	if len(fs) != 2 {
		return nil, fmt.Errorf("invalid IP range %q", cidr)
	}
	start, err := netip.ParseAddr(strings.TrimSpace(fs[0]))
	if err != nil || start.Zone() != "" {
		return nil, fmt.Errorf("invalid IP range %q: invalid start IP %q", cidr, fs[0])
	}
	end, err := netip.ParseAddr(strings.TrimSpace(fs[1]))
	if err != nil || end.Zone() != "" {
		return nil, fmt.Errorf("invalid IP range %q: invalid end IP %q", cidr, fs[1])
	}

	start = start.Unmap()
	end = end.Unmap()
	if start.Is4() != end.Is4() {
		return nil, fmt.Errorf("invalid IP range %q: start and end IPs must be the same family", cidr)
	}
	if start.Compare(end) > 0 {
		return nil, fmt.Errorf("invalid IP range %q: start IP %q is after the end IP %q", cidr, start, end)
	}

	return netipx.IPRangeFrom(start, end).Prefixes(), nil
}

// normalizePrefix converts prefixes wholly within IPv4-mapped IPv6 space to IPv4.
func normalizePrefix(prefix netip.Prefix) (netip.Prefix, error) {
	addr := prefix.Addr()
	if !addr.Is4In6() {
		return prefix, nil
	}
	if prefix.Bits() < 96 {
		return prefix, nil
	}
	return netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96), nil
}
