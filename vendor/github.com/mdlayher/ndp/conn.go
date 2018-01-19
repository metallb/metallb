package ndp

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// HopLimit is the expected IPv6 hop limit for all NDP messages.
const HopLimit = 255

// A Conn is a Neighbor Discovery Protocol connection.
type Conn struct {
	pc *ipv6.PacketConn
	cm *ipv6.ControlMessage

	ifi  *net.Interface
	addr *net.IPAddr

	// Used only in tests:
	//
	// icmpTest disables the self-filtering mechanism in ReadFrom, and
	// udpTestPort enables the Conn to run over UDP for easier unprivileged
	// tests.
	icmpTest    bool
	udpTestPort int
}

// Dial dials a NDP connection using the specified interface and address type.
//
// As a special case, literal IPv6 addresses may be specified to bind to a
// specific address for an interface.  If the IPv6 address does not exist on
// the interface, an error will be returned.
//
// Dial returns a Conn and the chosen IPv6 address of the interface.
func Dial(ifi *net.Interface, addr Addr) (*Conn, net.IP, error) {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, nil, err
	}

	ipAddr, err := chooseAddr(addrs, ifi.Name, addr)
	if err != nil {
		return nil, nil, err
	}

	ic, err := icmp.ListenPacket("ip6:ipv6-icmp", ipAddr.String())
	if err != nil {
		return nil, nil, err
	}

	pc := ic.IPv6PacketConn()

	// Calculate and place ICMPv6 checksum at correct offset in all messages.
	const chkOff = 2
	if err := pc.SetChecksum(true, chkOff); err != nil {
		return nil, nil, err
	}

	return newConn(pc, ipAddr, ifi)
}

// newConn is an internal test constructor used for creating a Conn from an
// arbitrary ipv6.PacketConn.
func newConn(pc *ipv6.PacketConn, src *net.IPAddr, ifi *net.Interface) (*Conn, net.IP, error) {
	c := &Conn{
		pc: pc,

		// The default control message used when none is specified.
		cm: &ipv6.ControlMessage{
			HopLimit: HopLimit,
			Src:      src.IP,
			IfIndex:  ifi.Index,
		},

		ifi:  ifi,
		addr: src,
	}

	return c, src.IP, nil
}

// Close closes the Conn's underlying connection.
func (c *Conn) Close() error {
	return c.pc.Close()
}

// SetReadDeadline sets a deadline for the next NDP message to arrive.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.pc.SetReadDeadline(t)
}

// ReadFrom reads a Message from the Conn and returns its control message and
// source network address.  Messages sourced from this machine and malformed or
// unrecognized ICMPv6 messages are filtered.
func (c *Conn) ReadFrom() (Message, *ipv6.ControlMessage, net.IP, error) {
	b := make([]byte, c.ifi.MTU)
	for {
		n, cm, src, err := c.pc.ReadFrom(b)
		if err != nil {
			return nil, nil, nil, err
		}

		// Filter message if:
		//   - not testing the Conn implementation.
		//   - this address sent this message.
		ip := srcIP(src)
		if !c.test() && ip.Equal(c.addr.IP) {
			continue
		}

		// Filter any malformed and unrecognized messages.
		m, err := ParseMessage(b[:n])
		if err != nil {
			continue
		}

		return m, cm, ip, nil
	}
}

// WriteTo writes a Message to the Conn, with an optional control message and
// destination network address.
//
// If cm is nil, a default control message will be sent.
func (c *Conn) WriteTo(m Message, cm *ipv6.ControlMessage, dst net.IP) error {
	b, err := MarshalMessage(m)
	if err != nil {
		return err
	}

	// Set reasonable defaults if control message is nil.
	if cm == nil {
		cm = c.cm
	}

	_, err = c.pc.WriteTo(b, cm, c.dstAddr(dst, c.ifi.Name))
	return err
}

// dstAddr returns a different net.Addr type depending on if the Conn is
// configured for testing.
func (c *Conn) dstAddr(ip net.IP, zone string) net.Addr {
	if !c.test() {
		return &net.IPAddr{
			IP:   ip,
			Zone: zone,
		}
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: c.udpTestPort,
		Zone: c.ifi.Name,
	}
}

// test determines if Conn is configured for testing.
func (c *Conn) test() bool {
	return c.icmpTest || c.udpTestPort != 0
}

// srcIP retrieves the net.IP from possible net.Addr types used in a Conn.
func srcIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	default:
		panic(fmt.Sprintf("ndp: unhandled source net.Addr: %#v", addr))
	}
}
