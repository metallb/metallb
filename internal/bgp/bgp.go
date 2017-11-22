package bgp

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"
)

type Session struct {
	asn      uint32
	routerID net.IP

	mu   sync.Mutex
	conn net.Conn
}

const marker = 0xffffffffffffffff

func (s *Session) Advertise(adv *Advertisement) error {
	if adv.Prefix.IP.To4() == nil {
		return fmt.Errorf("prefix must be IPv4, got %q", adv.Prefix)
	}
	if adv.NextHop.To4() == nil {
		return fmt.Errorf("next-hop must be IPv4, got %q", adv.NextHop)
	}
	if len(adv.Communities) > 63 {
		return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
	}

	var (
		msgLen  uint16
		attrLen uint16 = 20
	)
	msg := []interface{}{
		// Header
		uint64(marker), uint64(marker), // Markers
		&msgLen,  // Total message length
		uint8(2), // type = UPDATE

		// UPDATE
		uint16(0), // Withdraw length
		&attrLen,  // Path attributes length

		// UPDATE.attrs["origin"]
		uint8(0x40), uint8(1), // mandatory, origin
		uint8(1), // len
		uint8(2), // incomplete

		// UPDATE.attrs["as-path"]
		uint8(0x40), uint8(2), // mandatory, as-path
		uint8(6),      // len
		uint8(1),      // AS_SET
		uint8(4),      // len
		uint32(s.asn), // Our ASN is the only AS in the path

		// UPDATE.attrs["next-hop"]
		uint8(0x40), uint8(3), // mandatory, next-hop
		uint8(4), // len
		binary.BigEndian.Uint32(adv.NextHop), // next hop IP

		// UPDATE.attrs["communities"]
		uint8(0xc0), uint8(8), // optional transitive, communities
	}
	if len(adv.Communities) > 0 {
		attrLen += uint16(2) + uint16(len(adv.Communities)*4)
		msg = append(msg, uint8(0xc0), uint8(8), uint8(len(adv.Communities)*4))
		for _, c := range adv.Communities {
			msg = append(msg, c)
		}
	}

	o, _ := adv.Prefix.Mask.Size()
	msg = append(msg, byte(o))
	bytes := o / 8
	if o%8 != 0 {
		bytes++
	}
	for _, b := range adv.Prefix.IP.To4()[:bytes] {
		msg = append(msg, b)
	}
	msgLen = uint16(binary.Size(msg))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := binary.Write(s.conn, binary.BigEndian, msg); err != nil {
		return err
	}
	return nil
}

func (s *Session) Withdraw(prefix *net.IPNet) error {
	if prefix.IP.To4() == nil {
		return fmt.Errorf("prefix must be IPv4, got %q", prefix)
	}

	var (
		msgLen uint16
		wdrLen uint16
	)
	msg := []interface{}{
		// Header
		uint64(marker), uint64(marker), // Markers
		&msgLen,  // Total message length
		uint8(2), // type = UPDATE

		// UPDATE
		&wdrLen,   // Withdraw length
		uint16(0), // Path attributes length
	}

	o, _ := prefix.Mask.Size()
	msg = append(msg, byte(o))
	bytes := o / 8
	if o%8 != 0 {
		bytes++
	}
	for _, b := range prefix.IP.To4()[:bytes] {
		msg = append(msg, b)
	}
	msgLen = uint16(binary.Size(msg))

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := binary.Write(s.conn, binary.BigEndian, msg); err != nil {
		return err
	}
	return nil
}

type Advertisement struct {
	Prefix      *net.IPNet
	NextHop     net.IP
	Communities []uint32
}

func New(addr string, asn uint32, routerID net.IP, peerASN uint32) (*Session, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if err := sendOpen(conn, asn, routerID); err != nil {
		conn.Close()
		return nil, err
	}

	if err := readOpen(conn, peerASN); err != nil {
		conn.Close()
		return nil, err
	}

	go consumeBGP(conn)

	return &Session{
		asn:  asn,
		conn: conn,
	}, nil
}

func consumeBGP(conn net.Conn) {
	defer conn.Close()
	for {
		hdr := struct {
			Marker1, Marker2 uint64
			Len              uint16
			Type             uint8
		}{}
		if err := binary.Read(conn, binary.BigEndian, &hdr); err != nil {
			// TODO: log, or propagate the error somehow.
			return
		}
		if hdr.Marker1 != 0xffffffffffffffff || hdr.Marker2 != 0xffffffffffffffff {
			// TODO: propagate
			return
		}
		if _, err := io.Copy(ioutil.Discard, io.LimitReader(conn, int64(hdr.Len)-19)); err != nil {
			// TODO: propagate
			return
		}
	}
}

func readOpen(r io.Reader, asn uint32) error {
	hdr := struct {
		// Header
		Marker1, Marker2 uint64
		Len              uint16
		Type             uint8
	}{}
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return err
	}
	if hdr.Marker1 != 0xffffffffffffffff || hdr.Marker2 != 0xffffffffffffffff {
		return fmt.Errorf("synchronization error, incorrect header marker")
	}
	if hdr.Type != 1 {
		return fmt.Errorf("message type is not OPEN, got %d, want 1", hdr.Type)
	}
	if hdr.Len < 37 {
		return fmt.Errorf("message length %d too small to be OPEN", hdr.Len)
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
	if err := binary.Read(r, binary.BigEndian, &open); err != nil {
		return err
	}
	if open.Version != 4 {
		return fmt.Errorf("wrong BGP version")
	}
	if open.OptType != 2 {
		return fmt.Errorf("unknown option %d", open.OptType)
	}

	if int64(open.OptLen) != lr.N {
		return fmt.Errorf("%d trailing garbage bytes after capabilities", lr.N)
	}
	for {
		cap := struct {
			Code uint8
			Len  uint8
		}{}
		if err := binary.Read(lr, binary.BigEndian, &cap); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if cap.Code != 65 {
			// TODO: only ignore capabilities that we know are fine to
			// ignore.
			if _, err := io.Copy(ioutil.Discard, io.LimitReader(lr, int64(cap.Len))); err != nil {
				return err
			}
			continue
		}
		var peerASN uint32
		if err := binary.Read(lr, binary.BigEndian, &peerASN); err != nil {
			return err
		}
		if peerASN != asn {
			return fmt.Errorf("unexpected peer ASN %d, want %d", peerASN, asn)
		}
	}
}

func sendOpen(w io.Writer, asn uint32, routerID net.IP) error {
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
		HoldTime: 0,           // Disable hold-time
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
	copy(msg.RouterID[:], routerID)

	return binary.Write(w, binary.BigEndian, msg)
}
