// Copyright 2017 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

// +build go1.8

package ipaddr

import (
	"bytes"
	"sort"
)

func sortByAscending(ps []Prefix) {
	sort.Slice(ps, func(i, j int) bool {
		if n := bytes.Compare(ps[i].IP, ps[j].IP); n != 0 {
			return n < 0
		}
		if n := bytes.Compare(ps[i].Mask, ps[j].Mask); n != 0 {
			return n < 0
		}
		return false
	})
}

func sortByDescending(ps []Prefix) {
	sort.Slice(ps, func(i, j int) bool {
		if n := bytes.Compare(ps[i].Mask, ps[j].Mask); n != 0 {
			return n >= 0
		}
		if n := bytes.Compare(ps[i].IP, ps[i].IP); n != 0 {
			return n >= 0
		}
		return false
	})
}
