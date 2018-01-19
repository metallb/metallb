// Package ndptest provides test functions and types for package ndp.
package ndptest

import (
	"bytes"
	"fmt"
	"net"
)

// Shared test data for commonly needed data types.
var (
	Prefix = MustIPv6("2001:db8::")
	IP     = MustIPv6("2001:db8::1")
	MAC    = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
)

// Merge merges a slice of byte slices into a single, contiguous slice.
func Merge(bs [][]byte) []byte {
	var b []byte
	for _, bb := range bs {
		b = append(b, bb...)
	}

	return b
}

// Zero returns a byte slice of size n filled with zeros.
func Zero(n int) []byte {
	return bytes.Repeat([]byte{0x00}, n)
}

// MustIPv6 parses s as a valid IPv6 address, or it panics.
func MustIPv6(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		panic(fmt.Sprintf("invalid IPv6 address: %q", s))
	}

	return ip
}
