// Copyright 2013 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.

// +build !go1.9

package ipaddr

func leadingZeros32(bs uint32) int {
	bs |= bs >> 1
	bs |= bs >> 2
	bs |= bs >> 4
	bs |= bs >> 8
	bs |= bs >> 16
	return npop32(^bs)
}

func leadingZeros64(bs uint64) int {
	bs |= bs >> 1
	bs |= bs >> 2
	bs |= bs >> 4
	bs |= bs >> 8
	bs |= bs >> 16
	bs |= bs >> 32
	return npop64(^bs)
}

func npop32(bs uint32) int {
	bs = bs&0x55555555 + bs>>1&0x55555555
	bs = bs&0x33333333 + bs>>2&0x33333333
	bs = bs&0x0f0f0f0f + bs>>4&0x0f0f0f0f
	bs = bs&0x00ff00ff + bs>>8&0x00ff00ff
	return int(bs&0x0000ffff + bs>>16&0x0000ffff)
}

func npop64(bs uint64) int {
	bs = bs&0x5555555555555555 + bs>>1&0x5555555555555555
	bs = bs&0x3333333333333333 + bs>>2&0x3333333333333333
	bs = bs&0x0f0f0f0f0f0f0f0f + bs>>4&0x0f0f0f0f0f0f0f0f
	bs = bs&0x00ff00ff00ff00ff + bs>>8&0x00ff00ff00ff00ff
	bs = bs&0x0000ffff0000ffff + bs>>16&0x0000ffff0000ffff
	return int(bs&0x00000000ffffffff + bs>>32&0x00000000ffffffff)
}
