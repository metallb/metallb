// Copyright 2017 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

// +build go1.9

package ipaddr

import "math/bits"

func leadingZeros32(bs uint32) int {
	return int(bits.LeadingZeros32(bs))
}

func leadingZeros64(bs uint64) int {
	return int(bits.LeadingZeros64(bs))
}
