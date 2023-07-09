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
	if len(ips) == 0 {
		return Unknown, fmt.Errorf("IPFamilyForAddresses: no ips specified %d %q", len(ips), ips)
	}
	out := Unknown
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed.To4() != nil {
			if out == Unknown {
				out = IPv4
			} else if out == IPv6 {
				return DualStack, nil
			}
		} else if parsed != nil {
			if out == Unknown {
				out = IPv6
			} else if out == IPv4 {
				return DualStack, nil
			}
		} else {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: Invalid address %q", ip)
		}
	}
	return out, nil
}

func ForAddrs(ips []netip.Addr) (Family, error) {
	if len(ips) == 0 {
		return Unknown, fmt.Errorf("IPFamilyForAddrs: no ips specified %d %q", len(ips), ips)
	}
	out := Unknown
	for _, ip := range ips {
		if ip.Is4() || ip.Is4In6() {
			if out == Unknown {
				out = IPv4
			} else if out == IPv6 {
				return DualStack, nil
			}
		} else if ip.Is6() {
			if out == Unknown {
				out = IPv6
			} else if out == IPv4 {
				return DualStack, nil
			}
		} else {
			return Unknown, fmt.Errorf("IPFamilyForAddrs: Invalid address %q", ip)
		}
	}
	return out, nil
}

// ForAddressesIPs returns the address family from a given list of addresses IPs.
func ForAddressesIPs(ips []net.IP) (Family, error) {
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
func ForAddress(ip net.IP) Family {
	if ip.To4() == nil {
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
