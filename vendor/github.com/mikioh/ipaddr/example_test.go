// Copyright 2015 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"fmt"
	"log"
	"net"

	"github.com/mikioh/ipaddr"
)

func ExampleCursor_traversal() {
	c, err := ipaddr.Parse("2001:db8::/126,192.0.2.128/30,198.51.100.0/29")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(c.Pos(), c.First(), c.Last(), c.List())
	for pos := c.Next(); pos != nil; pos = c.Next() {
		fmt.Println(pos)
	}
	fmt.Println(c.Pos(), c.First(), c.Last(), c.List())
	for pos := c.Prev(); pos != nil; pos = c.Prev() {
		fmt.Println(pos)
	}
	fmt.Println(c.Pos(), c.First(), c.Last(), c.List())
	// Output:
	// &{192.0.2.128 192.0.2.128/30} &{192.0.2.128 192.0.2.128/30} &{2001:db8::3 2001:db8::/126} [192.0.2.128/30 198.51.100.0/29 2001:db8::/126]
	// &{192.0.2.129 192.0.2.128/30}
	// &{192.0.2.130 192.0.2.128/30}
	// &{192.0.2.131 192.0.2.128/30}
	// &{198.51.100.0 198.51.100.0/29}
	// &{198.51.100.1 198.51.100.0/29}
	// &{198.51.100.2 198.51.100.0/29}
	// &{198.51.100.3 198.51.100.0/29}
	// &{198.51.100.4 198.51.100.0/29}
	// &{198.51.100.5 198.51.100.0/29}
	// &{198.51.100.6 198.51.100.0/29}
	// &{198.51.100.7 198.51.100.0/29}
	// &{2001:db8:: 2001:db8::/126}
	// &{2001:db8::1 2001:db8::/126}
	// &{2001:db8::2 2001:db8::/126}
	// &{2001:db8::3 2001:db8::/126}
	// &{2001:db8::3 2001:db8::/126} &{192.0.2.128 192.0.2.128/30} &{2001:db8::3 2001:db8::/126} [192.0.2.128/30 198.51.100.0/29 2001:db8::/126]
	// &{2001:db8::2 2001:db8::/126}
	// &{2001:db8::1 2001:db8::/126}
	// &{2001:db8:: 2001:db8::/126}
	// &{198.51.100.7 198.51.100.0/29}
	// &{198.51.100.6 198.51.100.0/29}
	// &{198.51.100.5 198.51.100.0/29}
	// &{198.51.100.4 198.51.100.0/29}
	// &{198.51.100.3 198.51.100.0/29}
	// &{198.51.100.2 198.51.100.0/29}
	// &{198.51.100.1 198.51.100.0/29}
	// &{198.51.100.0 198.51.100.0/29}
	// &{192.0.2.131 192.0.2.128/30}
	// &{192.0.2.130 192.0.2.128/30}
	// &{192.0.2.129 192.0.2.128/30}
	// &{192.0.2.128 192.0.2.128/30}
	// &{192.0.2.128 192.0.2.128/30} &{192.0.2.128 192.0.2.128/30} &{2001:db8::3 2001:db8::/126} [192.0.2.128/30 198.51.100.0/29 2001:db8::/126]
}

func ExamplePrefix_subnettingAndSupernetting() {
	_, n, err := net.ParseCIDR("203.0.113.0/24")
	if err != nil {
		log.Fatal(err)
	}
	p := ipaddr.NewPrefix(n)
	fmt.Println(p.IP, p.Last(), p.Len(), p.Mask, p.Hostmask())
	fmt.Println()
	ps := p.Subnets(3)
	for _, p := range ps {
		fmt.Println(p)
	}
	fmt.Println()
	fmt.Println(ipaddr.Supernet(ps))
	fmt.Println(ipaddr.Supernet(ps[1:7]))
	fmt.Println(ipaddr.Supernet(ps[:4]))
	fmt.Println(ipaddr.Supernet(ps[4:8]))
	// Output:
	// 203.0.113.0 203.0.113.255 24 ffffff00 000000ff
	//
	// 203.0.113.0/27
	// 203.0.113.32/27
	// 203.0.113.64/27
	// 203.0.113.96/27
	// 203.0.113.128/27
	// 203.0.113.160/27
	// 203.0.113.192/27
	// 203.0.113.224/27
	//
	// 203.0.113.0/24
	// 203.0.113.0/24
	// 203.0.113.0/25
	// 203.0.113.128/25
}

func ExamplePrefix_subnettingAndAggregation() {
	_, n, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		log.Fatal(err)
	}
	p := ipaddr.NewPrefix(n)
	fmt.Println(p.IP, p.Last(), p.Len(), p.Mask, p.Hostmask())
	fmt.Println()
	ps := p.Subnets(3)
	for _, p := range ps {
		fmt.Println(p)
	}
	fmt.Println()
	fmt.Println(ipaddr.Aggregate(ps))
	fmt.Println(ipaddr.Aggregate(ps[1:7]))
	fmt.Println(ipaddr.Aggregate(ps[:4]))
	fmt.Println(ipaddr.Aggregate(ps[4:8]))
	// Output:
	// 192.0.2.0 192.0.2.255 24 ffffff00 000000ff
	//
	// 192.0.2.0/27
	// 192.0.2.32/27
	// 192.0.2.64/27
	// 192.0.2.96/27
	// 192.0.2.128/27
	// 192.0.2.160/27
	// 192.0.2.192/27
	// 192.0.2.224/27
	//
	// [192.0.2.0/24]
	// [192.0.2.32/27 192.0.2.64/26 192.0.2.128/26 192.0.2.192/27]
	// [192.0.2.0/25]
	// [192.0.2.128/25]
}

func ExamplePrefix_addressRangeSummarization() {
	ps := ipaddr.Summarize(net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::8000"))
	for _, p := range ps {
		fmt.Println(p)
	}
	// Output:
	// 2001:db8::1/128
	// 2001:db8::2/127
	// 2001:db8::4/126
	// 2001:db8::8/125
	// 2001:db8::10/124
	// 2001:db8::20/123
	// 2001:db8::40/122
	// 2001:db8::80/121
	// 2001:db8::100/120
	// 2001:db8::200/119
	// 2001:db8::400/118
	// 2001:db8::800/117
	// 2001:db8::1000/116
	// 2001:db8::2000/115
	// 2001:db8::4000/114
	// 2001:db8::8000/128
}
