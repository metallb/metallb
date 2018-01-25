// Copyright 2015 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/mikioh/ipaddr"
)

func TestCursorFirstLastIP(t *testing.T) {
	for i, tt := range []struct {
		in          []ipaddr.Prefix
		first, last net.IP
	}{
		// IPv4 prefixes
		{
			toPrefixes([]string{
				"0.0.0.0/0",
				"255.255.255.255/32",
			}),
			net.ParseIP("0.0.0.0"),
			net.ParseIP("255.255.255.255"),
		},
		{
			toPrefixes([]string{
				"192.168.0.0/32", "192.168.0.1/32", "192.168.0.2/32", "192.168.0.3/32",
				"192.168.4.0/24", "192.168.0.0/32", "192.168.0.1/32",
			}),
			net.ParseIP("192.168.0.0"),
			net.ParseIP("192.168.4.255"),
		},
		{
			toPrefixes([]string{"192.168.0.1/32"}),
			net.ParseIP("192.168.0.1"),
			net.ParseIP("192.168.0.1"),
		},

		// IPv6 prefixes
		{
			toPrefixes([]string{
				"::/0",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128",
			}),
			net.ParseIP("::"),
			net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"),
		},
		{
			toPrefixes([]string{
				"2001:db8::/64", "2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64",
				"2001:db8:0:4::/64", "2001:db8::/64", "2001:db8::1/64",
			}),
			net.ParseIP("2001:db8::"),
			net.ParseIP("2001:db8:0:4:ffff:ffff:ffff:ffff"),
		},
		{
			toPrefixes([]string{"2001:db8::1/128"}),
			net.ParseIP("2001:db8::1"),
			net.ParseIP("2001:db8::1"),
		},

		// Mixed prefixes
		{
			toPrefixes([]string{
				"192.168.0.1/32",
				"2001:db8::1/64",
				"192.168.255.0/24",
			}),
			net.ParseIP("192.168.0.1"),
			net.ParseIP("2001:db8::ffff:ffff:ffff:ffff"),
		},
	} {
		c := ipaddr.NewCursor(tt.in)
		fpos, lpos := c.First(), c.Last()
		if !tt.first.Equal(fpos.IP) || !tt.last.Equal(lpos.IP) {
			t.Errorf("#%d: got %v, %v; want %v, %v", i, fpos.IP, lpos.IP, tt.first, tt.last)
		}
	}
}

func TestCursorFirstLastPrefix(t *testing.T) {
	for i, tt := range []struct {
		in          []ipaddr.Prefix
		first, last *ipaddr.Prefix
	}{
		// IPv4 prefixes
		{
			toPrefixes([]string{
				"0.0.0.0/0",
				"255.255.255.255/32",
			}),
			toPrefix("0.0.0.0/0"),
			toPrefix("255.255.255.255/32"),
		},
		{
			toPrefixes([]string{
				"192.168.0.0/32", "192.168.0.1/32", "192.168.0.2/32", "192.168.0.3/32",
				"192.168.4.0/24", "192.168.0.0/32", "192.168.0.1/32",
			}),
			toPrefix("192.168.0.0/32"),
			toPrefix("192.168.4.0/24"),
		},

		// IPv6 prefixes
		{
			toPrefixes([]string{
				"::/0",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128",
			}),
			toPrefix("::/0"),
			toPrefix("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128"),
		},
		{
			toPrefixes([]string{
				"2001:db8::/64", "2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64",
				"2001:db8:0:4::/64", "2001:db8::/64", "2001:db8::1/64",
			}),
			toPrefix("2001:db8::/64"),
			toPrefix("2001:db8:0:4::/64"),
		},

		// Mixed prefixes
		{
			toPrefixes([]string{
				"192.168.0.1/32",
				"2001:db8::1/64",
				"192.168.255.0/24",
			}),
			toPrefix("192.168.0.1/32"),
			toPrefix("2001:db8::/64"),
		},
	} {
		c := ipaddr.NewCursor(tt.in)
		fpos, lpos := c.First(), c.Last()
		if !tt.first.Equal(&fpos.Prefix) || !tt.last.Equal(&lpos.Prefix) {
			t.Errorf("#%d: got %v, %v; want %v, %v", i, fpos.Prefix, lpos.Prefix, tt.first, tt.last)
		}
	}
}

func TestCursorPrevNext(t *testing.T) {
	for i, tt := range []struct {
		ps               []ipaddr.Prefix
		in, pwant, nwant *ipaddr.Position
	}{
		// IPv4 prefixes
		{
			toPrefixes([]string{"192.168.0.0/24"}),
			toPosition("192.168.0.0", "192.168.0.0/24"),
			nil,
			toPosition("192.168.0.1", "192.168.0.0/24"),
		},
		{
			toPrefixes([]string{"192.168.0.0/24"}),
			toPosition("192.168.0.255", "192.168.0.0/24"),
			toPosition("192.168.0.254", "192.168.0.0/24"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "192.168.1.0/24"}),
			toPosition("192.168.0.255", "192.168.0.0/24"),
			toPosition("192.168.0.254", "192.168.0.0/24"),
			toPosition("192.168.1.0", "192.168.1.0/24"),
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "192.168.1.0/24"}),
			toPosition("192.168.1.0", "192.168.1.0/24"),
			toPosition("192.168.0.255", "192.168.0.0/24"),
			toPosition("192.168.1.1", "192.168.1.0/24"),
		},

		// IPv6 prefixes
		{
			toPrefixes([]string{"2001:db8::/64"}),
			toPosition("2001:db8::", "2001:db8::/64"),
			nil,
			toPosition("2001:db8::1", "2001:db8::/64"),
		},
		{
			toPrefixes([]string{"2001:db8::/64"}),
			toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"),
			toPosition("2001:db8::ffff:ffff:ffff:fffe", "2001:db8::/64"),
			nil,
		},
		{
			toPrefixes([]string{"2001:db8::/64", "2001:db8:1::/64"}),
			toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"),
			toPosition("2001:db8::ffff:ffff:ffff:fffe", "2001:db8::/64"),
			toPosition("2001:db8:1::", "2001:db8:1::/64"),
		},
		{
			toPrefixes([]string{"2001:db8::/64", "2001:db8:1::/64"}),
			toPosition("2001:db8:1::", "2001:db8:1::/64"),
			toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"),
			toPosition("2001:db8:1::1", "2001:db8:1::/64"),
		},

		// Mixed prefixes
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"),
			toPosition("2001:db8::ffff:ffff:ffff:fffe", "2001:db8::/64"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("192.168.0.255", "192.168.0.0/24"),
			toPosition("192.168.0.254", "192.168.0.0/24"),
			toPosition("2001:db8::", "2001:db8::/64"),
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "::/64"}),
			toPosition("192.168.0.255", "192.168.0.0/24"),
			toPosition("192.168.0.254", "192.168.0.0/24"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "::/64"}),
			toPosition("::ffff:ffff:ffff:ffff", "::/64"),
			toPosition("::ffff:ffff:ffff:fffe", "::/64"),
			toPosition("192.168.0.0", "192.168.0.0/24"),
		},
	} {
		c := ipaddr.NewCursor(tt.ps)
		if err := c.Set(tt.in); err != nil {
			t.Fatal(err)
		}
		out := c.Prev()
		if !reflect.DeepEqual(out, tt.pwant) {
			t.Errorf("#%d: got %v; want %v", i, out, tt.pwant)
		}
		if err := c.Set(tt.in); err != nil {
			t.Fatal(err)
		}
		out = c.Next()
		if !reflect.DeepEqual(out, tt.nwant) {
			t.Errorf("#%d: got %v; want %v", i, out, tt.nwant)
		}
	}
}

func TestCursorReset(t *testing.T) {
	for i, tt := range []struct {
		in  []ipaddr.Prefix
		out *ipaddr.Position
	}{
		// IPv4 prefixes
		{
			toPrefixes([]string{"192.168.0.0/24"}),
			toPosition("192.168.0.0", "192.168.0.0/24"),
		},

		// IPv6 prefixes
		{
			toPrefixes([]string{"2001:db8::/64"}),
			toPosition("2001:db8::", "2001:db8::/64"),
		},

		// Mixed prefixes
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("192.168.0.0", "192.168.0.0/24"),
		},

		{
			nil,
			toPosition("0.0.0.0", "0.0.0.0/0"),
		},
	} {
		in := toPrefixes([]string{"0.0.0.0/0"})
		c := ipaddr.NewCursor(in)
		c.Reset(tt.in)
		if !reflect.DeepEqual(c.Pos(), tt.out) {
			t.Errorf("#%d: got %v; want %v", i, c.Pos(), tt.out)
		}
	}
}

func TestCursorPos(t *testing.T) {
	for i, tt := range []struct {
		ps []ipaddr.Prefix
		in *ipaddr.Position
		error
	}{
		// IPv4 prefixes
		{
			toPrefixes([]string{"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24"}),
			toPosition("192.168.1.1", "192.168.1.0/24"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24"}),
			toPosition("192.168.3.1", "192.168.1.0/24"),
			errors.New("should fail"),
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24"}),
			toPosition("192.168.1.1", "192.168.3.0/24"),
			errors.New("should fail"),
		},

		// IPv6 prefixes
		{
			toPrefixes([]string{"2001:db8::/64", "2001:db8:1::/64", "2001:db8:2::/64"}),
			toPosition("2001:db8:1::1", "2001:db8:1::/64"),
			nil,
		},
		{
			toPrefixes([]string{"2001:db8::/64", "2001:db8:1::/64", "2001:db8:2::/64"}),
			toPosition("2001:db8:3::1", "2001:db8:1::/64"),
			errors.New("should fail"),
		},
		{
			toPrefixes([]string{"2001:db8::/64", "2001:db8:1::/64", "2001:db8:2::/64"}),
			toPosition("2001:db8:1::1", "2001:db8:3::/64"),
			errors.New("should fail"),
		},

		// Mixed prefixes
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("192.168.0.1", "192.168.0.0/24"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("2001:db8::1", "2001:db8::/64"),
			nil,
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("2001:db8::1", "192.168.0.0/24"),
			errors.New("should fail"),
		},
		{
			toPrefixes([]string{"192.168.0.0/24", "2001:db8::/64"}),
			toPosition("192.168.0.1", "2001:db8::/64"),
			errors.New("should fail"),
		},
	} {
		c := ipaddr.NewCursor(tt.ps)
		err := c.Set(tt.in)
		if err != nil && tt.error == nil {
			t.Errorf("#%d: got %v; want %v", i, err, tt.error)
		}
		if err != nil {
			continue
		}
		if !reflect.DeepEqual(c.Pos(), tt.in) {
			t.Errorf("#%d: got %v; want %v", i, c.Pos(), tt.in)
		}
	}
}

func TestNewCursor(t *testing.T) {
	for i, tt := range []struct {
		in []string
	}{
		// IPv4 prefixes
		{
			[]string{
				"192.168.0.0/32", "192.168.0.1/32", "192.168.0.2/32", "192.168.0.3/32",
				"192.168.4.0/24", "192.168.0.0/32", "192.168.0.1/32",
			},
		},

		// IPv6 prefixes
		{
			[]string{
				"2001:db8::/64", "2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64",
				"2001:db8:0:4::/64", "2001:db8::/64", "2001:db8::1/64",
			},
		},

		// Mixed prefixes
		{
			[]string{
				"192.168.0.0/32", "192.168.0.1/32", "192.168.0.2/32", "192.168.0.3/32",
				"192.168.4.0/24", "192.168.0.0/32", "192.168.0.1/32", "2001:db8::/64",
				"2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64", "2001:db8:0:4::/64",
				"2001:db8::/64", "2001:db8::1/64",
			},
		},
	} {
		in, orig := toPrefixes(tt.in), toPrefixes(tt.in)
		ipaddr.NewCursor(in)
		if !reflect.DeepEqual(in, orig) {
			t.Errorf("#%d: %v is corrupted; want %v", i, in, orig)
		}
	}

	ipaddr.NewCursor(nil)
}
