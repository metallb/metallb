// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"net"

	"k8s.io/apimachinery/pkg/util/sets"
)

// IPAdvertisement is the advertisement Info about LB IP.
type IPAdvertisement struct {
	ip            net.IP
	interfaces    sets.String
	allInterfaces bool
}

func NewIPAdvertisement(ip net.IP, allInterfaces bool, interfaces sets.String) IPAdvertisement {
	return IPAdvertisement{
		ip:            ip,
		interfaces:    interfaces,
		allInterfaces: allInterfaces,
	}
}

func (i1 *IPAdvertisement) Equal(i2 *IPAdvertisement) bool {
	if i1 == nil && i2 == nil {
		return true
	}
	if i1 == nil || i2 == nil {
		return false
	}
	if !i1.ip.Equal(i2.ip) {
		return false
	}
	if i1.allInterfaces != i2.allInterfaces {
		return false
	}
	if i1.allInterfaces == true {
		return true
	}
	return i1.interfaces.Equal(i2.interfaces)
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
