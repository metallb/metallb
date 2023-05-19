// SPDX-License-Identifier:Apache-2.0

package ipfamily // import "go.universe.tf/e2etests/pkg/ipfamily"

import (
	"fmt"
	"net"

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
		ip := net.ParseIP(ips[0])
		if ip.To4() != nil {
			return IPv4, nil
		}
		return IPv6, nil
	case 2:
		ip1 := net.ParseIP(ips[0])
		ip2 := net.ParseIP(ips[1])
		if ip1 == nil || ip2 == nil {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: Invalid address %q", ips)
		}
		if (ip1.To4() == nil) == (ip2.To4() == nil) {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: same address family %q", ips)
		}
		return DualStack, nil
	default:
		return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid ips length %d %q", len(ips), ips)
	}
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
