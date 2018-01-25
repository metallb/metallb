// Copyright 2015 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"net"
	"testing"

	"github.com/mikioh/ipaddr"
)

func toPosition(s1, s2 string) *ipaddr.Position {
	if s1 == "" || s2 == "" {
		return nil
	}
	return &ipaddr.Position{IP: net.ParseIP(s1), Prefix: *toPrefix(s2)}
}

func TestPositionIsBroadcast(t *testing.T) {
	for i, tt := range []struct {
		in *ipaddr.Position
		ok bool
	}{
		{toPosition("192.168.0.255", "192.168.0.0/24"), true},
		{toPosition("255.255.255.255", "0.0.0.0/0"), true},

		{toPosition("192.168.0.0", "192.168.0.0/24"), false},
		{toPosition("224.0.0.0", "224.0.0.0/4"), false},
		{toPosition("239.255.255.255", "224.0.0.0/4"), false},
		{toPosition("0.0.0.0", "0.0.0.0/0"), false},

		{toPosition("2001:db8::", "2001:db8::/64"), false},
		{toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"), false},
		{toPosition("ff00::", "ff00::/8"), false},
		{toPosition("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "ff00::/8"), false},
		{toPosition("::1", "::1/128"), false},
		{toPosition("::", "::/0"), false},
	} {
		if ok := tt.in.IsBroadcast(); ok != tt.ok {
			t.Errorf("#%d: got %v for %v; want %v", i, ok, tt.in, tt.ok)
		}
	}
}

func TestPositionIsSubnetRouterAnycast(t *testing.T) {
	for i, tt := range []struct {
		in *ipaddr.Position
		ok bool
	}{
		{toPosition("2001:db8::", "2001:db8::/64"), true},

		{toPosition("2001:db8::ffff:ffff:ffff:ffff", "2001:db8::/64"), false},
		{toPosition("ff00::", "ff00::/8"), false},
		{toPosition("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "ff00::/8"), false},
		{toPosition("::1", "::1/128"), false},
		{toPosition("::", "::/0"), false},

		{toPosition("192.168.0.0", "192.168.0.0/24"), false},
		{toPosition("192.168.0.255", "192.168.0.0/24"), false},
		{toPosition("255.255.255.255", "0.0.0.0/0"), false},
		{toPosition("224.0.0.0", "224.0.0.0/4"), false},
		{toPosition("239.255.255.255", "224.0.0.0/4"), false},
		{toPosition("0.0.0.0", "0.0.0.0/0"), false},
	} {
		if ok := tt.in.IsSubnetRouterAnycast(); ok != tt.ok {
			t.Errorf("#%d: got %v for %v; want %v", i, ok, tt.in, tt.ok)
		}
	}
}
