// +build linux

package raw

import (
	"net"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

var (
	// Must implement net.PacketConn at compile-time.
	_ net.PacketConn = &packetConn{}
)

// packetConn is the Linux-specific implementation of net.PacketConn for this
// package.
type packetConn struct {
	ifi *net.Interface
	s   socket

	// Timeouts set via Set{Read,}Deadline, guarded by mutex
	timeoutMu sync.RWMutex
	rtimeout  time.Time
}

// socket is an interface which enables swapping out socket syscalls for
// testing.
type socket interface {
	Bind(syscall.Sockaddr) error
	Close() error
	FD() int
	Recvfrom([]byte, int) (int, syscall.Sockaddr, error)
	Sendto([]byte, int, syscall.Sockaddr) error
	SetSockopt(level, name int, v unsafe.Pointer, l uint32) error
	SetTimeout(time.Duration) error
}

// sleeper is an interface which enables swapping out an actual time.Sleep
// call for testing.
type sleeper interface {
	Sleep(time.Duration)
}

// listenPacket creates a net.PacketConn which can be used to send and receive
// data at the device driver level.
func listenPacket(ifi *net.Interface, proto uint16, cfg *Config) (*packetConn, error) {
	// Convert proto to big endian.
	pbe := htons(proto)

	// Enabling overriding the socket type via config.
	typ := syscall.SOCK_RAW
	if cfg != nil && cfg.LinuxSockDGRAM {
		typ = syscall.SOCK_DGRAM
	}

	// Open a packet socket using specified socket and protocol types.
	sock, err := syscall.Socket(syscall.AF_PACKET, typ, int(pbe))
	if err != nil {
		return nil, err
	}

	// Wrap raw socket in socket interface, use actual time package sleeper
	return newPacketConn(
		ifi,
		&sysSocket{
			fd: sock,
		},
		pbe,
	)
}

// newPacketConn creates a net.PacketConn using the specified network
// interface, wrapped socket and big endian protocol number.
//
// It is the entry point for tests in this package.
func newPacketConn(ifi *net.Interface, s socket, pbe uint16) (*packetConn, error) {
	// Bind the packet socket to the interface specified by ifi
	// packet(7):
	//   Only the sll_protocol and the sll_ifindex address fields are used for
	//   purposes of binding.
	err := s.Bind(&syscall.SockaddrLinklayer{
		Protocol: pbe,
		Ifindex:  ifi.Index,
	})

	return &packetConn{
		ifi: ifi,
		s:   s,
	}, err
}

// ReadFrom implements the net.PacketConn.ReadFrom method.
func (p *packetConn) ReadFrom(b []byte) (int, net.Addr, error) {
	p.timeoutMu.Lock()
	deadline := p.rtimeout
	p.timeoutMu.Unlock()

	// Information returned by syscall.Recvfrom
	var n int
	var addr syscall.Sockaddr

	for {
		var timeout time.Duration

		if deadline.IsZero() {
			timeout = readTimeout
		} else {
			timeout = deadline.Sub(time.Now())
			if timeout > readTimeout {
				timeout = readTimeout
			}
		}

		err := p.s.SetTimeout(timeout)
		if err != nil {
			return 0, nil, err
		}

		// Attempt to receive on socket
		// The recvfrom sycall will NOT be interrupted by closing of the socket
		n, addr, err = p.s.Recvfrom(b, 0)

		if err == syscall.EAGAIN {
			// timeout
			continue
		}
		if err != nil {
			n = 0
			// Return on error
			return n, nil, err
		}

		// Got data, exit the loop
		break
	}

	// Retrieve hardware address and other information from addr
	sa, ok := addr.(*syscall.SockaddrLinklayer)
	if !ok || sa.Halen < 6 {
		return n, nil, syscall.EINVAL
	}

	// Use length specified to convert byte array into a hardware address slice
	mac := make(net.HardwareAddr, sa.Halen)
	copy(mac, sa.Addr[:])

	// packet(7):
	//   sll_hatype and sll_pkttype are set on received packets for your
	//   information.
	// TODO(mdlayher): determine if similar fields exist and are useful on
	// non-Linux platforms
	return n, &Addr{
		HardwareAddr: mac,
	}, nil
}

// WriteTo implements the net.PacketConn.WriteTo method.
func (p *packetConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	// Ensure correct Addr type
	a, ok := addr.(*Addr)
	if !ok || a.HardwareAddr == nil || len(a.HardwareAddr) < 6 {
		return 0, syscall.EINVAL
	}

	// Convert hardware address back to byte array form
	var baddr [8]byte
	copy(baddr[:], a.HardwareAddr)

	// Send message on socket to the specified hardware address from addr
	// packet(7):
	//   When you send packets it is enough to specify sll_family, sll_addr,
	//   sll_halen, sll_ifindex.  The other fields should  be 0.
	// In this case, sll_family is taken care of automatically by syscall
	err := p.s.Sendto(b, 0, &syscall.SockaddrLinklayer{
		Ifindex: p.ifi.Index,
		Halen:   uint8(len(a.HardwareAddr)),
		Addr:    baddr,
	})
	return len(b), err
}

// Close closes the connection.
func (p *packetConn) Close() error {
	return p.s.Close()
}

// LocalAddr returns the local network address.
func (p *packetConn) LocalAddr() net.Addr {
	return &Addr{
		HardwareAddr: p.ifi.HardwareAddr,
	}
}

// TODO(mdlayher): it is unfortunate that we have to implement deadlines using
// a context, but it appears that there may not be a better solution until
// Go 1.6 or later.  See here: https://github.com/golang/go/issues/10565.

// SetDeadline implements the net.PacketConn.SetDeadline method.
func (p *packetConn) SetDeadline(t time.Time) error {
	return p.SetReadDeadline(t)
}

// SetReadDeadline implements the net.PacketConn.SetReadDeadline method.
func (p *packetConn) SetReadDeadline(t time.Time) error {
	p.timeoutMu.Lock()
	p.rtimeout = t
	p.timeoutMu.Unlock()
	return nil
}

// SetWriteDeadline implements the net.PacketConn.SetWriteDeadline method.
func (p *packetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// SetBPF attaches an assembled BPF program to a raw net.PacketConn.
func (p *packetConn) SetBPF(filter []bpf.RawInstruction) error {
	prog := syscall.SockFprog{
		Len:    uint16(len(filter)),
		Filter: (*syscall.SockFilter)(unsafe.Pointer(&filter[0])),
	}

	err := p.s.SetSockopt(
		syscall.SOL_SOCKET,
		syscall.SO_ATTACH_FILTER,
		unsafe.Pointer(&prog),
		uint32(unsafe.Sizeof(prog)),
	)
	if err != nil {
		return os.NewSyscallError("setsockopt", err)
	}

	return nil
}

// SetPromiscuous enables or disables promiscuous mode on the interface, allowing it
// to receive traffic that is not addressed to the interface.
func (p *packetConn) SetPromiscuous(b bool) error {
	mreq := unix.PacketMreq{
		Ifindex: int32(p.ifi.Index),
		Type:    unix.PACKET_MR_PROMISC,
	}

	membership := unix.PACKET_ADD_MEMBERSHIP
	if !b {
		membership = unix.PACKET_DROP_MEMBERSHIP
	}

	return p.s.SetSockopt(unix.SOL_PACKET, membership, unsafe.Pointer(&mreq), unix.SizeofPacketMreq)
}

// sysSocket is the default socket implementation.  It makes use of
// Linux-specific system calls to handle raw socket functionality.
type sysSocket struct {
	fd int
}

// Method implementations simply invoke the syscall of the same name, but pass
// the file descriptor stored in the sysSocket as the socket to use.
func (s *sysSocket) Bind(sa syscall.Sockaddr) error { return syscall.Bind(s.fd, sa) }
func (s *sysSocket) Close() error                   { return syscall.Close(s.fd) }
func (s *sysSocket) FD() int                        { return s.fd }
func (s *sysSocket) Recvfrom(p []byte, flags int) (int, syscall.Sockaddr, error) {
	return syscall.Recvfrom(s.fd, p, flags)
}
func (s *sysSocket) Sendto(p []byte, flags int, to syscall.Sockaddr) error {
	return syscall.Sendto(s.fd, p, flags, to)
}
func (s *sysSocket) SetSockopt(level, name int, v unsafe.Pointer, l uint32) error {
	_, _, err := syscall.Syscall6(syscall.SYS_SETSOCKOPT, uintptr(s.fd), uintptr(level), uintptr(name), uintptr(v), uintptr(l), 0)
	if err != 0 {
		return syscall.Errno(err)
	}
	return nil
}
func (s *sysSocket) SetTimeout(timeout time.Duration) error {
	tv, err := newTimeval(timeout)
	if err != nil {
		return err
	}
	return syscall.SetsockoptTimeval(s.fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, tv)
}
