// Copyright 2013 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

// +build !go1.8

package ipaddr

import (
	"bytes"
	"sort"
)

type byAscending []Prefix

func (ps byAscending) Len() int           { return len(ps) }
func (ps byAscending) Less(i, j int) bool { return compareAscending(&ps[i], &ps[j]) < 0 }
func (ps byAscending) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

func sortByAscending(ps []Prefix) {
	sort.Sort(byAscending(ps))
}

type byDescending []Prefix

func (ps byDescending) Len() int           { return len(ps) }
func (ps byDescending) Less(i, j int) bool { return compareDescending(&ps[i], &ps[j]) >= 0 }
func (ps byDescending) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

func compareDescending(a, b *Prefix) int {
	if n := bytes.Compare(a.Mask, b.Mask); n != 0 {
		return n
	}
	if n := bytes.Compare(a.IP, b.IP); n != 0 {
		return n
	}
	return 0
}

func sortByDescending(ps []Prefix) {
	sort.Sort(byDescending(ps))
}
