package arp

import (
	"bytes"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestClientClose(t *testing.T) {
	p := &closeCapturePacketConn{}
	c := &Client{p: p}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	if !p.closed {
		t.Fatal("client was not closed")
	}
}

func TestClientSetDeadline(t *testing.T) {
	p := &deadlineCapturePacketConn{}
	c := &Client{p: p}

	d := time.Now()
	if err := c.SetDeadline(d); err != nil {
		t.Fatal(err)
	}

	if want, got := d, p.r; want != got {
		t.Fatalf("unexpected read deadline: %v != %v", want, got)
	}
	if want, got := d, p.w; want != got {
		t.Fatalf("unexpected write deadline: %v != %v", want, got)
	}
}

func TestClientSetReadDeadline(t *testing.T) {
	p := &deadlineCapturePacketConn{}
	c := &Client{p: p}

	d := time.Now()
	if err := c.SetReadDeadline(d); err != nil {
		t.Fatal(err)
	}

	if want, got := d, p.r; want != got {
		t.Fatalf("unexpected read deadline: %v != %v", want, got)
	}
	if want, got := (time.Time{}), p.w; want != got {
		t.Fatalf("non-zero write deadline: %v", got)
	}
}

func TestClientSetWriteDeadline(t *testing.T) {
	p := &deadlineCapturePacketConn{}
	c := &Client{p: p}

	d := time.Now()
	if err := c.SetWriteDeadline(d); err != nil {
		t.Fatal(err)
	}

	if want, got := (time.Time{}), p.r; want != got {
		t.Fatalf("non-zero read deadline: %v", got)
	}
	if want, got := d, p.w; want != got {
		t.Fatalf("unexpected write deadline: %v != %v", want, got)
	}
}

func TestClientHardwareAddr(t *testing.T) {
	c := &Client{
		ifi: &net.Interface{
			HardwareAddr: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		},
	}

	if want, got := c.ifi.HardwareAddr.String(), c.HardwareAddr().String(); want != got {
		t.Fatalf("unexpected hardware address: %v != %v", want, got)
	}
}

func Test_newClient(t *testing.T) {
	var tests = []struct {
		desc  string
		addrs []net.Addr
		c     *Client
		err   error
	}{
		{
			desc: "no network addresses",
			c:    &Client{},
		},
		{
			desc: "OK",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.IPv4(192, 168, 1, 1),
					Mask: []byte{255, 255, 255, 0},
				},
			},
			c: &Client{
				ip: net.IPv4(192, 168, 1, 1).To4(),
			},
		},
	}

	for i, tt := range tests {
		c, err := newClient(nil, nil, tt.addrs)
		if err != nil {
			if want, got := tt.err.Error(), err.Error(); want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.c, c; !reflect.DeepEqual(want, got) {
			t.Fatalf("[%02d] test %q, unexpected Client: %v != %v",
				i, tt.desc, want, got)
		}
	}
}

func Test_firstIPv4Addr(t *testing.T) {
	var tests = []struct {
		desc  string
		addrs []net.Addr
		ip    net.IP
		err   error
	}{
		{
			desc: "no network addresses",
		},
		{
			desc: "non-IP network",
			addrs: []net.Addr{
				&net.UnixAddr{
					Name: "foo.sock",
					Net:  "unix",
				},
			},
		},
		{
			desc: "bad CIDR address",
			addrs: []net.Addr{
				&net.IPNet{
					IP: net.IPv4(192, 168, 1, 1),
				},
			},
			err: &net.ParseError{
				Type: "CIDR address",
				Text: "<nil>",
			},
		},
		{
			desc: "IPv6 address only",
			addrs: []net.Addr{
				&net.IPNet{
					IP: net.IPv6loopback,
					Mask: []byte{
						0xff, 0xff, 0xff, 0xff,
						0xff, 0xff, 0xff, 0xff,
						0, 0, 0, 0,
						0, 0, 0, 0,
					},
				},
			},
		},
		{
			desc: "IPv4 address only",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.IPv4(192, 168, 1, 1),
					Mask: []byte{255, 255, 255, 0},
				},
			},
			ip: net.IPv4(192, 168, 1, 1),
		},
		{
			desc: "IPv4 and IPv6 addresses",
			addrs: []net.Addr{
				&net.IPNet{
					IP: net.IPv6loopback,
					Mask: []byte{
						0xff, 0xff, 0xff, 0xff,
						0xff, 0xff, 0xff, 0xff,
						0, 0, 0, 0,
						0, 0, 0, 0,
					},
				},
				&net.IPNet{
					IP:   net.IPv4(192, 168, 1, 1),
					Mask: []byte{255, 255, 255, 0},
				},
			},
			ip: net.IPv4(192, 168, 1, 1),
		},
		{
			desc: "multiple IPv4 addresses",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.IPv4(10, 0, 0, 1),
					Mask: []byte{255, 0, 0, 0},
				},
				&net.IPNet{
					IP:   net.IPv4(192, 168, 1, 1),
					Mask: []byte{255, 255, 255, 0},
				},
			},
			ip: net.IPv4(10, 0, 0, 1),
		},
	}

	for i, tt := range tests {
		ip, err := firstIPv4Addr(tt.addrs)
		if err != nil {
			if want, got := tt.err.Error(), err.Error(); want != got {
				t.Fatalf("[%02d] test %q, unexpected error: %v != %v",
					i, tt.desc, want, got)
			}

			continue
		}

		if want, got := tt.ip.To4(), ip.To4(); !bytes.Equal(want, got) {
			t.Fatalf("[%02d] test %q, unexpected IPv4 address: %v != %v",
				i, tt.desc, want, got)
		}
	}
}

// closeCapturePacketConn is a net.PacketConn which captures when
// it is closed.
type closeCapturePacketConn struct {
	closed bool

	noopPacketConn
}

func (p *closeCapturePacketConn) Close() error {
	p.closed = true
	return nil
}

// deadlineCapturePacketConn is a net.PacketConn which captures read and
// write deadlines.
type deadlineCapturePacketConn struct {
	r time.Time
	w time.Time

	noopPacketConn
}

func (p *deadlineCapturePacketConn) SetDeadline(t time.Time) error {
	p.r = t
	p.w = t
	return nil
}
func (p *deadlineCapturePacketConn) SetReadDeadline(t time.Time) error {
	p.r = t
	return nil
}
func (p *deadlineCapturePacketConn) SetWriteDeadline(t time.Time) error {
	p.w = t
	return nil
}

// noopPacketConn is a net.PacketConn which simply no-ops any input.  It is
// embedded in other implementations so they do not have to implement every
// single method.
type noopPacketConn struct{}

func (noopPacketConn) ReadFrom(b []byte) (int, net.Addr, error)     { return 0, nil, nil }
func (noopPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) { return 0, nil }

func (noopPacketConn) Close() error                       { return nil }
func (noopPacketConn) LocalAddr() net.Addr                { return nil }
func (noopPacketConn) SetDeadline(t time.Time) error      { return nil }
func (noopPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (noopPacketConn) SetWriteDeadline(t time.Time) error { return nil }
func (noopPacketConn) HardwareAddr() net.HardwareAddr     { return nil }
