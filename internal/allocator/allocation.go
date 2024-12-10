// SPDX-License-Identifier:Apache-2.0

package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"fmt"
	"net"

	"go.universe.tf/metallb/internal/ipfamily"
	v1 "k8s.io/api/core/v1"
)

// Allocation represents a mapping of IP families (IPv4, IPv6, or both) to their respective IP addresses.
// This type is used to select and organize IPs based on a service's IP family policy.
// Depending on the family policy, the map may contain:
// - Just an IPv4 address
// - Just an IPv6 address
// - Both IPv4 and IPv6 addresses for dual-stack support.
//
// Key:
// - ipfamily.IPv4: Corresponds to the IPv4 address allocated for the service.
// - ipfamily.IPv6: Corresponds to the IPv6 address allocated for the service.
type Allocation struct {
	PoolName string
	IPV4     net.IP
	IPV6     net.IP
}

func (a *Allocation) getIPForFamily(family ipfamily.Family) net.IP {
	switch family {
	case ipfamily.IPv4:
		return a.IPV4
	case ipfamily.IPv6:
		return a.IPV6
	default:
		return nil
	}
}

func (a *Allocation) setIPForFamily(family ipfamily.Family, ip net.IP) {
	if ip == nil {
		return
	}
	switch family {
	case ipfamily.IPv4:
		a.IPV4 = ip
	case ipfamily.IPv6:
		a.IPV6 = ip
	}
}

// selectIPsForFamilyPolicy returns a slice of IPs from the ipPool, which are suitable for the
// given service ipfamily and IPFamilyPolicy.
func (a *Allocation) selectIPsForFamilyAndPolicy(
	serviceIPFamily ipfamily.Family,
	serviceIPFamilyPolicy v1.IPFamilyPolicy,
) ([]net.IP, error) {
	ipv4 := a.getIPForFamily(ipfamily.IPv4)
	ipv6 := a.getIPForFamily(ipfamily.IPv6)

	switch serviceIPFamilyPolicy {
	case v1.IPFamilyPolicySingleStack:
		if ip := a.getIPForFamily(serviceIPFamily); ip != nil {
			return []net.IP{ip}, nil
		}
	case v1.IPFamilyPolicyRequireDualStack:
		if ipv4 != nil && ipv6 != nil {
			return []net.IP{ipv4, ipv6}, nil
		}
	case v1.IPFamilyPolicyPreferDualStack:
		if ipv4 != nil && ipv6 != nil {
			return []net.IP{ipv4, ipv6}, nil
		}
		if ipv4 != nil {
			return []net.IP{ipv4}, nil
		}
		if ipv6 != nil {
			return []net.IP{ipv6}, nil
		}
	}
	return nil, fmt.Errorf("no available IPs in pool %s for %s IPFamily", a.PoolName, serviceIPFamily)
}
