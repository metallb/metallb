// SPDX-License-Identifier:Apache-2.0

// Package netutil contains shared helpers for working with network addresses.
package netutil

import (
	"net"
	"net/netip"
)

// AddrFromNetAddr extracts a normalized netip.Addr from a net.Addr.
//
// It supports the address types returned by network interfaces and IP
// connections. IPv4-mapped IPv6 addresses are returned as IPv4 addresses.
// The boolean is false for unsupported address types and invalid IP addresses.
func AddrFromNetAddr(a net.Addr) (netip.Addr, bool) {
	var ip net.IP
	var zone string

	switch v := a.(type) {
	case *net.IPNet:
		if v == nil {
			return netip.Addr{}, false
		}
		ip = v.IP
	case *net.IPAddr:
		if v == nil {
			return netip.Addr{}, false
		}
		ip, zone = v.IP, v.Zone
	case *net.TCPAddr:
		if v == nil {
			return netip.Addr{}, false
		}
		ip, zone = v.IP, v.Zone
	case *net.UDPAddr:
		if v == nil {
			return netip.Addr{}, false
		}
		ip, zone = v.IP, v.Zone
	default:
		return netip.Addr{}, false
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}
	if addr.Is6() && zone != "" {
		addr = addr.WithZone(zone)
	}
	addr = addr.Unmap()
	return addr, addr.IsValid()
}
