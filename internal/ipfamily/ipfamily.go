// SPDX-License-Identifier:Apache-2.0

package ipfamily // import "go.universe.tf/metallb/internal/ipfamily"

import (
	"fmt"
	"net"
	"net/netip"

	v1 "k8s.io/api/core/v1"
)

// IP family helps identifying single stack IPv4/IPv6 vs Dual-stack ["IPv4", "IPv6"] or ["IPv6", "Ipv4"].
type Family string

func (f Family) String() string {
	return string(f)
}

const (
	IPv4      Family = "ipv4"
	IPv6      Family = "ipv6"
	DualStack Family = "dual"
	Unknown   Family = "unknown"
)

// ForAddresses returns the address family given list of addresses strings.
func ForAddresses(ips []string) (Family, error) {
	switch len(ips) {
	case 1:
		ip, err := netip.ParseAddr(ips[0])
		if err != nil {
			return Unknown, err
		}
		if ip.Is4() {
			return IPv4, nil
		} else {
			return IPv6, nil
		}
	case 2:
		ip1, err := netip.ParseAddr(ips[0])
		if err != nil {
			return Unknown, err
		}
		ip2, err := netip.ParseAddr(ips[1])
		if err != nil {
			return Unknown, err
		}
		if (ip1.Is4()) == (ip2.Is4()) {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: same address family %q", ips)
		}
		return DualStack, nil
	default:
		return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid ips length %d %q", len(ips), ips)
	}
}

// ForAddressesIPs returns the address family from a given list of addresses IPs.
func ForAddressesIPs(ips []netip.Addr) (Family, error) {
	ipsStrings := []string{}

	for _, ip := range ips {
		ipsStrings = append(ipsStrings, ip.String())
	}
	return ForAddresses(ipsStrings)
}

// ForCIDR returns the address family from a given CIDR.
func ForCIDR(cidr *net.IPNet) Family {
	if cidr.IP.To4() == nil {
		return IPv6
	}
	return IPv4
}

// ForAddress returns the address family for a given address.
func ForAddress(ip netip.Addr) Family {
	if ip.Is6() {
		return IPv6
	}
	return IPv4
}

// ForService returns the address family of a given service.
func ForService(svc *v1.Service) (Family, error) {
	if len(svc.Spec.ClusterIPs) > 0 {
		return ForAddresses(svc.Spec.ClusterIPs)
	}
	// fallback to clusterip if clusterips are not set
	addresses := []string{svc.Spec.ClusterIP}
	return ForAddresses(addresses)
}
