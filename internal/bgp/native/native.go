// SPDX-License-Identifier:Apache-2.0

package native

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/safeconvert"
	"golang.org/x/sys/unix"
)

var errClosed = errors.New("session closed")

// session represents one BGP session to an external router.
type session struct {
	bgp.SessionParameters
	peerFBASNSupport bool

	logger log.Logger

	newHoldTime chan bool
	backoff     backoff

	mu             sync.Mutex
	cond           *sync.Cond
	closed         bool
	conn           net.Conn
	actualHoldTime time.Duration
	nextHop        net.IP
	advertised     map[string]*bgp.Advertisement
	new            map[string]*bgp.Advertisement
}

// The 'Native' implementation does not require a session manager .
type sessionManager struct {
}

func NewSessionManager(l log.Logger) bgp.SessionManager {
	return &sessionManager{}
}

// NewSession() creates a BGP session using the given session parameters.
//
// The session will immediately try to connect and synchronize its
// local state with the peer.
func (sm *sessionManager) NewSession(l log.Logger, args bgp.SessionParameters) (bgp.Session, error) {
	sessionsParams := args
	// native mode does not support empty holdtime,
	// we explicitly set it to 90s in this case.
	if args.HoldTime == nil {
		ht := 90 * time.Second
		sessionsParams.HoldTime = &ht
	}
	ret := &session{
		SessionParameters: sessionsParams,
		logger:            log.With(l, "peer", args.PeerAddress, "localASN", args.MyASN, "peerASN", args.PeerASN),
		newHoldTime:       make(chan bool, 1),
		advertised:        map[string]*bgp.Advertisement{},
	}
	ret.cond = sync.NewCond(&ret.mu)
	go ret.sendKeepalives()
	go ret.run()

	stats.sessionUp.WithLabelValues(ret.PeerAddress).Set(0)
	stats.prefixes.WithLabelValues(ret.PeerAddress).Set(0)

	return ret, nil
}

func (sm *sessionManager) SyncBFDProfiles(profiles map[string]*config.BFDProfile) error {
	if len(profiles) > 0 {
		return errors.New("bfd profiles not supported in native mode")
	}
	return nil
}

func (sm *sessionManager) SyncExtraInfo(extras string) error {
	if extras != "" {
		return errors.New("bgp extra info not supported in native mode")
	}
	return nil
}

func (sm *sessionManager) SetEventCallback(func(interface{})) {}

// run tries to stay connected to the peer, and pumps route updates to it.
func (s *session) run() {
	defer stats.DeleteSession(s.PeerAddress)
	for {
		if err := s.connect(); err != nil {
			if err == errClosed {
				return
			}
			level.Error(s.logger).Log("op", "connect", "error", err, "msg", "failed to connect to peer")
			backoff := s.backoff.Duration()
			time.Sleep(backoff)
			continue
		}
		stats.SessionUp(s.PeerAddress)
		s.backoff.Reset()

		level.Info(s.logger).Log("event", "sessionUp", "msg", "BGP session established")

		if !s.sendUpdates() {
			return
		}
		stats.SessionDown(s.PeerAddress)
		level.Warn(s.logger).Log("event", "sessionDown", "msg", "BGP session down")
	}
}

// sendUpdates waits for changes to desired advertisements, and pushes
// them out to the peer.
func (s *session) sendUpdates() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}
	if s.conn == nil {
		return true
	}

	ibgp := s.MyASN == s.PeerASN
	fbasn := s.peerFBASNSupport

	if s.new != nil {
		s.advertised, s.new = s.new, nil
	}

	for c, adv := range s.advertised {
		if err := sendUpdate(s.conn, s.MyASN, ibgp, fbasn, s.nextHop, adv); err != nil {
			s.abort()
			level.Error(s.logger).Log("op", "sendUpdate", "ip", c, "error", err, "msg", "failed to send BGP update")
			return true
		}
		stats.UpdateSent(s.PeerAddress)
	}
	stats.AdvertisedPrefixes(s.PeerAddress, len(s.advertised))

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

			if err := sendUpdate(s.conn, s.MyASN, ibgp, fbasn, s.nextHop, adv); err != nil {
				s.abort()
				level.Error(s.logger).Log("op", "sendUpdate", "prefix", c, "error", err, "msg", "failed to send BGP update")
				return true
			}
			stats.UpdateSent(s.PeerAddress)
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
					level.Error(s.logger).Log("op", "sendWithdraw", "prefix", pfx, "error", err, "msg", "failed to send BGP withdraw")
				}
				return true
			}
			stats.UpdateSent(s.PeerAddress)
		}
		s.advertised, s.new = s.new, nil
		stats.AdvertisedPrefixes(s.PeerAddress, len(s.advertised))
	}
}

// connect establishes the BGP session with the peer.
// Sets TCP_MD5 sockopt if password is !="".
func (s *session) connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	conn, err := dialMD5(ctx, s.PeerAddress, s.SourceAddress, s.Password)
	if err != nil {
		return fmt.Errorf("dial %q: %s", s.PeerAddress, err)
	}

	if err = conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return fmt.Errorf("setting deadline on conn to %q: %s", s.PeerAddress, err)
	}

	addr, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		conn.Close()
		return fmt.Errorf("getting local addr for default nexthop to %q: %s", s.PeerAddress, err)
	}
	s.nextHop = addr.IP

	routerID := s.RouterID
	if routerID == nil {
		routerID, err = getRouterID(s.nextHop, s.CurrentNode)
		if err != nil {
			return err
		}
	}

	if err = sendOpen(conn, s.MyASN, routerID, *s.HoldTime); err != nil {
		conn.Close()
		return fmt.Errorf("send OPEN to %q: %s", s.PeerAddress, err)
	}

	op, err := readOpen(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("read OPEN from %q: %s", s.PeerAddress, err)
	}
	if op.asn != s.PeerASN {
		conn.Close()
		return fmt.Errorf("unexpected peer ASN %d, want %d", op.asn, s.PeerASN)
	}
	s.peerFBASNSupport = op.fbasn
	if s.MyASN > 65536 && !s.peerFBASNSupport {
		conn.Close()
		return fmt.Errorf("peer does not support 4-byte ASNs")
	}

	// BGP session is established, clear the connect timeout deadline.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return fmt.Errorf("clearing deadline on conn to %q: %s", s.PeerAddress, err)
	}

	// Consume BGP messages until the connection closes.
	go s.consumeBGP(conn)

	// Send one keepalive to say that yes, we accept the OPEN.
	if err := sendKeepalive(conn); err != nil {
		conn.Close()
		return fmt.Errorf("accepting peer OPEN from %q: %s", s.PeerAddress, err)
	}

	// Set up regular keepalives from now on.
	s.actualHoldTime = *s.HoldTime
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

func hashRouterID(hostname string) (net.IP, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, crc32.ChecksumIEEE([]byte(hostname)))
	if err != nil {
		return nil, err
	}
	return net.IP(buf.Bytes()), nil
}

// Ipv4; Use the address as-is.
// Ipv6; Pick the first ipv4 address on the same interface as the address.
func getRouterID(addr net.IP, myNode string) (net.IP, error) {
	if addr.To4() != nil {
		return addr, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return hashRouterID(myNode)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip.Equal(addr) {
				// This is the interface.
				// Loop through the addresses again and search for ipv4
				for _, a := range addrs {
					var ip net.IP
					switch v := a.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip.To4() != nil {
						return ip, nil
					}
				}
				return hashRouterID(myNode)
			}
		}
	}
	return hashRouterID(myNode)
}

// sendKeepalives sends BGP KEEPALIVE packets at the negotiated rate
// whenever the session is connected.
func (s *session) sendKeepalives() {
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
func (s *session) sendKeepalive() error {
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
		level.Error(s.logger).Log("op", "sendKeepalive", "error", err, "msg", "failed to send keepalive")
		return fmt.Errorf("sending keepalive to %q: %s", s.PeerAddress, err)
	}
	return nil
}

// consumeBGP receives BGP messages from the peer, and ignores
// them. It does minimal checks for the well-formedness of messages,
// and terminates the connection if something looks wrong.
func (s *session) consumeBGP(conn io.ReadCloser) {
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
			level.Error(s.logger).Log("event", "peerNotification", "error", err, "msg", "peer sent notification, closing session")
			return
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(conn, int64(hdr.Len)-19)); err != nil {
			// TODO: propagate
			return
		}
	}
}

func validate(adv *bgp.Advertisement) error {
	if adv.Prefix.IP.To4() == nil {
		return fmt.Errorf("cannot advertise non-v4 prefix %q", adv.Prefix)
	}

	if len(adv.Communities) > 63 {
		return fmt.Errorf("max supported communities is 63, got %d", len(adv.Communities))
	}
	return nil
}

// Set updates the set of Advertisements that this session's peer should receive.
//
// Changes are propagated to the peer asynchronously, Set may return
// before the peer learns about the changes.
func (s *session) Set(advs ...*bgp.Advertisement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newAdvs := map[string]*bgp.Advertisement{}
	for _, adv := range advs {
		err := validate(adv)
		if err != nil {
			return err
		}
		newAdvs[adv.Prefix.String()] = adv
	}

	s.new = newAdvs
	stats.PendingPrefixes(s.PeerAddress, len(s.new))
	s.cond.Broadcast()

	return nil
}

// abort closes any existing connection, updates stats, and cleans up
// state ready for another connection attempt.
func (s *session) abort() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		stats.SessionDown(s.PeerAddress)
	}
	// Next time we retry the connection, we can just skip straight to
	// the desired end state.
	if s.new != nil {
		s.advertised, s.new = s.new, nil
		stats.PendingPrefixes(s.PeerAddress, len(s.advertised))
	}
	s.cond.Broadcast()
}

// Close shuts down the BGP session.
func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.abort()
	return nil
}

// DialTCP does the part of creating a connection manually,  including setting the
// proper TCP MD5 options when the password is not empty. Works by manipulating
// the low level FD's, skipping the net.Conn API as it has not hooks to set
// the necessary sockopts for TCP MD5.
func dialMD5(ctx context.Context, addr string, srcAddr net.IP, password string) (net.Conn, error) {
	// If srcAddr exists on any of the local network interfaces, use it as the
	// source address of the TCP socket. Otherwise, use the IPv6 unspecified
	// address ("::") to let the kernel figure out the source address.
	// NOTE: On Linux, "::" also includes "0.0.0.0" (all IPv4 addresses).
	a := "[::]"
	if srcAddr != nil {
		ifs, err := net.Interfaces()
		if err != nil {
			return nil, fmt.Errorf("querying local interfaces: %w", err)
		}

		if !localAddressExists(ifs, srcAddr) {
			return nil, fmt.Errorf("address %q doesn't exist on this host", srcAddr)
		}

		a = fmt.Sprintf("[%s]", srcAddr.String())
	}

	laddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", a))
	if err != nil {
		return nil, fmt.Errorf("error resolving local address: %s ", err)
	}

	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("invalid remote address: %s ", err)
	}

	var family int
	var ra, la unix.Sockaddr
	if raddr.IP.To4() != nil {
		family = unix.AF_INET
		rsockaddr := &unix.SockaddrInet4{Port: raddr.Port}
		copy(rsockaddr.Addr[:], raddr.IP.To4())
		ra = rsockaddr
		lsockaddr := &unix.SockaddrInet4{}
		copy(lsockaddr.Addr[:], laddr.IP.To4())
		la = lsockaddr
	} else {
		family = unix.AF_INET6
		rsockaddr := &unix.SockaddrInet6{Port: raddr.Port}
		copy(rsockaddr.Addr[:], raddr.IP.To16())
		ra = rsockaddr
		var zone uint32
		if laddr.Zone != "" {
			intf, errs := net.InterfaceByName(laddr.Zone)
			if errs != nil {
				return nil, errs
			}
			zone, err = safeconvert.IntToUInt32(intf.Index)
			if err != nil {
				return nil, fmt.Errorf("invalid interface index %d", intf.Index)
			}
		}
		lsockaddr := &unix.SockaddrInet6{ZoneId: zone}
		copy(lsockaddr.Addr[:], laddr.IP.To16())
		la = lsockaddr
	}

	sockType := unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK
	proto := 0
	fd, err := unix.Socket(family, sockType, proto)
	if err != nil {
		return nil, err
	}

	// A new socket was created so we must close it before this
	// function returns either on failure or success. On success,
	// net.FileConn() in newTCPConn() increases the refcount of
	// the socket so this fi.Close() doesn't destroy the socket.
	// The caller must call Close() with the file later.
	// Note that the above os.NewFile() doesn't play with the
	// refcount.
	fi := os.NewFile(uintptr(fd), "")
	defer func() {
		if tmpErr := fi.Close(); tmpErr != nil {
			err = tmpErr
		}
	}()

	if password != "" {
		sig, err := buildTCPMD5Sig(raddr.IP, password)
		if err != nil {
			return nil, err
		}
		// Better way may be available in  Go 1.11, see go-review.googlesource.com/c/go/+/72810
		if err = os.NewSyscallError("setsockopt", unix.SetsockoptTCPMD5Sig(fd, unix.IPPROTO_TCP, unix.TCP_MD5SIG, sig)); err != nil {
			return nil, err
		}
	}

	if err = unix.Bind(fd, la); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	err = unix.Connect(fd, ra)

	switch err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil:
		return net.FileConn(fi)
	default:
		return nil, os.NewSyscallError("connect", err)
	}

	// With a non-blocking socket, the connection process is
	// asynchronous, so we need to manually wait with epoll until the
	// connection succeeds. All of the following is doing that, with
	// appropriate use of the deadline in the context.
	epfd, err := unix.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	defer unix.Close(epfd)

	var event unix.EpollEvent
	events := make([]unix.EpollEvent, 1)

	event.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLPRI
	event.Fd, err = safeconvert.IntToInt32(fd)
	if err != nil {
		return nil, fmt.Errorf("invalid fd %w", err)
	}
	if err = unix.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
		return nil, err
	}

	for {
		timeout := int(-1)
		if deadline, ok := ctx.Deadline(); ok {
			timeout = int(time.Until(deadline).Nanoseconds() / 1000000)
			if timeout <= 0 {
				return nil, fmt.Errorf("timeout")
			}
		}
		nevents, err := unix.EpollWait(epfd, events, timeout)
		if err != nil {
			return nil, err
		}
		if nevents == 0 {
			return nil, fmt.Errorf("timeout")
		}
		fdToCheck, err := safeconvert.IntToInt32(fd)
		if err != nil {
			return nil, fmt.Errorf("invalid fd %w", err)
		}
		if nevents > 1 || events[0].Fd != fdToCheck {
			return nil, fmt.Errorf("unexpected epoll behavior")
		}

		nerr, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
		if err != nil {
			return nil, os.NewSyscallError("getsockopt", err)
		}
		switch err := syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
		case syscall.Errno(0), unix.EISCONN:
			return net.FileConn(fi)
		default:
			return nil, os.NewSyscallError("getsockopt", err)
		}
	}
}

func buildTCPMD5Sig(addr net.IP, key string) (*unix.TCPMD5Sig, error) {
	t := unix.TCPMD5Sig{}
	if addr.To4() != nil {
		t.Addr.Family = unix.AF_INET
		copy(t.Addr.Data[2:], addr.To4())
	} else {
		t.Addr.Family = unix.AF_INET6
		copy(t.Addr.Data[6:], addr.To16())
	}

	var err error
	t.Keylen, err = safeconvert.IntToUInt16(len(key))
	if err != nil {
		return nil, fmt.Errorf("invalid keyLen %w", err)
	}
	copy(t.Key[0:], []byte(key))

	return &t, nil
}

// localAddressExists returns true if the address addr exists on any of the
// network interfaces in the ifs slice.
func localAddressExists(ifs []net.Interface, addr net.IP) bool {
	for _, i := range ifs {
		addresses, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addresses {
			ip, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip.IP.Equal(addr) {
				return true
			}
		}
	}

	return false
}
