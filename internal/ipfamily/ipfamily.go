// SPDX-License-Identifier:Apache-2.0

package ipfamily // import "go.universe.tf/metallb/internal/ipfamily"

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
	IPv4              Family = "ipv4"
	IPv6              Family = "ipv6"
	RequiredDualStack Family = "require-dual"
	PreferDualStack   Family = "prefer-dual"
	Unknown           Family = "unknown"
)

// determineIPFamily returns the address family for a given ip string.
func determineIPFamily(ipString string) Family {
	ip := net.ParseIP(ipString)
	if ip == nil {
		return Unknown
	}
  return ForAddress(ip)
}

// ForAddresses returns the address family given list of addresses strings and the
// ipFamilyPolicy of the service
func ForAddresses(ips []string, requestedFamily v1.IPFamilyPolicy) (Family, error) {
	switch len(ips) {
	case 1:
		ipType := determineIPFamily(ips[0])
		if ipType == requestedFamily {
			return ipType, nil
		}
		return Unknown, fmt.Errorf("IPFamilyForAddresses: Invalid address %q", ips)
	case 2:
		ipType1 := determineIPFamily(ips[0])
		ipType2 := determineIPFamily(ips[1])
		if ipType1 == Unknown || ipType2 == Unknown {
			return Unknown, fmt.Errorf("IPFamilyForAddresses: Invalid address %q", ips)
		}
		if ipType1 == requestedFamily || ipType2 == requestedFamily {
			return requestedFamily, nil
		}
		if ipType1 != ipType2 {
			return requestedFamily, nil
		}
		if requestedFamily == PreferDualStack {
			return requestedFamily, nil
		}
		return Unknown, fmt.Errorf("IPFamilyForAddresses: Invalid address %q", ips)
	default:
		return Unknown, fmt.Errorf("IPFamilyForAddresses: invalid ips length %d %q", len(ips), ips)
	}
}

// ForAddressesIPs returns the address family from a given list of addresses ips
// and the ipFamilyPolicy of the service
func ForAddressesIPs(ips []net.IP, requestedFamily Family) (Family, error) {
	ipsStrings := []string{}

	for _, ip := range ips {
		ipsStrings = append(ipsStrings, ip.String())
	}
	return ForAddresses(ipsStrings, requestedFamily)
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
	requestedFamily := svc.Spec.IPFamilyPolicy
	if requestedFamily == nil {
		requestedFamily = IPv4
	}
	if len(svc.Spec.ClusterIPs) > 0 {
		return ForAddresses(svc.Spec.ClusterIPs, requestedFamily)
	}
	// fallback to clusterip if clusterips are not set
	addresses := []string{svc.Spec.ClusterIP}
	return ForAddresses(addresses, requestedFamily)
}
