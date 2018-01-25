// Copyright 2015 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipaddr_test

import (
	"reflect"
	"testing"

	"github.com/mikioh/ipaddr"
)

func TestParse(t *testing.T) {
	for i, tt := range []struct {
		in  string
		pos *ipaddr.Position
		ps  []ipaddr.Prefix
	}{
		// IPv4 addresses, prefixes
		{
			"192.168.0.1",
			toPosition("192.168.0.1", "192.168.0.1/32"),
			toPrefixes([]string{
				"192.168.0.1/32",
			}),
		},
		{
			"192.168.0.1/32",
			toPosition("192.168.0.1", "192.168.0.1/32"),
			toPrefixes([]string{
				"192.168.0.1/32",
			}),
		},
		{
			"192.168.0.1/24",
			toPosition("192.168.0.1", "192.168.0.0/24"),
			toPrefixes([]string{
				"192.168.0.0/24",
			}),
		},
		{
			"192.168.0.2,192.168.0.2/24",
			toPosition("192.168.0.0", "192.168.0.0/24"),
			toPrefixes([]string{
				"192.168.0.0/24",
				"192.168.0.2/32",
			}),
		},
		{
			"192.168.0.2/24,192.168.0.2",
			toPosition("192.168.0.0", "192.168.0.0/24"),
			toPrefixes([]string{
				"192.168.0.0/24",
				"192.168.0.2/32",
			}),
		},

		// IPv6 addresses, prefixes
		{
			"2001:db8::1",
			toPosition("2001:db8::1", "2001:db8::1/128"),
			toPrefixes([]string{
				"2001:db8::1/128",
			}),
		},
		{
			"2001:db8::1/128",
			toPosition("2001:db8::1", "2001:db8::1/128"),
			toPrefixes([]string{
				"2001:db8::1/128",
			}),
		},
		{
			"2001:db8::1/64",
			toPosition("2001:db8::1", "2001:db8::/64"),
			toPrefixes([]string{
				"2001:db8::/64",
			}),
		},
		{
			"2001:db8::2,2001:db8::2/64",
			toPosition("2001:db8::", "2001:db8::/64"),
			toPrefixes([]string{
				"2001:db8::/64",
				"2001:db8::2/128",
			}),
		},
		{
			"2001:db8::2/64,2001:db8::2",
			toPosition("2001:db8::", "2001:db8::/64"),
			toPrefixes([]string{
				"2001:db8::/64",
				"2001:db8::2/128",
			}),
		},

		// Mixed addresses, prefixes
		{
			"192.168.0.3,192.168.0.3/24,172.16.0.0/16,2001:db8::3,2001:db8::3/64",
			toPosition("172.16.0.0", "172.16.0.0/16"),
			toPrefixes([]string{
				"172.16.0.0/16",
				"192.168.0.0/24",
				"192.168.0.3/32",
				"2001:db8::/64",
				"2001:db8::3/128",
			}),
		},
		{
			"2001:db8::3,2001:db8::3/64,192.168.0.3,192.168.0.3/24,172.16.0.0/16",
			toPosition("172.16.0.0", "172.16.0.0/16"),
			toPrefixes([]string{
				"172.16.0.0/16",
				"192.168.0.0/24",
				"192.168.0.3/32",
				"2001:db8::/64",
				"2001:db8::3/128",
			}),
		},
	} {
		out, err := ipaddr.Parse(tt.in)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(out.Pos(), tt.pos) {
			t.Errorf("#%d: got %v; want %v", i, out.Pos(), tt.pos)
		}
		if !reflect.DeepEqual(out.List(), tt.ps) {
			t.Errorf("#%d: got %v; want %v", i, out.List(), tt.ps)
		}
	}
}
