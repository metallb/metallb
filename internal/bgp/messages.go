package bgp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"time"
)

func sendOpen(w io.Writer, asn uint32, routerID net.IP, holdTime time.Duration) error {
	if routerID.To4() == nil {
		panic("ipv4 address used as RouterID")
	}

	msg := struct {
		// Header
		Marker1, Marker2 uint64
		Len              uint16
		Type             uint8

		// OPEN
		Version  uint8
		ASN16    uint16
		HoldTime uint16
		RouterID [4]byte

		// Options (we only send one, capabilities)
		OptsLen uint8
		OptType uint8
		OptLen  uint8

		// Capabilities (we send only one, 4-byte ASN)
		CapType uint8
		CapLen  uint8
		ASN32   uint32
	}{
		Marker1: 0xffffffffffffffff,
		Marker2: 0xffffffffffffffff,
		Len:     0, // Filled below
		Type:    1, // OPEN

		Version:  4,
		ASN16:    uint16(asn), // Possibly tweaked below
		HoldTime: uint16(holdTime.Seconds()),
		// RouterID filled below

		OptsLen: 8,
		OptType: 2, // Capabilities
		OptLen:  6,

		CapType: 65, // 4-byte ASN
		CapLen:  4,
		ASN32:   asn,
	}
	msg.Len = uint16(binary.Size(msg))
	if asn > 65535 {
		msg.ASN16 = 23456
	}
	copy(msg.RouterID[:], routerID.To4())

	return binary.Write(w, binary.BigEndian, msg)
}

func readOpen(r io.Reader) (uint32, time.Duration, error) {
	hdr := struct {
		// Header
		Marker1, Marker2 uint64
		Len              uint16
		Type             uint8
	}{}
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return 0, 0, err
	}
	if hdr.Marker1 != 0xffffffffffffffff || hdr.Marker2 != 0xffffffffffffffff {
		return 0, 0, fmt.Errorf("synchronization error, incorrect header marker")
	}
	if hdr.Type != 1 {
		return 0, 0, fmt.Errorf("message type is not OPEN, got %d, want 1", hdr.Type)
	}
	if hdr.Len < 37 {
		return 0, 0, fmt.Errorf("message length %d too small to be OPEN", hdr.Len)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(hdr.Len) - 19,
	}
	open := struct {
		Version  uint8
		ASN16    uint16
		HoldTime uint16
		RouterID uint32
		OptsLen  uint8
		OptType  uint8
		OptLen   uint8
	}{}
	if err := binary.Read(lr, binary.BigEndian, &open); err != nil {
		return 0, 0, err
	}
	if open.Version != 4 {
		return 0, 0, fmt.Errorf("wrong BGP version")
	}
	if open.HoldTime != 0 && open.HoldTime < 3 {
		return 0, 0, fmt.Errorf("invalid hold time %q, must be 0 or >=3s", open.HoldTime)
	}
	if open.OptType != 2 {
		return 0, 0, fmt.Errorf("unknown option %d", open.OptType)
	}

	asn := uint32(open.ASN16)

	if int64(open.OptLen) != lr.N {
		return 0, 0, fmt.Errorf("%d trailing garbage bytes after capabilities", lr.N)
	}
	for {
		cap := struct {
			Code uint8
			Len  uint8
		}{}
		if err := binary.Read(lr, binary.BigEndian, &cap); err != nil {
			if err == io.EOF {
				return asn, time.Duration(open.HoldTime) * time.Second, nil
			}
			return 0, 0, err
		}
		if cap.Code != 65 {
			// TODO: only ignore capabilities that we know are fine to
			// ignore.
			if _, err := io.Copy(ioutil.Discard, io.LimitReader(lr, int64(cap.Len))); err != nil {
				return 0, 0, err
			}
			continue
		}
		if err := binary.Read(lr, binary.BigEndian, &asn); err != nil {
			return 0, 0, err
		}
	}
}

func sendUpdate(w io.Writer, asn uint32, adv *Advertisement) error {
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
	if err := encodePathAttrs(&b, asn, adv); err != nil {
		return err
	}
	binary.BigEndian.PutUint16(b.Bytes()[21:23], uint16(b.Len()-l))
	encodePrefixes(&b, []*net.IPNet{adv.Prefix})
	binary.BigEndian.PutUint16(b.Bytes()[16:18], uint16(b.Len()))

	if _, err := io.Copy(w, &b); err != nil {
		return err
	}
	return nil
}

func encodePrefixes(b *bytes.Buffer, pfxs []*net.IPNet) {
	for _, pfx := range pfxs {
		o, _ := pfx.Mask.Size()
		b.WriteByte(byte(o))
		b.Write(pfx.IP.To4()[:bytesForBits(o)])
	}
}

func bytesForBits(n int) int {
	// Evil bit hack that rounds n up to the next multiple of 8, then
	// divides by 8. This returns the minimum number of whole bytes
	// required to contain n bits.
	return ((n + 7) &^ 7) / 8
}

func encodePathAttrs(b *bytes.Buffer, asn uint32, adv *Advertisement) error {
	b.Write([]byte{
		0x40, 1, // mandatory, origin
		1, // len
		2, // incomplete

		0x40, 2, // mandatory, as-path
	})
	if asn == 0 {
		b.WriteByte(0) // empty AS path
	} else {
		b.Write([]byte{
			6, // len
			1, // AS_SET
			1, // len (in number of ASes)
		})
		if err := binary.Write(b, binary.BigEndian, asn); err != nil {
			return err
		}
	}
	b.Write([]byte{
		0x40, 3, // mandatory, next-hop
		4, // len
	})
	b.Write(adv.NextHop.To4())
	b.Write([]byte{
		0x40, 5, // well-known, localpref
		4, // len
	})
	if err := binary.Write(b, binary.BigEndian, adv.LocalPref); err != nil {
		return err
	}

	if len(adv.Communities) > 0 {
		b.Write([]byte{
			0xc0, 8, // optional transitive, communities
		})
		if err := binary.Write(b, binary.BigEndian, uint8(len(adv.Communities)*4)); err != nil {
			return err
		}
		for _, c := range adv.Communities {
			if err := binary.Write(b, binary.BigEndian, c); err != nil {
				return err
			}
		}
	}

	return nil
}

func sendWithdraw(w io.Writer, prefixes []*net.IPNet) error {
	var b bytes.Buffer

	hdr := struct {
		M1, M2 uint64
		Len    uint16
		Type   uint8
		WdrLen uint16
	}{
		M1:   uint64(0xffffffffffffffff),
		M2:   uint64(0xffffffffffffffff),
		Type: 2,
	}
	if err := binary.Write(&b, binary.BigEndian, hdr); err != nil {
		return err
	}
	l := b.Len()
	encodePrefixes(&b, prefixes)
	binary.BigEndian.PutUint16(b.Bytes()[19:21], uint16(b.Len()-l))
	if err := binary.Write(&b, binary.BigEndian, uint16(0)); err != nil {
		return err
	}
	binary.BigEndian.PutUint16(b.Bytes()[16:18], uint16(b.Len()))

	if _, err := io.Copy(w, &b); err != nil {
		return err
	}
	return nil
}

func sendKeepalive(w io.Writer) error {
	msg := struct {
		Marker1, Marker2 uint64
		Len              uint16
		Type             uint8
	}{
		Marker1: 0xffffffffffffffff,
		Marker2: 0xffffffffffffffff,
		Len:     19,
		Type:    4,
	}
	return binary.Write(w, binary.BigEndian, msg)
}
