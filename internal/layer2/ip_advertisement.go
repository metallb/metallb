// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"net/netip"

	"k8s.io/apimachinery/pkg/util/sets"
)

// IPAdvertisement is the advertisement Info about LB IP.
type IPAdvertisement struct {
	ip            netip.Addr
	interfaces    sets.Set[string]
	allInterfaces bool
}

// NewIPAdvertisement creates an advertisement, unmapping an IPv4-mapped IPv6 IP.
func NewIPAdvertisement(ip netip.Addr, allInterfaces bool, interfaces sets.Set[string]) IPAdvertisement {
	return IPAdvertisement{
		ip:            ip.Unmap(),
		interfaces:    interfaces,
		allInterfaces: allInterfaces,
	}
}

func (i *IPAdvertisement) Equal(other *IPAdvertisement) bool {
	if i == nil && other == nil {
		return true
	}
	if i == nil || other == nil {
		return false
	}
	if i.ip != other.ip {
		return false
	}
	if i.allInterfaces != other.allInterfaces {
		return false
	}
	if i.allInterfaces {
		return true
	}
	return i.interfaces.Equal(other.interfaces)
}

func (i *IPAdvertisement) MatchInterfaces(intfs ...string) bool {
	if i.allInterfaces {
		return true
	}
	for _, intf := range intfs {
		if i.matchInterface(intf) {
			return true
		}
	}
	return false
}

func (i *IPAdvertisement) matchInterface(intf string) bool {
	if i == nil {
		return false
	}
	if i.allInterfaces {
		return true
	}
	return i.interfaces.Has(intf)
}
func (i *IPAdvertisement) IsAllInterfaces() bool {
	return i.allInterfaces
}

func (i *IPAdvertisement) GetInterfaces() sets.Set[string] {
	return i.interfaces
}
