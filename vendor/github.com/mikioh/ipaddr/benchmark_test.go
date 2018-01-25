// Copyright 2015 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"net"
	"testing"

	"github.com/mikioh/ipaddr"
)

var (
	aggregatablePrefixesIPv4 = toPrefixes([]string{
		"192.0.2.0/28", "192.0.2.16/28", "192.0.2.32/28", "192.0.2.48/28",
		"192.0.2.64/28", "192.0.2.80/28", "192.0.2.96/28", "192.0.2.112/28",
		"192.0.2.128/28", "192.0.2.144/28", "192.0.2.160/28", "192.0.2.176/28",
		"192.0.2.192/28", "192.0.2.208/28", "192.0.2.224/28", "192.0.2.240/28",
		"198.51.100.0/24", "203.0.113.0/24",
	})
	aggregatablePrefixesIPv6 = toPrefixes([]string{
		"2001:db8::/64", "2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64",
		"2001:db8:0:4::/64", "2001:db8:0:5::/64", "2001:db8:0:6::/64", "2001:db8:0:7::/64",
		"2001:db8:cafe::/64", "2001:db8:babe::/64",
	})
)

func BenchmarkAggregate(b *testing.B) {
	for _, bb := range []struct {
		name string
		ps   []ipaddr.Prefix
	}{
		{"IPv4", aggregatablePrefixesIPv4},
		{"IPv6", aggregatablePrefixesIPv6},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ipaddr.Aggregate(bb.ps)
			}
		})
	}
}

func BenchmarkCompare(b *testing.B) {
	for _, bb := range []struct {
		name   string
		px, py *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/25"), toPrefix("192.0.2.128/25")},
		{"IPv6", toPrefix("2001:db8:f001:f002::/64"), toPrefix("2001:db8:f001:f003::/64")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ipaddr.Compare(bb.px, bb.py)
			}
		})
	}
}

func BenchmarkSummarize(b *testing.B) {
	for _, bb := range []struct {
		name     string
		fip, lip net.IP
	}{
		{"IPv4", net.IPv4(192, 0, 2, 1), net.IPv4(192, 0, 2, 255)},
		{"IPv6", net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::00ff")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ipaddr.Summarize(bb.fip, bb.lip)
			}
		})
	}
}

func BenchmarkSupernet(b *testing.B) {
	for _, bb := range []struct {
		name string
		ps   []ipaddr.Prefix
	}{
		{"IPv4", aggregatablePrefixesIPv4},
		{"IPv6", aggregatablePrefixesIPv6},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ipaddr.Supernet(bb.ps)
			}
		})
	}
}

func BenchmarkCursorNext(b *testing.B) {
	for _, bb := range []struct {
		name string
		c    *ipaddr.Cursor
	}{
		{"IPv4", ipaddr.NewCursor(toPrefixes([]string{"192.0.2.0/24"}))},
		{"IPv6", ipaddr.NewCursor(toPrefixes([]string{"2001:db8::/120"}))},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for bb.c.Next() != nil {
				}
			}
		})
	}
}

func BenchmarkCursorPrev(b *testing.B) {
	for _, bb := range []struct {
		name string
		c    *ipaddr.Cursor
	}{
		{"IPv4", ipaddr.NewCursor(toPrefixes([]string{"192.0.2.255/24"}))},
		{"IPv6", ipaddr.NewCursor(toPrefixes([]string{"2001:db8::ff/120"}))},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for bb.c.Prev() != nil {
				}
			}
		})
	}
}

func BenchmarkPrefixEqual(b *testing.B) {
	for _, bb := range []struct {
		name   string
		px, py *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/25"), toPrefix("192.0.2.128/25")},
		{"IPv6", toPrefix("2001:db8:f001:f002::/64"), toPrefix("2001:db8:f001:f003::/64")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.px.Equal(bb.py)
			}
		})
	}
}

func BenchmarkPrefixExclude(b *testing.B) {
	for _, bb := range []struct {
		name   string
		px, py *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/24"), toPrefix("192.0.2.192/32")},
		{"IPv6", toPrefix("2001:db8::/120"), toPrefix("2001:db8::1/128")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.px.Exclude(bb.py)
			}
		})
	}
}

func BenchmarkPrefixMarshalBinary(b *testing.B) {
	for _, bb := range []struct {
		name string
		p    *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/31")},
		{"IPv6", toPrefix("2001:db8:cafe:babe::/127")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.p.MarshalBinary()
			}
		})
	}
}

func BenchmarkPrefixMarshalText(b *testing.B) {
	for _, bb := range []struct {
		name string
		p    *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/31")},
		{"IPv6", toPrefix("2001:db8:cafe:babe::/127")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.p.MarshalText()
			}
		})
	}
}

func BenchmarkPrefixOverlaps(b *testing.B) {
	for _, bb := range []struct {
		name   string
		px, py *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/25"), toPrefix("192.0.2.128/25")},
		{"IPv6", toPrefix("2001:db8:f001:f002::/64"), toPrefix("2001:db8:f001:f003::/64")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.px.Overlaps(bb.py)
			}
		})
	}
}

func BenchmarkPrefixSubnets(b *testing.B) {
	for _, bb := range []struct {
		name string
		p    *ipaddr.Prefix
	}{
		{"IPv4", toPrefix("192.0.2.0/24")},
		{"IPv6", toPrefix("2001:db8::/32")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.p.Subnets(3)
			}
		})
	}
}

func BenchmarkPrefixUnmarshalBinary(b *testing.B) {
	for _, bb := range []struct {
		name string
		p    *ipaddr.Prefix
		nlri []byte
	}{
		{"IPv4", toPrefix("0.0.0.0/0"), []byte{22, 192, 168, 0}},
		{"IPv6", toPrefix("::/0"), []byte{66, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0xca, 0xfe, 0x80}},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.p.UnmarshalBinary(bb.nlri)
			}
		})
	}
}

func BenchmarkPrefixUnmarshalTextIPv4(b *testing.B) {
	for _, bb := range []struct {
		name string
		p    *ipaddr.Prefix
		lit  []byte
	}{
		{"IPv4", toPrefix("0.0.0.0/0"), []byte("192.168.0.0/31")},
		{"IPv6", toPrefix("::/0"), []byte("2001:db8:cafe:babe::/127")},
	} {
		b.Run(bb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bb.p.UnmarshalText(bb.lit)
			}
		})
	}
}
