package bgp

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

type mhIpv6 int

func (mh mhIpv6) sendUpdate(w io.Writer, asn uint32, ibgp bool, defaultNextHop net.IP, adv *Advertisement) error {
	var b bytes.Buffer

	hdr := struct {
		M1, M2  uint64
		Len     uint16
		Type    uint8
		WdrLen  uint16
		AttrLen uint16
	}{
		M1:   uint64(0xffffffffffffffff),
		M2:   uint64(0xffffffffffffffff),
		Type: 2,
	}
	if err := binary.Write(&b, binary.BigEndian, hdr); err != nil {
		return err
	}

	l := b.Len()

	b.Write([]byte{
		0x40, 1, // mandatory, origin
		1, // len
		2, // incomplete

		0x40, 2, // mandatory, as-path
	})
	if ibgp {
		b.WriteByte(0) // empty AS path
	} else {
		b.Write([]byte{
			6, // len
			2, // AS_SEQUENCE
			1, // len (in number of ASes)
		})
		if err := binary.Write(&b, binary.BigEndian, asn); err != nil {
			return err
		}
	}

	o := b.Len() // Save the offset so we can set the length later
	b.Write([]byte{
		0x80, 14, // optional, MP_REACH_NLRI
		0,    // len (filled later)
		0, 2, // AFI IPv6
		1,  // SAFI Unicast
		16, // length of nexthop
	})

	if adv.NextHop != nil {
		b.Write(adv.NextHop)
	} else {
		b.Write(defaultNextHop)
	}

	b.WriteByte(0)   // SNPA
	b.WriteByte(128) // The advertised address always /128
	b.Write(adv.Prefix.IP.To16())

	b.Bytes()[o+2] = byte(b.Len() - o - 3)
	binary.BigEndian.PutUint16(b.Bytes()[21:23], uint16(b.Len()-l))
	binary.BigEndian.PutUint16(b.Bytes()[16:18], uint16(b.Len()))

	if _, err := io.Copy(w, &b); err != nil {
		return err
	}
	return nil
}

func (mh mhIpv6) sendWithdraw(w io.Writer, prefixes []*net.IPNet) error {
	var b bytes.Buffer

	hdr := struct {
		M1, M2  uint64
		Len     uint16
		Type    uint8
		WdrLen  uint16
		AttrLen uint16
	}{
		M1:   uint64(0xffffffffffffffff),
		M2:   uint64(0xffffffffffffffff),
		Type: 2,
	}
	if err := binary.Write(&b, binary.BigEndian, hdr); err != nil {
		return err
	}

	l := b.Len()

	b.Write([]byte{
		0x40, 1, // mandatory, origin
		1, // len
		2, // incomplete
	})

	o := b.Len() // Save the offset so we can set the length later
	b.Write([]byte{
		0x80, 15, // optional, MP_UNREACH_NLRI
		0,    // len (filled later)
		0, 2, // AFI IPv6
		1, // SAFI Unicast
	})

	for _, p := range prefixes {
		b.WriteByte(128)
		b.Write(p.IP.To16())
	}

	b.Bytes()[o+2] = byte(b.Len() - o - 3)
	binary.BigEndian.PutUint16(b.Bytes()[21:23], uint16(b.Len()-l))
	binary.BigEndian.PutUint16(b.Bytes()[16:18], uint16(b.Len()))

	if _, err := io.Copy(w, &b); err != nil {
		return err
	}
	return nil
}
