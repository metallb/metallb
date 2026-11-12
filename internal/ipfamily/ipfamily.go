// SPDX-License-Identifier:Apache-2.0

package ipfamily // import "go.universe.tf/metallb/internal/ipfamily"

import (
	"fmt"
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
	if len(ips) != 1 && len(ips) != 2 {
		return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid ips length %d %q", len(ips), ips)
	}
	addresses := make([]netip.Addr, len(ips))
	for i, ip := range ips {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid address %q: %w", ip, err)
		}
		addresses[i] = addr
	}
	return ForAddressesIPs(addresses)
}

// ForAddressesIPs returns the address family from a list of typed addresses.
func ForAddressesIPs(ips []netip.Addr) (Family, error) {
	if len(ips) != 1 && len(ips) != 2 {
		return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid ips length %d %q", len(ips), ips)
	}
	for _, ip := range ips {
		if !ip.IsValid() {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid address %q", ip)
		}
	}
	if len(ips) == 1 {
		return ForAddress(ips[0]), nil
	}
	if ips[0].Unmap().Is4() == ips[1].Unmap().Is4() {
		return Unknown, fmt.Errorf("IPFamilyForAddresses: same address family %q", ips)
	}
	return DualStack, nil
}

// ForCIDR returns the address family from a given CIDR.
func ForCIDR(cidr netip.Prefix) Family {
	if !cidr.Addr().Unmap().Is4() {
		return IPv6
	}
	return IPv4
}

// ForAddress returns the address family for a given address.
func ForAddress(ip netip.Addr) Family {
	if !ip.Unmap().Is4() {
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
