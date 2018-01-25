// Copyright 2013 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

package ipaddr_test

import (
	"container/heap"
	"net"
	"testing"

	"github.com/mikioh/ipaddr"
)

type prefixHeap []ipaddr.Prefix

func (h *prefixHeap) Less(i, j int) bool   { return ipaddr.Compare(&(*h)[i], &(*h)[j]) < 0 }
func (h *prefixHeap) Swap(i, j int)        { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *prefixHeap) Len() int             { return len(*h) }
func (h *prefixHeap) Pop() (v interface{}) { *h, v = (*h)[:h.Len()-1], (*h)[h.Len()-1]; return }
func (h *prefixHeap) Push(v interface{})   { *h = append(*h, v.(ipaddr.Prefix)) }

func TestPrefixHeap(t *testing.T) {
	_, n, err := net.ParseCIDR("2001:db8:f001::/48")
	if err != nil {
		t.Fatal(err)
	}
	super := ipaddr.NewPrefix(n)
	h := new(prefixHeap)
	heap.Init(h)
	for _, p := range super.Subnets(6) {
		heap.Push(h, p)
	}
	for h.Len() > 0 {
		heap.Pop(h)
	}
}
