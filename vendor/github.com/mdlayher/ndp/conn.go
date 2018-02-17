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

// JoinGroup joins the specified multicast group.
func (c *Conn) JoinGroup(group net.IP) error {
	return c.pc.JoinGroup(c.ifi, &net.IPAddr{
		IP:   group,
		Zone: c.ifi.Name,
	})
}

// LeaveGroup leaves the specified multicast group.
func (c *Conn) LeaveGroup(group net.IP) error {
	return c.pc.LeaveGroup(c.ifi, &net.IPAddr{
		IP:   group,
		Zone: c.ifi.Name,
	})
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

// SolicitedNodeMulticast returns the solicited-node multicast address for
// an IPv6 address.
func SolicitedNodeMulticast(ip net.IP) (net.IP, error) {
	if err := checkIPv6(ip); err != nil {
		return nil, err
	}

	// Fixed prefix, and low 24 bits taken from input address.
	snm := net.ParseIP("ff02::1:ff00:0")
	for i := 13; i < 16; i++ {
		snm[i] = ip[i]
	}

	return snm, nil
}

// TestConns sets up a pair of testing NDP peer Conns over UDP using the
// specified interface, and returns the address which can be used to send
// messages between them.
//
// TestConns is useful for environments and tests which do not allow direct
// ICMPv6 communications.
func TestConns(ifi *net.Interface) (*Conn, *Conn, net.IP, error) {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ndp: failed to get interface %q addresses: %v", ifi.Name, err)
	}

	addr, err := chooseAddr(addrs, ifi.Name, LinkLocal)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ndp: failed to find link-local address for %q: %v", ifi.Name, err)
	}

	// Create two UDPv6 connections and instruct them to communicate
	// with each other for Conn tests.
	c1, p1, err := udpConn(addr, ifi)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ndp: failed to set up first test connection: %v", err)
	}

	c2, p2, err := udpConn(addr, ifi)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ndp: failed to set up second test connection: %v", err)
	}

	c1.udpTestPort = p2
	c2.udpTestPort = p1

	return c1, c2, addr.IP, nil
}

// udpConn creates a single test Conn over UDP, and returns the port used to
// send messages to it.
func udpConn(addr *net.IPAddr, ifi *net.Interface) (*Conn, int, error) {
	laddr := &net.UDPAddr{
		IP: addr.IP,
		// Port ommitted so it will be assigned automatically.
		Zone: addr.Zone,
	}

	uc, err := net.ListenUDP("udp6", laddr)
	if err != nil {
		return nil, 0, fmt.Errorf("ndp: failed to listen UDPv6: %v", err)
	}

	pc := ipv6.NewPacketConn(uc)

	c, _, err := newConn(pc, addr, ifi)
	if err != nil {
		return nil, 0, fmt.Errorf("ndp: failed to create test NDP conn: %v", err)
	}

	return c, uc.LocalAddr().(*net.UDPAddr).Port, nil
}
