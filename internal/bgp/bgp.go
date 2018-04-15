package bgp // import "go.universe.tf/metallb/internal/bgp"

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
)

var errClosed = errors.New("session closed")

// Session represents one BGP session to an external router.
type Session struct {
	asn      uint32
	routerID net.IP // May be nil, meaning "derive from context"
	addr     string
	peerASN  uint32
	holdTime time.Duration
	logger   log.Logger

	newHoldTime chan bool
	backoff     backoff

	mu             sync.Mutex
	cond           *sync.Cond
	closed         bool
	conn           net.Conn
	actualHoldTime time.Duration
	defaultNextHop net.IP
	advertised     map[string]*Advertisement
	new            map[string]*Advertisement
}

// run tries to stay connected to the peer, and pumps route updates to it.
func (s *Session) run() {
	defer stats.DeleteSession(s.addr)
	for {
		if err := s.connect(); err != nil {
			if err == errClosed {
				return
			}
			s.logger.Log("op", "connect", "error", err, "msg", "failed to connect to peer")
			backoff := s.backoff.Duration()
			time.Sleep(backoff)
			continue
		}
		stats.SessionUp(s.addr)
		s.backoff.Reset()

		s.logger.Log("event", "sessionUp", "msg", "BGP session established")

		if !s.sendUpdates() {
			return
		}
		stats.SessionDown(s.addr)
		s.logger.Log("event", "sessionDown", "msg", "BGP session down")
	}
}

// sendUpdates waits for changes to desired advertisements, and pushes
// them out to the peer.
func (s *Session) sendUpdates() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	asn := s.asn
	if s.peerASN == s.asn {
		asn = 0
	}

	if s.new != nil {
		s.advertised, s.new = s.new, nil
	}

	for c, adv := range s.advertised {
		if err := sendUpdate(s.conn, asn, s.defaultNextHop, adv); err != nil {
			s.abort()
			s.logger.Log("op", "sendUpdate", "ip", c, "error", err, "msg", "failed to send BGP update")
			return true
		}
		stats.UpdateSent(s.addr)
	}
	stats.AdvertisedPrefixes(s.addr, len(s.advertised))

	for {
		for s.new == nil && s.conn != nil {
			s.cond.Wait()
		}

		if s.closed {
			return false
		}
		if s.conn == nil {
			return true
		}
		if s.new == nil {
			// nil is "no pending updates", contrast to a non-nil
			// empty map which means "withdraw all".
			continue
		}

		for c, adv := range s.new {
			if adv2, ok := s.advertised[c]; ok && adv.Equal(adv2) {
				// Peer already has correct state for this
				// advertisement, nothing to do.
				continue
			}

			if err := sendUpdate(s.conn, asn, s.defaultNextHop, adv); err != nil {
				s.abort()
				s.logger.Log("op", "sendUpdate", "prefix", c, "error", err, "msg", "failed to send BGP update")
				return true
			}
			stats.UpdateSent(s.addr)
		}

		wdr := []*net.IPNet{}
		for c, adv := range s.advertised {
			if s.new[c] == nil {
				wdr = append(wdr, adv.Prefix)
			}
		}
		if len(wdr) > 0 {
			if err := sendWithdraw(s.conn, wdr); err != nil {
				s.abort()
				for _, pfx := range wdr {
					s.logger.Log("op", "sendWithdraw", "prefix", pfx, "error", err, "msg", "failed to send BGP withdraw")
				}
				return true
			}
			stats.UpdateSent(s.addr)
		}
		s.advertised, s.new = s.new, nil
		stats.AdvertisedPrefixes(s.addr, len(s.advertised))
	}
}

// connect establishes the BGP session with the peer.
func (s *Session) connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("dial %q: %s", s.addr, err)
	}
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return fmt.Errorf("setting deadline on conn to %q: %s", s.addr, err)
	}

	addr, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		conn.Close()
		return fmt.Errorf("getting local addr for default nexthop to %q: %s", s.addr, err)
	}
	s.defaultNextHop = addr.IP

	routerID := s.routerID
	if routerID == nil {
		// Use the connection's source IP as the router ID
		routerID = s.defaultNextHop.To4()
		if routerID == nil {
			conn.Close()
			return fmt.Errorf("cannot automatically derive router ID for IPv6 connection to %q", s.addr)
		}
	}

	if err = sendOpen(conn, s.asn, routerID, s.holdTime); err != nil {
		conn.Close()
		return fmt.Errorf("send OPEN to %q: %s", s.addr, err)
	}

	op, err := readOpen(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("read OPEN from %q: %s", s.addr, err)
	}
	if op.asn != s.peerASN {
		conn.Close()
		return fmt.Errorf("unexpected peer ASN %d, want %d", op.asn, s.peerASN)
	}

	// BGP session is established, clear the connect timeout deadline.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return fmt.Errorf("clearing deadline on conn to %q: %s", s.addr, err)
	}

	// Consume BGP messages until the connection closes.
	go s.consumeBGP(conn)

	// Send one keepalive to say that yes, we accept the OPEN.
	if err := sendKeepalive(conn); err != nil {
		conn.Close()
		return fmt.Errorf("accepting peer OPEN from %q: %s", s.addr, err)
	}

	// Set up regular keepalives from now on.
	s.actualHoldTime = s.holdTime
	if op.holdTime < s.actualHoldTime {
		s.actualHoldTime = op.holdTime
	}
	select {
	case s.newHoldTime <- true:
	default:
	}

	s.conn = conn
	return nil
}

// sendKeepalives sends BGP KEEPALIVE packets at the negotiated rate
// whenever the session is connected.
func (s *Session) sendKeepalives() {
	var (
		t  *time.Ticker
		ch <-chan time.Time
	)

	for {
		select {
		case <-s.newHoldTime:
			s.mu.Lock()
			ht := s.actualHoldTime
			s.mu.Unlock()
			if t != nil {
				t.Stop()
				t = nil
				ch = nil
			}
			if ht != 0 {
				t = time.NewTicker(ht / 3)
				ch = t.C
			}

		case <-ch:
			if err := s.sendKeepalive(); err == errClosed {
				// Session has been closed by package caller, we're
				// done here.
				return
			}
		}
	}
}

// sendKeepalive sends a single BGP KEEPALIVE packet.
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
		s.logger.Log("op", "sendKeepalive", "error", err, "msg", "failed to send keepalive")
		return fmt.Errorf("sending keepalive to %q: %s", s.addr, err)
	}
	return nil
}

// New creates a BGP session using the given session parameters.
//
// The session will immediately try to connect and synchronize its
// local state with the peer.
func New(l log.Logger, addr string, asn uint32, routerID net.IP, peerASN uint32, holdTime time.Duration) (*Session, error) {
	ret := &Session{
		addr:        addr,
		asn:         asn,
		routerID:    routerID.To4(),
		peerASN:     peerASN,
		holdTime:    holdTime,
		logger:      log.With(l, "peer", addr, "localASN", asn, "peerASN", peerASN),
		newHoldTime: make(chan bool, 1),
		advertised:  map[string]*Advertisement{},
	}
	ret.cond = sync.NewCond(&ret.mu)
	go ret.sendKeepalives()
	go ret.run()

	stats.sessionUp.WithLabelValues(ret.addr).Set(0)
	stats.prefixes.WithLabelValues(ret.addr).Set(0)

	return ret, nil
}

// consumeBGP receives BGP messages from the peer, and ignores
// them. It does minimal checks for the well-formedness of messages,
// and terminates the connection if something looks wrong.
func (s *Session) consumeBGP(conn io.ReadCloser) {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.conn == conn {
			s.abort()
		} else {
			conn.Close()
		}
	}()

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
		if hdr.Type == 3 {
			// TODO: propagate better than just logging directly.
			err := readNotification(conn)
			s.logger.Log("event", "peerNotification", "error", err, "msg", "peer sent notification, closing session")
			return
		}
		if _, err := io.Copy(ioutil.Discard, io.LimitReader(conn, int64(hdr.Len)-19)); err != nil {
			// TODO: propagate
			return
		}
	}
}

// Set updates the set of Advertisements that this session's peer should receive.
//
// Changes are propagated to the peer asynchronously, Set may return
// before the peer learns about the changes.
func (s *Session) Set(advs ...*Advertisement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newAdvs := map[string]*Advertisement{}
	for _, adv := range advs {
		if adv.Prefix.IP.To4() == nil {
			return fmt.Errorf("cannot advertise non-v4 prefix %q", adv.Prefix)
		}

		if adv.NextHop != nil && adv.NextHop.To4() == nil {
			return fmt.Errorf("next-hop must be IPv4, got %q", adv.NextHop)
		}
		if len(adv.Communities) > 63 {
			return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
		}
		newAdvs[adv.Prefix.String()] = adv
	}

	s.new = newAdvs
	stats.PendingPrefixes(s.addr, len(s.new))
	s.cond.Broadcast()
	return nil
}

// abort closes any existing connection, updates stats, and cleans up
// state ready for another connection attempt.
func (s *Session) abort() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		stats.SessionDown(s.addr)
	}
	// Next time we retry the connection, we can just skip straight to
	// the desired end state.
	if s.new != nil {
		s.advertised, s.new = s.new, nil
		stats.PendingPrefixes(s.addr, len(s.advertised))
	}
	s.cond.Broadcast()
}

// Close shuts down the BGP session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.abort()
	return nil
}

// Advertisement represents one network path and its BGP attributes.
type Advertisement struct {
	// The prefix being advertised to the peer.
	Prefix *net.IPNet
	// The address of the router to which the peer should forward traffic.
	NextHop net.IP
	// The local preference of this route. Only propagated to IBGP
	// peers (i.e. where the peer ASN matches the local ASN).
	LocalPref uint32
	// BGP communities to attach to the path.
	Communities []uint32
}

// Equal returns true if a and b are equivalent advertisements.
func (a *Advertisement) Equal(b *Advertisement) bool {
	if a.Prefix.String() != b.Prefix.String() {
		return false
	}
	if !a.NextHop.Equal(b.NextHop) {
		return false
	}
	if a.LocalPref != b.LocalPref {
		return false
	}
	return reflect.DeepEqual(a.Communities, b.Communities)
}
