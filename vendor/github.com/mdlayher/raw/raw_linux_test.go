// +build linux

package raw

import (
	"bytes"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// Test to ensure that socket is bound with correct sockaddr_ll information

type bindSocket struct {
	bind syscall.Sockaddr
	noopSocket
}

func (s *bindSocket) Bind(sa syscall.Sockaddr) error {
	s.bind = sa
	return nil
}

func Test_newPacketConnBind(t *testing.T) {
	s := &bindSocket{}

	ifIndex := 1
	protocol := uint16(1)

	_, err := newPacketConn(
		&net.Interface{
			Index: ifIndex,
		},
		s,
		protocol,
	)
	if err != nil {
		t.Fatal(err)
	}

	sall, ok := s.bind.(*syscall.SockaddrLinklayer)
	if !ok {
		t.Fatalf("bind sockaddr has incorrect type: %T", s.bind)
	}

	if want, got := ifIndex, sall.Ifindex; want != got {
		t.Fatalf("unexpected network interface index:\n- want: %v\n-  got: %v", want, got)
	}
	if want, got := protocol, sall.Protocol; want != got {
		t.Fatalf("unexpected protocol:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test for incorrect sockaddr type after recvfrom on a socket.

type addrRecvfromSocket struct {
	addr syscall.Sockaddr
	noopSocket
}

func (s *addrRecvfromSocket) Recvfrom(p []byte, flags int) (int, syscall.Sockaddr, error) {
	return 0, s.addr, nil
}

func Test_packetConnReadFromRecvfromInvalidSockaddr(t *testing.T) {
	p, err := newPacketConn(
		&net.Interface{},
		&addrRecvfromSocket{
			addr: &syscall.SockaddrInet4{},
		},
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = p.ReadFrom(nil)
	if want, got := syscall.EINVAL, err; want != got {
		t.Fatalf("unexpected error:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test for malformed hardware address after recvfrom on a socket

func Test_packetConnReadFromRecvfromInvalidHardwareAddr(t *testing.T) {
	p, err := newPacketConn(
		&net.Interface{},
		&addrRecvfromSocket{
			addr: &syscall.SockaddrLinklayer{
				Halen: 5,
			},
		},
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = p.ReadFrom(nil)
	if want, got := syscall.EINVAL, err; want != got {
		t.Fatalf("unexpected error:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test for a correct ReadFrom with data and address.

type recvfromSocket struct {
	p     []byte
	flags int
	addr  syscall.Sockaddr
	noopSocket
}

func (s *recvfromSocket) Recvfrom(p []byte, flags int) (int, syscall.Sockaddr, error) {
	copy(p, s.p)
	s.flags = flags
	return len(s.p), s.addr, nil
}

func Test_packetConnReadFromRecvfromOK(t *testing.T) {
	const wantN = 4
	data := []byte{0, 1, 2, 3}
	deadbeefHW := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	s := &recvfromSocket{
		p: data,
		addr: &syscall.SockaddrLinklayer{
			Halen: 6,
			Addr:  [8]byte{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0x00, 0x00},
		},
	}

	p, err := newPacketConn(
		&net.Interface{},
		s,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 8)
	n, addr, err := p.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, s.flags; want != got {
		t.Fatalf("unexpected flags:\n- want: %v\n-  got: %v", want, got)
	}

	raddr, ok := addr.(*Addr)
	if !ok {
		t.Fatalf("read sockaddr has incorrect type: %T", addr)
	}
	if want, got := deadbeefHW, raddr.HardwareAddr; !bytes.Equal(want, got) {
		t.Fatalf("unexpected hardware address:\n- want: %v\n-  got: %v", want, got)
	}

	if want, got := wantN, n; want != got {
		t.Fatalf("unexpected data length:\n- want: %v\n-  got: %v", want, got)
	}

	if want, got := data, buf[:n]; !bytes.Equal(want, got) {
		t.Fatalf("unexpected data:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test for incorrect sockaddr type for WriteTo.

func Test_packetConnWriteToInvalidSockaddr(t *testing.T) {
	_, err := (&packetConn{}).WriteTo(nil, &net.IPAddr{})
	if want, got := syscall.EINVAL, err; want != got {
		t.Fatalf("unexpected error:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test for malformed hardware address with WriteTo.

func Test_packetConnWriteToInvalidHardwareAddr(t *testing.T) {
	addrs := []net.HardwareAddr{
		// Too short.
		{0xde, 0xad, 0xbe, 0xef, 0xde},
		// Explicitly nil.
		nil,
	}

	for _, addr := range addrs {
		_, err := (&packetConn{}).WriteTo(nil, &Addr{
			HardwareAddr: addr,
		})
		if want, got := syscall.EINVAL, err; want != got {
			t.Fatalf("unexpected error:\n- want: %v\n-  got: %v", want, got)
		}
	}
}

// Test for a correct WriteTo with data and address.

type sendtoSocket struct {
	p     []byte
	flags int
	addr  syscall.Sockaddr
	noopSocket
}

func (s *sendtoSocket) Sendto(p []byte, flags int, to syscall.Sockaddr) error {
	copy(s.p, p)
	s.flags = flags
	s.addr = to
	return nil
}

func Test_packetConnWriteToSendtoOK(t *testing.T) {
	const wantN = 4
	data := []byte{0, 1, 2, 3}

	deadbeefHW := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	s := &sendtoSocket{
		p: make([]byte, wantN),
	}

	p, err := newPacketConn(
		&net.Interface{},
		s,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	n, err := p.WriteTo(data, &Addr{
		HardwareAddr: deadbeefHW,
	})
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, s.flags; want != got {
		t.Fatalf("unexpected flags:\n- want: %v\n-  got: %v", want, got)
	}

	if want, got := wantN, n; want != got {
		t.Fatalf("unexpected data length:\n- want: %v\n-  got: %v", want, got)
	}
	if want, got := data, s.p; !bytes.Equal(want, got) {
		t.Fatalf("unexpected data:\n- want: %v\n-  got: %v", want, got)
	}

	sall, ok := s.addr.(*syscall.SockaddrLinklayer)
	if !ok {
		t.Fatalf("write sockaddr has incorrect type: %T", s.addr)
	}

	if want, got := deadbeefHW, sall.Addr[:][:sall.Halen]; !bytes.Equal(want, got) {
		t.Fatalf("unexpected hardware address:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test that socket close functions as intended.

type captureCloseSocket struct {
	closed bool
	noopSocket
}

func (s *captureCloseSocket) Close() error {
	s.closed = true
	return nil
}

func Test_packetConnClose(t *testing.T) {
	s := &captureCloseSocket{}
	p := &packetConn{
		s: s,
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	if !s.closed {
		t.Fatalf("socket should be closed, but is not")
	}
}

// Test that LocalAddr returns the hardware address of the network interface
// which is being used by the socket.

func Test_packetConnLocalAddr(t *testing.T) {
	deadbeefHW := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	p := &packetConn{
		ifi: &net.Interface{
			HardwareAddr: deadbeefHW,
		},
	}

	if want, got := deadbeefHW, p.LocalAddr().(*Addr).HardwareAddr; !bytes.Equal(want, got) {
		t.Fatalf("unexpected hardware address:\n- want: %v\n-  got: %v", want, got)
	}
}

// Test that BPF filter attachment works as intended.

type setSockoptSocket struct {
	setsockopt func(level, name int, v unsafe.Pointer, l uint32) error
	noopSocket
}

func (s *setSockoptSocket) SetSockopt(level, name int, v unsafe.Pointer, l uint32) error {
	return s.setsockopt(level, name, v, l)
}

func Test_packetConnSetBPF(t *testing.T) {
	filter, err := bpf.Assemble([]bpf.Instruction{
		bpf.RetConstant{Val: 0},
	})
	if err != nil {
		t.Fatalf("failed to assemble filter: %v", err)
	}

	fn := func(level, name int, _ unsafe.Pointer, _ uint32) error {
		// Though we can't check the filter itself, we can check the setsockopt
		// level and name for correctness.
		if want, got := syscall.SOL_SOCKET, level; want != got {
			t.Fatalf("unexpected setsockopt level:\n- want: %v\n-  got: %v", want, got)
		}
		if want, got := syscall.SO_ATTACH_FILTER, name; want != got {
			t.Fatalf("unexpected setsockopt name:\n- want: %v\n-  got: %v", want, got)
		}

		return nil
	}

	s := &setSockoptSocket{
		setsockopt: fn,
	}
	p := &packetConn{
		s: s,
	}

	if err := p.SetBPF(filter); err != nil {
		t.Fatalf("failed to attach filter: %v", err)
	}
}

// Test that timeouts are not set when they are disabled.

type setTimeoutSocket struct {
	setTimeout func(timeout time.Duration) error
	noopSocket
}

func (s *setTimeoutSocket) Recvfrom(p []byte, flags int) (int, syscall.Sockaddr, error) {
	return 0, &syscall.SockaddrLinklayer{
		Halen: 6,
	}, nil
}

func (s *setTimeoutSocket) SetTimeout(timeout time.Duration) error {
	return s.setTimeout(timeout)
}

func Test_packetConnNoTimeouts(t *testing.T) {
	s := &setTimeoutSocket{
		setTimeout: func(_ time.Duration) error {
			panic("a timeout was set")
		},
	}

	p := &packetConn{
		s:          s,
		noTimeouts: true,
	}

	buf := make([]byte, 64)
	if _, _, err := p.ReadFrom(buf); err != nil {
		t.Fatalf("failed to read: %v", err)
	}
}

func Test_packetConn_handleStats(t *testing.T) {
	tests := []struct {
		name         string
		noCumulative bool
		stats        []unix.TpacketStats
		out          []Stats
	}{
		{
			name:         "no cumulative",
			noCumulative: true,
			stats: []unix.TpacketStats{
				// Expect these exact outputs.
				{Packets: 1, Drops: 1},
				{Packets: 2, Drops: 2},
			},
			out: []Stats{
				{Packets: 1, Drops: 1},
				{Packets: 2, Drops: 2},
			},
		},
		{
			name: "cumulative",
			stats: []unix.TpacketStats{
				// Expect accumulation of structures.
				{Packets: 1, Drops: 1},
				{Packets: 2, Drops: 2},
			},
			out: []Stats{
				{Packets: 1, Drops: 1},
				{Packets: 3, Drops: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &packetConn{noCumulativeStats: tt.noCumulative}

			if diff := cmp.Diff(len(tt.stats), len(tt.out)); diff != "" {
				t.Fatalf("unexpected number of test cases (-want +got):\n%s", diff)
			}

			for i := 0; i < len(tt.stats); i++ {
				out := *p.handleStats(tt.stats[i])

				if diff := cmp.Diff(tt.out[i], out); diff != "" {
					t.Fatalf("unexpected Stats[%02d] (-want +got):\n%s", i, diff)
				}
			}

		})
	}
}

// noopSocket is a socket implementation which noops every operation.  It is
// the basis for more specific socket implementations.
type noopSocket struct{}

func (noopSocket) Bind(sa syscall.Sockaddr) error                                { return nil }
func (noopSocket) Close() error                                                  { return nil }
func (noopSocket) FD() int                                                       { return 0 }
func (noopSocket) GetSockopt(level, name int, v unsafe.Pointer, l uintptr) error { return nil }
func (noopSocket) Recvfrom(p []byte, flags int) (int, syscall.Sockaddr, error)   { return 0, nil, nil }
func (noopSocket) Sendto(p []byte, flags int, to syscall.Sockaddr) error         { return nil }
func (noopSocket) SetSockopt(level, name int, v unsafe.Pointer, l uint32) error  { return nil }
func (noopSocket) SetTimeout(timeout time.Duration) error                        { return nil }
