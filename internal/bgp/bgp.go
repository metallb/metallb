package bgp

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	backoff = 2 * time.Second
)

var errClosed = errors.New("session closed")

type Session struct {
	asn      uint32
	routerID net.IP
	addr     string
	peerASN  uint32
	holdTime time.Duration

	mu         sync.Mutex
	cond       *sync.Cond
	closed     bool
	conn       net.Conn
	advertised map[string]*Advertisement
	changed    map[string]*net.IPNet
	pending    *list.List
}

func (s *Session) run() {
	for {
		if err := s.connect(); err != nil {
			glog.Error(err)
			time.Sleep(backoff)
			continue
		}

		glog.Infof("BGP session to %q established", s.addr)

		if err := s.sendUpdates(); err != nil {
			if err == errClosed {
				return
			}
			glog.Error(err)
		}

		glog.Infof("BGP session to %q down", s.addr)
	}
}

func (s *Session) sendUpdates() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for c, adv := range s.advertised {
		if err := sendUpdate(s.conn, s.asn, adv); err != nil {
			s.abort()
			return fmt.Errorf("sending update of %q to %q: %s", c, s.addr, err)
		}
	}
	s.changed = map[string]*net.IPNet{}

	for {
		for len(s.changed) == 0 && s.conn != nil {
			s.cond.Wait()
		}

		if s.closed {
			return errClosed
		}
		if s.conn == nil {
			return nil
		}

		wdr := []*net.IPNet{}
		for c, pfx := range s.changed {
			adv := s.advertised[c]
			if adv == nil {
				wdr = append(wdr, pfx)
			}
			if err := sendUpdate(s.conn, s.asn, adv); err != nil {
				s.abort()
				return fmt.Errorf("sending update of %q to %q: %s", c, s.addr, err)
			}
		}

		if len(wdr) != 0 {
			if err := sendWithdraw(s.conn, wdr); err != nil {
				s.abort()
				return fmt.Errorf("sending withdraw of %q to %q: %s", wdr, s.addr, err)
			}
		}
		s.changed = map[string]*net.IPNet{}
	}
}

func (s *Session) connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("dial %q: %s", s.addr, err)
	}

	if err := sendOpen(conn, s.asn, s.routerID, s.holdTime); err != nil {
		conn.Close()
		return fmt.Errorf("send OPEN to %q: %s", s.addr, err)
	}

	requestedHold, err := readOpen(conn, s.peerASN)
	if err != nil {
		conn.Close()
		return fmt.Errorf("read OPEN from %q: %s", s.addr, err)
	}
	hold := s.holdTime
	if requestedHold < hold {
		hold = requestedHold
	}

	// Consume BGP messages until the connection closes.
	go consumeBGP(conn)

	// Send one keepalive to say that yes, we accept the OPEN.
	if err := sendKeepalive(conn); err != nil {
		conn.Close()
		return fmt.Errorf("accepting peer OPEN from %q: %s", s.addr, err)
	}

	s.conn = conn
	return nil
}

func (s *Session) sendKeepalives() {
	for {
		time.Sleep(s.holdTime / 3)
		if err := s.sendKeepalive(); err != nil {
			if err == errClosed {
				// Session has been closed by package caller, we're
				// done here.
				return
			}
			glog.Error(err)
		}
	}
}

func (s *Session) sendKeepalive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosed
	}
	if s.conn == nil {
		// No connection established, othing to do.
		return nil
	}
	if err := sendKeepalive(s.conn); err != nil {
		s.abort()
		return fmt.Errorf("sending keepalive to %q: %s", s.addr, err)
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

func sendWithdraw(w io.Writer, prefixes []*net.IPNet) error {
	var (
		msgLen uint16
		wdrLen uint16
	)
	msg := []interface{}{
		// Header
		uint64(0xffffffffffffffff), uint64(0xffffffffffffffff), // Markers
		&msgLen,  // Total message length
		uint8(2), // type = UPDATE

		// UPDATE
		&wdrLen, // Withdraw length
	}

	for _, pfx := range prefixes {
		o, _ := pfx.Mask.Size()
		msg = append(msg, byte(o))
		bytes := o / 8
		if o%8 != 0 {
			bytes++
		}
		for _, b := range pfx.IP.To4()[:bytes] {
			msg = append(msg, b)
		}
		wdrLen += uint16(bytes + 1)
	}
	msgLen = uint16(binary.Size(msg))

	return binary.Write(w, binary.BigEndian, msg)
}

func encodePathAttrs(b *bytes.Buffer, asn uint32, adv *Advertisement) error {
	b.Write([]byte{
		0x40, 1, // mandatory, origin
		1, // len
		2, // incomplete

		0x40, 2, // mandatory, as-path
		6, // len
		1, // AS_SET
		1, // len (in number of ASes)
	})
	if err := binary.Write(b, binary.BigEndian, asn); err != nil {
		return err
	}
	b.Write([]byte{
		0x40, 3, // mandatory, next-hop
		4, // len
	})
	b.Write(adv.NextHop.To4())

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

func bytesForBits(n int) int {
	// Evil bit hack that rounds n up to the next multiple of 8, then
	// divides by 8. This returns the minimum number of whole bytes
	// required to contain n bits.
	return ((n + 7) &^ 7) / 8
}

func encodePrefixes(b *bytes.Buffer, pfxs []*net.IPNet) {
	for _, pfx := range pfxs {
		o, _ := pfx.Mask.Size()
		b.WriteByte(byte(o))
		b.Write(pfx.IP.To4()[:bytesForBits(o)])
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

func New(addr string, asn uint32, routerID net.IP, peerASN uint32, holdTime time.Duration) (*Session, error) {
	ret := &Session{
		addr:         addr,
		asn:          asn,
		routerID:     routerID.To4(),
		peerASN:      peerASN,
		holdTime:     holdTime,
		advertised:   map[string]*Advertisement{},
		changed:      map[string]*net.IPNet{},
		newKeepalive: make(chan (<-chan time.Time)),
	}
	if ret.routerID == nil {
		return nil, fmt.Errorf("invalid routerID %q, must be IPv4", routerID)
	}
	ret.cond = sync.NewCond(&ret.mu)
	go ret.sendKeepalives()
	go ret.run()

	return ret, nil
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

func readOpen(r io.Reader, asn uint32) (time.Duration, error) {
	hdr := struct {
		// Header
		Marker1, Marker2 uint64
		Len              uint16
		Type             uint8
	}{}
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return 0, err
	}
	if hdr.Marker1 != 0xffffffffffffffff || hdr.Marker2 != 0xffffffffffffffff {
		return 0, fmt.Errorf("synchronization error, incorrect header marker")
	}
	if hdr.Type != 1 {
		return 0, fmt.Errorf("message type is not OPEN, got %d, want 1", hdr.Type)
	}
	if hdr.Len < 37 {
		return 0, fmt.Errorf("message length %d too small to be OPEN", hdr.Len)
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
		return 0, err
	}
	if open.Version != 4 {
		return 0, fmt.Errorf("wrong BGP version")
	}
	if open.HoldTime != 0 && open.HoldTime < 3 {
		return 0, fmt.Errorf("invalid hold time %q, must be 0 or >=3s", open.HoldTime)
	}
	if open.OptType != 2 {
		return 0, fmt.Errorf("unknown option %d", open.OptType)
	}

	if int64(open.OptLen) != lr.N {
		return 0, fmt.Errorf("%d trailing garbage bytes after capabilities", lr.N)
	}
	for {
		cap := struct {
			Code uint8
			Len  uint8
		}{}
		if err := binary.Read(lr, binary.BigEndian, &cap); err != nil {
			if err == io.EOF {
				return time.Duration(open.HoldTime) * time.Second, nil
			}
			return 0, err
		}
		if cap.Code != 65 {
			// TODO: only ignore capabilities that we know are fine to
			// ignore.
			if _, err := io.Copy(ioutil.Discard, io.LimitReader(lr, int64(cap.Len))); err != nil {
				return 0, err
			}
			continue
		}
		var peerASN uint32
		if err := binary.Read(lr, binary.BigEndian, &peerASN); err != nil {
			return 0, err
		}
		if peerASN != asn {
			return 0, fmt.Errorf("unexpected peer ASN %d, want %d", peerASN, asn)
		}
	}
}

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

func (s *Session) Advertise(advs ...*Advertisement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, adv := range advs {
		if adv.Prefix.IP.To4() == nil {
			return fmt.Errorf("cannot advertise non-v4 prefix %q", adv.Prefix)
		}

		if adv.NextHop.To4() == nil {
			return fmt.Errorf("next-hop must be IPv4, got %q", adv.NextHop)
		}
		if len(adv.Communities) > 63 {
			return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
		}
	}
	for _, adv := range advs {
		if a, ok := s.advertised[adv.Prefix.String()]; ok {
			if a.NextHop.Equal(adv.NextHop) && !reflect.DeepEqual(a.Communities, adv.Communities) {
				// No need to advertise this, peer already knows the
				// correct state.
				continue
			}
		} else {
			s.advertised[adv.Prefix.String()] = adv
		}
		s.changed[adv.Prefix.String()] = adv.Prefix
	}

	if len(s.changed) > 0 {
		// Wake up the syncer routine to actually send updates.
		s.cond.Broadcast()
	}
	return nil
}

func (s *Session) Withdraw(prefixes []*net.IPNet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pfx := range prefixes {
		if pfx.IP.To4() == nil {
			return fmt.Errorf("prefix must be IPv4, got %q", pfx)
		}
		if s.advertised[pfx.String()] == nil {
			return fmt.Errorf("cannot withdraw %q, not advertised", pfx)
		}
	}

	for _, pfx := range prefixes {
		delete(s.advertised, pfx.String())
		s.changed[pfx.String()] = pfx
	}

	if len(s.changed) > 0 {
		// Wake up syncer routine to send updates
		s.cond.Broadcast()
	}
	return nil
}

func (s *Session) abort() {
	s.conn.Close()
	s.conn = nil
	s.cond.Broadcast()
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.abort()
	return nil
}

type Advertisement struct {
	Prefix      *net.IPNet
	NextHop     net.IP
	Communities []uint32
}
