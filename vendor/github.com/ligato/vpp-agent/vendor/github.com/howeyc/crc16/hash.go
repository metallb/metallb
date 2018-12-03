// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crc16

import (
	"errors"
	"hash"
)

// Hash16 is the common interface implemented by all 16-bit hash functions.
type Hash16 interface {
	hash.Hash
	Sum16() uint16
}

// New creates a new Hash16 computing the CRC-16 checksum
// using the polynomial represented by the Table.
func New(tab *Table) Hash16 { return &digest{0, tab} }

// NewIBM creates a new Hash16 computing the CRC-16 checksum
// using the IBM polynomial.
func NewIBM() Hash16 { return New(IBMTable) }

// NewCCITT creates a new hash.Hash16 computing the CRC-16 checksum
// using the CCITT polynomial.
func NewCCITT() Hash16 { return New(CCITTTable) }

// NewSCSI creates a new Hash16 computing the CRC-16 checksum
// using the SCSI polynomial.
func NewSCSI() Hash16 { return New(SCSITable) }

// digest represents the partial evaluation of a checksum.
type digest struct {
	crc uint16
	tab *Table
}

func (d *digest) Size() int { return 2 }

func (d *digest) BlockSize() int { return 1 }

func (d *digest) Reset() { d.crc = 0 }

func (d *digest) Write(p []byte) (n int, err error) {
	d.crc = Update(d.crc, d.tab, p)
	return len(p), nil
}

func (d *digest) Sum16() uint16 { return d.crc }

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum16()
	return append(in, byte(s>>8), byte(s))
}

const (
	magic         = "crc16\x01"
	marshaledSize = len(magic) + 2 + 2 + 1
)

func (d *digest) MarshalBinary() ([]byte, error) {
	b := make([]byte, 0, marshaledSize)
	b = append(b, magic...)
	b = appendUint16(b, tableSum(d.tab))
	b = appendUint16(b, d.crc)
	if d.tab.reversed {
		b = append(b, byte(0x01))
	} else {
		b = append(b, byte(0x00))
	}
	return b, nil
}

func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return errors.New("hash/crc16: invalid hash state identifier")
	}
	if len(b) != marshaledSize {
		return errors.New("hash/crc16: invalid hash state size")
	}
	if tableSum(d.tab) != readUint16(b[6:]) {
		return errors.New("hash/crc16: tables do not match")
	}
	d.crc = readUint16(b[8:])
	if b[10] == 0x01 {
		d.tab.reversed = true
	}
	return nil
}

func appendUint16(b []byte, x uint16) []byte {
	a := [2]byte{
		byte(x >> 8),
		byte(x),
	}
	return append(b, a[:]...)
}

func readUint16(b []byte) uint16 {
	_ = b[1]
	return uint16(b[1]) | uint16(b[0])<<8
}

// tableSum returns the IBM checksum of table t.
func tableSum(t *Table) uint16 {
	var a [1024]byte
	b := a[:0]
	if t != nil {
		for _, x := range t.entries {
			b = appendUint16(b, x)
		}
	}
	return ChecksumIBM(b)
}
