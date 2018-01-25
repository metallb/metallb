// Copyright 2013 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"bytes"
	"math/big"
	"net"
	"reflect"
	"sort"
	"testing"

	"github.com/mikioh/ipaddr"
)

type byAscending []ipaddr.Prefix

func (ps byAscending) Len() int           { return len(ps) }
func (ps byAscending) Less(i, j int) bool { return compareAscending(&ps[i], &ps[j]) < 0 }
func (ps byAscending) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

func compareAscending(a, b *ipaddr.Prefix) int {
	if n := bytes.Compare(a.IP, b.IP); n != 0 {
		return n
	}
	if n := bytes.Compare(a.Mask, b.Mask); n != 0 {
		return n
	}
	return 0
}

func toPrefix(s string) *ipaddr.Prefix {
	if s == "" {
		return nil
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return ipaddr.NewPrefix(n)
}

func toPrefixes(ss []string) []ipaddr.Prefix {
	if ss == nil {
		return nil
	}
	var ps []ipaddr.Prefix
	for _, s := range ss {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil
		}
		ps = append(ps, *ipaddr.NewPrefix(n))
	}
	return ps
}

func TestAggregate(t *testing.T) {
	for i, tt := range []struct {
		in, want []string
	}{
		// IPv4 prefixes
		{
			[]string{
				"192.0.2.0/25", "192.0.2.128/25",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/26", "192.0.2.64/26", "192.0.2.128/26", "192.0.2.192/26",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/27", "192.0.2.32/27", "192.0.2.64/27", "192.0.2.96/27",
				"192.0.2.128/27", "192.0.2.160/27", "192.0.2.192/27", "192.0.2.224/27",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/28", "192.0.2.16/28", "192.0.2.32/28", "192.0.2.48/28",
				"192.0.2.64/28", "192.0.2.80/28", "192.0.2.96/28", "192.0.2.112/28",
				"192.0.2.128/28", "192.0.2.144/28", "192.0.2.160/28", "192.0.2.176/28",
				"192.0.2.192/28", "192.0.2.208/28", "192.0.2.224/28", "192.0.2.240/28",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/29", "192.0.2.8/29", "192.0.2.16/29", "192.0.2.24/29",
				"192.0.2.32/29", "192.0.2.40/29", "192.0.2.48/29", "192.0.2.56/29",
				"192.0.2.64/29", "192.0.2.72/29", "192.0.2.80/29", "192.0.2.88/29",
				"192.0.2.96/29", "192.0.2.104/29", "192.0.2.112/29", "192.0.2.120/29",
				"192.0.2.128/29", "192.0.2.136/29", "192.0.2.144/29", "192.0.2.152/29",
				"192.0.2.160/29", "192.0.2.168/29", "192.0.2.176/29", "192.0.2.184/29",
				"192.0.2.192/29", "192.0.2.200/29", "192.0.2.208/29", "192.0.2.216/29",
				"192.0.2.224/29", "192.0.2.232/29", "192.0.2.240/29", "192.0.2.248/29",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/26", "192.0.2.64/26", "192.0.2.192/26",
				"192.0.2.128/28", "192.0.2.144/28", "192.0.2.160/28", "192.0.2.176/28",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.1/32", "192.0.2.1/32",
			},
			[]string{
				"192.0.2.1/32",
			},
		},
		{
			[]string{
				"192.0.2.0/25", "192.0.2.128/25",
				"192.0.2.248/29",
			},
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/24",
				"198.51.100.0/24",
				"203.0.113.0/24",
			},
			[]string{
				"192.0.2.0/24",
				"198.51.100.0/24",
				"203.0.113.0/24",
			},
		},
		{
			[]string{
				"192.0.2.0/25",
				"192.0.2.0/26",
				"192.0.2.0/27",
				"192.0.2.0/28",
				"192.0.2.0/29",
				"192.0.2.0/30",
			},
			[]string{
				"192.0.2.0/25",
			},
		},
		{
			[]string{
				"0.0.0.0/0",
				"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24",
				"255.255.255.255/32",
			},
			[]string{
				"0.0.0.0/0",
			},
		},
		{
			[]string{
				"0.0.0.0/0", "0.0.0.0/0",
				"255.255.255.255/32", "255.255.255.255/32",
			},
			[]string{
				"0.0.0.0/0",
			},
		},
		{
			[]string{
				"192.168.0.0/25", "192.168.0.128/25",
				"192.168.1.0/24", "192.168.3.0/24", "192.168.4.0/24",
				"192.168.5.0/26",
				"192.168.128.0/22", "192.168.132.0/22",
				"192.168.128.0/21",
			},
			[]string{
				"192.168.0.0/23",
				"192.168.3.0/24", "192.168.4.0/24",
				"192.168.5.0/26",
				"192.168.128.0/21",
			},
		},
		{
			[]string{
				"192.168.0.0/25", "192.168.0.128/25",
				"192.168.1.0/24", "192.168.3.0/24", "192.168.4.0/24",
				"192.168.5.0/26",
			},
			[]string{
				"192.168.0.0/23",
				"192.168.3.0/24", "192.168.4.0/24",
				"192.168.5.0/26",
			},
		},

		// IPv6 prefixes
		{
			[]string{
				"2001:db8::/64", "2001:db8:0:1::/64", "2001:db8:0:2::/64", "2001:db8:0:3::/64",
				"2001:db8:0:4::/64",
			},
			[]string{
				"2001:db8::/62",
				"2001:db8:0:4::/64",
			},
		},
		{
			[]string{
				"::/0",
				"2001:db8::/32",
				"2001:db8::/126",
				"2001:db8::/127",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128",
			},
			[]string{
				"::/0",
			},
		},
		{
			[]string{
				"::/0", "::/0",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128",
			},
			[]string{
				"::/0",
			},
		},
	} {
		in, orig, want := toPrefixes(tt.in), toPrefixes(tt.in), toPrefixes(tt.want)
		sort.Sort(byAscending(want))
		out := ipaddr.Aggregate(in)
		if !reflect.DeepEqual(out, want) {
			t.Errorf("#%d: got %v; want %v", i, out, want)
		}
		if !reflect.DeepEqual(in, orig) {
			t.Errorf("#%d: %v is corrupted; want %v", i, in, orig)
		}
	}

	ipaddr.Aggregate(nil)
}

func TestCompare(t *testing.T) {
	for i, tt := range []struct {
		in []ipaddr.Prefix
		n  int
	}{
		{toPrefixes([]string{"192.0.2.0/23", "192.0.2.0/24"}), -1},
		{toPrefixes([]string{"192.0.2.0/24", "192.0.2.0/24"}), 0},
		{toPrefixes([]string{"192.0.2.0/25", "192.0.2.0/24"}), +1},

		{toPrefixes([]string{"192.0.2.0/24", "198.51.100.0/24"}), -1},
		{toPrefixes([]string{"198.51.100.0/24", "198.51.100.0/24"}), 0},
		{toPrefixes([]string{"203.0.113.0/24", "198.51.100.0/24"}), +1},

		{toPrefixes([]string{"2001:db8:1::/47", "2001:db8:1::/48"}), -1},
		{toPrefixes([]string{"2001:db8:1::/48", "2001:db8:1::/48"}), 0},
		{toPrefixes([]string{"2001:db8:1::/49", "2001:db8:1::/48"}), +1},

		{toPrefixes([]string{"2001:db8:1::/128", "2001:db8:1::1/128"}), -1},
		{toPrefixes([]string{"2001:db8:1::1/128", "2001:db8:1::1/128"}), 0},
		{toPrefixes([]string{"2001:db8:1::2/128", "2001:db8:1::1/128"}), +1},

		{toPrefixes([]string{"192.0.2.1/24", "2001:db8:1::1/64"}), -1},
		{toPrefixes([]string{"2001:db8:1::1/64", "192.0.2.1/24"}), +1},
	} {
		if n := ipaddr.Compare(&tt.in[0], &tt.in[1]); n != tt.n {
			t.Errorf("#%d: got %v for %v; want %v", i, n, tt.in, tt.n)
		}
	}
}

func TestSummarize(t *testing.T) {
	for i, tt := range []struct {
		first, last string
		want        []string
	}{
		// IPv4 prefixes
		{
			"192.0.2.0", "192.0.2.255",
			[]string{
				"192.0.2.0/24",
			},
		},
		{
			"1.2.3.4", "5.6.7.8",
			[]string{
				"1.2.3.4/30", "1.2.3.8/29", "1.2.3.16/28", "1.2.3.32/27",
				"1.2.3.64/26", "1.2.3.128/25", "1.2.4.0/22", "1.2.8.0/21",
				"1.2.16.0/20", "1.2.32.0/19", "1.2.64.0/18", "1.2.128.0/17",
				"1.3.0.0/16", "1.4.0.0/14", "1.8.0.0/13", "1.16.0.0/12",
				"1.32.0.0/11", "1.64.0.0/10", "1.128.0.0/9", "2.0.0.0/7",
				"4.0.0.0/8", "5.0.0.0/14", "5.4.0.0/15", "5.6.0.0/22",
				"5.6.4.0/23", "5.6.6.0/24", "5.6.7.0/29", "5.6.7.8/32",
			},
		},
		{
			"255.255.255.255", "255.255.255.255",
			[]string{
				"255.255.255.255/32",
			},
		},
		{
			"0.0.0.0", "255.255.255.255",
			[]string{
				"0.0.0.0/0",
			},
		},
		{
			"0.0.0.1", "255.255.255.254",
			[]string{
				"0.0.0.1/32", "0.0.0.2/31", "0.0.0.4/30", "0.0.0.8/29",
				"0.0.0.16/28", "0.0.0.32/27", "0.0.0.64/26", "0.0.0.128/25",
				"0.0.1.0/24", "0.0.2.0/23", "0.0.4.0/22", "0.0.8.0/21",
				"0.0.16.0/20", "0.0.32.0/19", "0.0.64.0/18", "0.0.128.0/17",
				"0.1.0.0/16", "0.2.0.0/15", "0.4.0.0/14", "0.8.0.0/13",
				"0.16.0.0/12", "0.32.0.0/11", "0.64.0.0/10", "0.128.0.0/9",
				"1.0.0.0/8", "2.0.0.0/7", "4.0.0.0/6", "8.0.0.0/5",
				"16.0.0.0/4", "32.0.0.0/3", "64.0.0.0/2", "128.0.0.0/2",
				"192.0.0.0/3", "224.0.0.0/4", "240.0.0.0/5", "248.0.0.0/6",
				"252.0.0.0/7", "254.0.0.0/8", "255.0.0.0/9", "255.128.0.0/10",
				"255.192.0.0/11", "255.224.0.0/12", "255.240.0.0/13", "255.248.0.0/14",
				"255.252.0.0/15", "255.254.0.0/16", "255.255.0.0/17", "255.255.128.0/18",
				"255.255.192.0/19", "255.255.224.0/20", "255.255.240.0/21", "255.255.248.0/22",
				"255.255.252.0/23", "255.255.254.0/24", "255.255.255.0/25", "255.255.255.128/26",
				"255.255.255.192/27", "255.255.255.224/28", "255.255.255.240/29", "255.255.255.248/30",
				"255.255.255.252/31", "255.255.255.254/32",
			},
		},

		// IPv6 prefixes
		{
			"2001:db8:1::", "2001:db8:2::",
			[]string{
				"2001:db8:1::/48",
				"2001:db8:2::/128",
			},
		},
		{
			"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			[]string{
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128",
			},
		},
		{
			"::", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			[]string{
				"::/0",
			},
		},
		{
			"::1", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe",
			[]string{
				"::1/128", "::2/127", "::4/126", "::8/125",
				"::10/124", "::20/123", "::40/122", "::80/121",
				"::100/120", "::200/119", "::400/118", "::800/117",
				"::1000/116", "::2000/115", "::4000/114", "::8000/113",
				"::1:0/112", "::2:0/111", "::4:0/110", "::8:0/109",
				"::10:0/108", "::20:0/107", "::40:0/106", "::80:0/105",
				"::100:0/104", "::200:0/103", "::400:0/102", "::800:0/101",
				"::1000:0/100", "::2000:0/99", "::4000:0/98", "::8000:0/97",
				"::1:0:0/96", "::2:0:0/95", "::4:0:0/94", "::8:0:0/93",
				"::10:0:0/92", "::20:0:0/91", "::40:0:0/90", "::80:0:0/89",
				"::100:0:0/88", "::200:0:0/87", "::400:0:0/86", "::800:0:0/85",
				"::1000:0:0/84", "::2000:0:0/83", "::4000:0:0/82", "::8000:0:0/81",
				"::1:0:0:0/80", "::2:0:0:0/79", "::4:0:0:0/78", "::8:0:0:0/77",
				"::10:0:0:0/76", "::20:0:0:0/75", "::40:0:0:0/74", "::80:0:0:0/73",
				"::100:0:0:0/72", "::200:0:0:0/71", "::400:0:0:0/70", "::800:0:0:0/69",
				"::1000:0:0:0/68", "::2000:0:0:0/67", "::4000:0:0:0/66", "::8000:0:0:0/65",
				"0:0:0:1::/64", "0:0:0:2::/63", "0:0:0:4::/62", "0:0:0:8::/61",
				"0:0:0:10::/60", "0:0:0:20::/59", "0:0:0:40::/58", "0:0:0:80::/57",
				"0:0:0:100::/56", "0:0:0:200::/55", "0:0:0:400::/54", "0:0:0:800::/53",
				"0:0:0:1000::/52", "0:0:0:2000::/51", "0:0:0:4000::/50", "0:0:0:8000::/49",
				"0:0:1::/48", "0:0:2::/47", "0:0:4::/46", "0:0:8::/45",
				"0:0:10::/44", "0:0:20::/43", "0:0:40::/42", "0:0:80::/41",
				"0:0:100::/40", "0:0:200::/39", "0:0:400::/38", "0:0:800::/37",
				"0:0:1000::/36", "0:0:2000::/35", "0:0:4000::/34", "0:0:8000::/33",
				"0:1::/32", "0:2::/31", "0:4::/30", "0:8::/29",
				"0:10::/28", "0:20::/27", "0:40::/26", "0:80::/25",
				"0:100::/24", "0:200::/23", "0:400::/22", "0:800::/21",
				"0:1000::/20", "0:2000::/19", "0:4000::/18", "0:8000::/17",
				"1::/16", "2::/15", "4::/14", "8::/13",
				"10::/12", "20::/11", "40::/10", "80::/9",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ff00/121", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ff80/122", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffc0/123", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffe0/124",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:fff0/125", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fff8/126", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffc/127", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe/128",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff::/113", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:8000/114", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:c000/115", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:e000/116",
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:f000/117", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:f800/118", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fc00/119", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fe00/120",
				"ffff:ffff:ffff:ffff:ffff:ffff:ff00::/105", "ffff:ffff:ffff:ffff:ffff:ffff:ff80::/106", "ffff:ffff:ffff:ffff:ffff:ffff:ffc0::/107", "ffff:ffff:ffff:ffff:ffff:ffff:ffe0::/108",
				"ffff:ffff:ffff:ffff:ffff:ffff:fff0::/109", "ffff:ffff:ffff:ffff:ffff:ffff:fff8::/110", "ffff:ffff:ffff:ffff:ffff:ffff:fffc::/111", "ffff:ffff:ffff:ffff:ffff:ffff:fffe::/112",
				"ffff:ffff:ffff:ffff:ffff:ffff::/97", "ffff:ffff:ffff:ffff:ffff:ffff:8000::/98", "ffff:ffff:ffff:ffff:ffff:ffff:c000::/99", "ffff:ffff:ffff:ffff:ffff:ffff:e000::/100",
				"ffff:ffff:ffff:ffff:ffff:ffff:f000::/101", "ffff:ffff:ffff:ffff:ffff:ffff:f800::/102", "ffff:ffff:ffff:ffff:ffff:ffff:fc00::/103", "ffff:ffff:ffff:ffff:ffff:ffff:fe00::/104",
				"ffff:ffff:ffff:ffff:ffff:ff00::/89", "ffff:ffff:ffff:ffff:ffff:ff80::/90", "ffff:ffff:ffff:ffff:ffff:ffc0::/91", "ffff:ffff:ffff:ffff:ffff:ffe0::/92",
				"ffff:ffff:ffff:ffff:ffff:fff0::/93", "ffff:ffff:ffff:ffff:ffff:fff8::/94", "ffff:ffff:ffff:ffff:ffff:fffc::/95", "ffff:ffff:ffff:ffff:ffff:fffe::/96",
				"ffff:ffff:ffff:ffff:ffff::/81", "ffff:ffff:ffff:ffff:ffff:8000::/82", "ffff:ffff:ffff:ffff:ffff:c000::/83", "ffff:ffff:ffff:ffff:ffff:e000::/84",
				"ffff:ffff:ffff:ffff:ffff:f000::/85", "ffff:ffff:ffff:ffff:ffff:f800::/86", "ffff:ffff:ffff:ffff:ffff:fc00::/87", "ffff:ffff:ffff:ffff:ffff:fe00::/88",
				"ffff:ffff:ffff:ffff:ff00::/73", "ffff:ffff:ffff:ffff:ff80::/74", "ffff:ffff:ffff:ffff:ffc0::/75", "ffff:ffff:ffff:ffff:ffe0::/76",
				"ffff:ffff:ffff:ffff:fff0::/77", "ffff:ffff:ffff:ffff:fff8::/78", "ffff:ffff:ffff:ffff:fffc::/79", "ffff:ffff:ffff:ffff:fffe::/80",
				"ffff:ffff:ffff:ffff::/65", "ffff:ffff:ffff:ffff:8000::/66", "ffff:ffff:ffff:ffff:c000::/67", "ffff:ffff:ffff:ffff:e000::/68",
				"ffff:ffff:ffff:ffff:f000::/69", "ffff:ffff:ffff:ffff:f800::/70", "ffff:ffff:ffff:ffff:fc00::/71", "ffff:ffff:ffff:ffff:fe00::/72",
				"ffff:ffff:ffff:ff00::/57", "ffff:ffff:ffff:ff80::/58", "ffff:ffff:ffff:ffc0::/59", "ffff:ffff:ffff:ffe0::/60",
				"ffff:ffff:ffff:fff0::/61", "ffff:ffff:ffff:fff8::/62", "ffff:ffff:ffff:fffc::/63", "ffff:ffff:ffff:fffe::/64",
				"ffff:ffff:ffff::/49", "ffff:ffff:ffff:8000::/50", "ffff:ffff:ffff:c000::/51", "ffff:ffff:ffff:e000::/52",
				"ffff:ffff:ffff:f000::/53", "ffff:ffff:ffff:f800::/54", "ffff:ffff:ffff:fc00::/55", "ffff:ffff:ffff:fe00::/56",
				"ffff:ffff:ff00::/41", "ffff:ffff:ff80::/42", "ffff:ffff:ffc0::/43", "ffff:ffff:ffe0::/44",
				"ffff:ffff:fff0::/45", "ffff:ffff:fff8::/46", "ffff:ffff:fffc::/47", "ffff:ffff:fffe::/48",
				"ffff:ffff::/33", "ffff:ffff:8000::/34", "ffff:ffff:c000::/35", "ffff:ffff:e000::/36",
				"ffff:ffff:f000::/37", "ffff:ffff:f800::/38", "ffff:ffff:fc00::/39", "ffff:ffff:fe00::/40",
				"ffff:ff00::/25", "ffff:ff80::/26", "ffff:ffc0::/27", "ffff:ffe0::/28",
				"ffff:fff0::/29", "ffff:fff8::/30", "ffff:fffc::/31", "ffff:fffe::/32",
				"ffff::/17", "ffff:8000::/18", "ffff:c000::/19", "ffff:e000::/20",
				"ffff:f000::/21", "ffff:f800::/22", "ffff:fc00::/23", "ffff:fe00::/24",
				"ff00::/9", "ff80::/10", "ffc0::/11", "ffe0::/12",
				"fff0::/13", "fff8::/14", "fffc::/15", "fffe::/16",
				"100::/8", "200::/7", "400::/6", "800::/5",
				"1000::/4", "2000::/3", "4000::/2", "8000::/2",
				"c000::/3", "e000::/4", "f000::/5", "f800::/6",
				"fc00::/7", "fe00::/8",
			},
		},
	} {
		fip := net.ParseIP(tt.first)
		if fip == nil {
			t.Fatalf("non-IP address: %s", tt.first)
		}
		lip := net.ParseIP(tt.last)
		if lip == nil {
			t.Fatalf("non-IP address: %s", tt.last)
		}
		want := toPrefixes(tt.want)
		sort.Sort(byAscending(want))
		out := ipaddr.Summarize(fip, lip)
		if !reflect.DeepEqual(out, want) {
			t.Errorf("#%d: got %v; want %v", i, out, want)
		}
	}

	ipaddr.Summarize(nil, nil)
}

func TestSupernet(t *testing.T) {
	for i, tt := range []struct {
		in   []string
		want string
	}{
		// IPv4 prefixes
		{
			[]string{
				"192.0.2.0/25", "192.0.2.128/25",
			},
			"192.0.2.0/24",
		},
		{
			[]string{
				"192.0.2.0/27", "192.0.2.32/27", "192.0.2.64/27", "192.0.2.96/27",
				"192.0.2.128/27", "192.0.2.160/27", "192.0.2.192/27", "192.0.2.224/27",
			},
			"192.0.2.0/24",
		},
		{
			[]string{
				"192.0.2.0/28", "192.0.2.16/28", "192.0.2.32/28", "192.0.2.48/28",
				"192.0.2.64/28", "192.0.2.80/28", "192.0.2.96/28", "192.0.2.112/28",
				"192.0.2.128/28", "192.0.2.144/28", "192.0.2.160/28", "192.0.2.176/28",
				"192.0.2.192/28", "192.0.2.208/28", "192.0.2.224/28", "192.0.2.240/28",
			},
			"192.0.2.0/24",
		},
		{
			[]string{
				"10.40.101.1/32", "10.40.102.1/32", "11.40.103.1/32",
			},
			"10.0.0.0/7",
		},
		{
			[]string{
				"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24",
				"192.168.100.0/24", "192.168.200.0/24",
			},
			"192.168.0.0/16",
		},

		// IPv4 prefixes, no supernet
		{
			[]string{
				"128.0.0.0/24",
				"192.0.0.0/24",
				"65.0.0.0/24",
			},
			"",
		},
		{
			[]string{
				"0.0.0.0/0",
				"192.0.0.0/24",
				"65.0.0.0/24",
			},
			"",
		},

		// IPv6 prefixes
		{
			[]string{
				"2001:db8:1::/32",
				"2001:db8:2::/39",
			},
			"2001:db8::/32",
		},
		{
			[]string{
				"2013:db8:1::1/64",
				"192.168.0.1/24",
				"2013:db8:2::1/64",
			},
			"2013:db8::/46",
		},

		// IPv6 prefixes, no supernet
		{
			[]string{
				"8001:db8:1::/34",
				"2013:db8:2::/32",
			},
			"",
		},
		{
			[]string{
				"2001:db8::1/64",
				"192.168.0.1/24",
				"8001:db8::1/64",
			},
			"",
		},

		// Mixed prefixes
		{
			[]string{
				"192.0.2.0/25",
				"2001:db8::/64",
				"192.0.2.128/25",
			},
			"192.0.2.0/24",
		},

		// Mixed prefixes, no supernet
		{
			[]string{
				"0.0.0.0/0",
				"192.0.2.1/24",
				"2001:db8::1/64",
			},
			"",
		},
	} {
		in, orig := toPrefixes(tt.in), toPrefixes(tt.in)
		want := toPrefix(tt.want)
		out := ipaddr.Supernet(in)
		if !reflect.DeepEqual(out, want) {
			t.Errorf("#%d: got %v; want %v", i, out, want)
		}
		if !reflect.DeepEqual(in, orig) {
			t.Errorf("#%d: %v is corrupted; want %v", i, in, orig)
		}
	}

	ipaddr.Supernet(nil)
}

func TestPrefixBinaryMarshalerUnmarshaler(t *testing.T) {
	for i, tt := range []struct {
		in, tmp string
		want    []byte
	}{
		{
			"0.0.0.0/0", "1.2.3.4/32",
			[]byte{0},
		},
		{
			"192.0.0.0/7", "5.6.7.8/32",
			[]byte{7, 192},
		},
		{
			"192.168.0.0/23", "0.0.0.0/0",
			[]byte{23, 192, 168, 0},
		},

		{
			"::/0", "2001:db8::/8",
			[]byte{0},
		},
		{
			"2001::/8", "2001:db8::/8",
			[]byte{8, 0x20},
		},
		{
			"2001:db8:0:cafe:babe::/66", "::/0",
			[]byte{66, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0xca, 0xfe, 0x80},
		},
		{
			"2001:db8:0:cafe:babe::3/127", "::/0",
			[]byte{127, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
		},
	} {
		p1 := toPrefix(tt.in)
		out, err := p1.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(out, tt.want) {
			t.Errorf("#%d: got %v; want %v", i, out, tt.want)
		}
		p2 := toPrefix(tt.tmp)
		if err := p2.UnmarshalBinary(tt.want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p2, p1) {
			t.Errorf("#%d: got %v; want %v", i, p2, p1)
		}
	}
}

func TestPrefixIPNetContains(t *testing.T) {
	for i, tt := range []struct {
		in   string
		ip   net.IP
		want bool
	}{
		{"192.168.0.0/24", net.ParseIP("192.168.0.1"), true},

		{"192.168.0.0/24", net.ParseIP("192.168.1.1"), false},

		{"2001:db8:f001::/48", net.ParseIP("2001:db8:f001::1"), true},

		{"2001:db8:f001::/48", net.ParseIP("2001:db8:f002::1"), false},
	} {
		p := toPrefix(tt.in)
		if out := p.IPNet.Contains(tt.ip); out != tt.want {
			t.Errorf("#%d: got %v; want %v", i, out, tt.want)
		}
	}
}

func TestPrefixContains(t *testing.T) {
	for i, tt := range []struct {
		in []ipaddr.Prefix
		ok bool
	}{
		{toPrefixes([]string{"192.0.2.0/23", "192.0.2.0/24"}), true},
		{toPrefixes([]string{"192.0.2.0/24", "192.0.2.0/24"}), false},
		{toPrefixes([]string{"192.0.2.0/25", "192.0.2.0/24"}), false},

		{toPrefixes([]string{"2001:db8:1::/47", "2001:db8:1::/48"}), true},
		{toPrefixes([]string{"2001:db8:1::/48", "2001:db8:1::/48"}), false},
		{toPrefixes([]string{"2001:db8:1::/49", "2001:db8:1::/48"}), false},

		{toPrefixes([]string{"2001:db8:1::/127", "2001:db8:1::/128"}), true},
		{toPrefixes([]string{"2001:db8:1::/127", "2001:db8:1::/127"}), false},
		{toPrefixes([]string{"2001:db8:1::/128", "2001:db8:1::/127"}), false},

		{toPrefixes([]string{"192.0.2.1/24", "2001:db8:1::1/64"}), false},
		{toPrefixes([]string{"2001:db8:1::1/64", "192.0.2.1/24"}), false},
	} {
		if ok := tt.in[0].Contains(&tt.in[1]); ok != tt.ok {
			t.Errorf("#%d: got %v; want %v", i, ok, tt.ok)
		}
	}
}

func TestPrefixExclude(t *testing.T) {
	for i, tt := range []struct {
		in, excl string
	}{
		{"192.0.0.0/16", "192.0.2.0/24"},
		{"192.0.2.0/24", "192.0.2.0/32"},

		{"2001:db8:f001::/48", "2001:db8:f001:f002::/56"},
		{"2001:db8:f001:f002::/64", "2001:db8:f001:f002::cafe/128"},
	} {
		p, excl := toPrefix(tt.in), toPrefix(tt.excl)
		ps := p.Exclude(excl)
		if len(ps) != excl.Len()-p.Len() {
			for _, p := range ps {
				t.Logf("subnet: %v", p)
			}
			t.Errorf("#%d: got %v; want %v", i, len(ps), excl.Len()-p.Len())
		}
		diff, sum := big.NewInt(0), big.NewInt(0)
		diff.Sub(p.NumNodes(), excl.NumNodes())
		for _, p := range ps {
			sum.Add(sum, p.NumNodes())
		}
		if diff.String() != sum.String() {
			for _, p := range ps {
				t.Logf("subnet: %v", p)
			}
			t.Errorf("#%d: got %v; want %v", i, sum.String(), diff.String())
		}
	}
}

func TestPrefixLast(t *testing.T) {
	for i, tt := range []struct {
		in      string
		l       int
		ip, lip net.IP
	}{
		{"192.0.0.0/16", 16, net.ParseIP("192.0.0.0"), net.ParseIP("192.0.255.255")},
		{"192.0.2.255/24", 24, net.ParseIP("192.0.2.0"), net.ParseIP("192.0.2.255")},

		{"2001:db8:0:0:1:2:3:cafe/64", 64, net.ParseIP("2001:db8::"), net.ParseIP("2001:db8::ffff:ffff:ffff:ffff")},
		{"2001:db8::ca7e/121", 121, net.ParseIP("2001:db8::ca00"), net.ParseIP("2001:db8::ca7f")},
	} {
		p := toPrefix(tt.in)
		if !p.IP.Equal(tt.ip) {
			t.Errorf("#%d: got %v; want %v", i, p.IP, tt.ip)
		}
		if p.Len() != tt.l {
			t.Errorf("#%d: got %v; want %v", i, p.Len(), tt.l)
		}
		if !p.Last().Equal(tt.lip) {
			t.Errorf("#%d: got %v; want %v", i, p.Last(), tt.lip)
		}
	}
}

func TestPrefixMask(t *testing.T) {
	inverse := func(s net.IPMask) net.IPMask {
		d := make(net.IPMask, len(s))
		for i := range s {
			d[i] = ^s[i]
		}
		return d
	}

	for i, tt := range []struct {
		in string
		m  net.IPMask
	}{
		{"192.0.2.255/16", net.CIDRMask(16, ipaddr.IPv4PrefixLen)},

		{"2001:db8::/64", net.CIDRMask(64, ipaddr.IPv6PrefixLen)},
	} {
		p := toPrefix(tt.in)
		if bytes.Compare(p.Mask, tt.m) != 0 {
			t.Errorf("#%d: got %v; want %v", i, p.Mask, tt.m)
		}
		m := inverse(tt.m)
		if bytes.Compare(p.Hostmask(), m) != 0 {
			t.Errorf("#%d: got %v; want %v", i, p.Hostmask(), m)
		}
	}
}

func TestPrefixNumNodes(t *testing.T) {
	for i, tt := range []struct {
		in string
		n  *big.Int
	}{
		{"192.0.2.0/0", big.NewInt(1 << 32)},
		{"192.0.2.0/16", big.NewInt(1 << 16)},
		{"192.0.2.0/32", big.NewInt(1)},

		{"2001:db8::/0", new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil)},
		{"2001:db8::/32", new(big.Int).Exp(big.NewInt(2), big.NewInt(96), nil)},
		{"2001:db8::/64", new(big.Int).Exp(big.NewInt(2), big.NewInt(64), nil)},
		{"2001:db8::/96", new(big.Int).Exp(big.NewInt(2), big.NewInt(32), nil)},
		{"2001:db8::/128", new(big.Int).Exp(big.NewInt(2), big.NewInt(0), nil)},
	} {
		p := toPrefix(tt.in)
		if p.NumNodes().String() != tt.n.String() {
			t.Errorf("#%d: got %v; want %v", i, p.NumNodes().String(), tt.n.String())
		}
	}
}

func TestPrefixOverlaps(t *testing.T) {
	for i, tt := range []struct {
		in     string
		others []string
		want   bool
	}{
		{"192.0.2.0/24", []string{"192.0.2.0/25", "192.0.2.64/26"}, true},

		{"192.0.2.0/24", []string{"198.51.100.0/25", "198.51.100.128/25"}, false},

		{"2001:db8:f001::/48", []string{"2001:db8:f001:4000::/49", "2001:db8:f001:8000::/49"}, true},

		{"2001:db8:f001::/48", []string{"2001:db8:f002:4000::/49", "2001:db8:f002:8000::/49"}, false},
	} {
		p1 := toPrefix(tt.in)
		others := toPrefixes(tt.others)
		p2 := ipaddr.Supernet(others)
		if out := p1.Overlaps(p2); out != tt.want {
			t.Errorf("#%d: got %v; want %v", i, out, tt.want)
		}
		if out := p2.Overlaps(p1); out != tt.want {
			t.Errorf("#%d: got %v; want %v", i, out, tt.want)
		}
	}
}

func TestPrefixSubnets(t *testing.T) {
	for i, tt := range []struct {
		in string
		l  int
		n  int
	}{
		{"192.0.2.128/25", 25, 4},
		{"192.0.2.0/29", 29, 2},

		{"2001:db8::/65", 65, 8},
		{"2001:db8::/51", 51, 9},
		{"2001:db8::/32", 32, 1},
		{"2001:db8::/13", 13, 7},
		{"2001:db8::/64", 64, 3},
		{"2001:db8::/61", 61, 5},
		{"2001:db8::80/121", 121, 6},
	} {
		p := toPrefix(tt.in)
		ps := p.Subnets(tt.n)
		if len(ps) != 1<<uint(tt.n) {
			t.Errorf("#%d: got %v; want %v", i, len(ps), 1<<uint(tt.n))
		}
		for _, p := range ps {
			if p.Len() != tt.l+tt.n {
				t.Errorf("#%d: got %v; want %v", i, p.Len(), tt.l+tt.n)
			}
		}
		if super := ipaddr.Supernet(ps); super == nil {
			for _, p := range ps {
				t.Logf("subnet: %v", p)
			}
			t.Errorf("#%d: got %v; want %v", i, super, p)
		}
	}
}

func TestPrefixTextMarshalerUnmarshaler(t *testing.T) {
	for i, tt := range []struct {
		in, tmp string
		out     []byte
	}{
		{"192.0.2.0/24", "0.0.0.0/0", []byte("192.0.2.0/24")},

		{"2001:db8::cafe/127", "::/0", []byte("2001:db8::cafe/127")},
	} {
		p1 := toPrefix(tt.in)
		out, err := p1.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(out, tt.out) {
			t.Errorf("#%d: got %v; want %v", i, out, tt.out)
		}
		p2 := toPrefix(tt.tmp)
		if err := p2.UnmarshalText(tt.out); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p2, p1) {
			t.Errorf("#%d: got %v; want %v", i, p2, p1)
		}
	}
}
