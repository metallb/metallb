package ndp

import (
	"fmt"
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

// HopLimit is the expected IPv6 hop limit for all NDP messages.
const HopLimit = 255

// A Conn is a Neighbor Discovery Protocol connection.
type Conn struct {
	pc *ipv6.PacketConn
	cm *ipv6.ControlMessage

	allNodes *net.IPAddr
	llAddr   *net.IPAddr
}

// Dial dials a NDP connection using the specified interface.  It returns
// a Conn and the link-local IPv6 address of the interface.
func Dial(ifi *net.Interface) (*Conn, net.IP, error) {
	llAddr, err := linkLocalAddr(ifi)
	if err != nil {
		return nil, nil, err
	}

	ic, err := icmp.ListenPacket("ip6:ipv6-icmp", llAddr.String())
	if err != nil {
		return nil, nil, err
	}

	// Join the "all nodes" multicast group for this interface.
	allNodes := &net.IPAddr{
		IP:   net.IPv6linklocalallnodes,
		Zone: ifi.Name,
	}

	pc := ic.IPv6PacketConn()
	if err := pc.JoinGroup(ifi, allNodes); err != nil {
		return nil, nil, err
	}

	// Calculate and place ICMPv6 checksum at correct offset in all messages.
	const chkOff = 2
	if err := pc.SetChecksum(true, chkOff); err != nil {
		return nil, nil, err
	}

	c := &Conn{
		pc: pc,

		// The default control message used when none is specified.
		cm: &ipv6.ControlMessage{
			HopLimit: HopLimit,
			Src:      llAddr.IP,
			IfIndex:  ifi.Index,
		},

		allNodes: allNodes,
		llAddr:   llAddr,
	}

	return c, llAddr.IP, nil
}

// Close closes the Conn's underlying connection.
func (c *Conn) Close() error {
	return c.pc.Close()
}

// ReadFrom reads a Message from the Conn and returns its control message and
// source network address.  Messages sourced from this machine and malformed or
// unrecognized ICMPv6 messages are filtered.
func (c *Conn) ReadFrom() (Message, *ipv6.ControlMessage, net.IP, error) {
	b := make([]byte, 1280)
	for {
		n, cm, src, err := c.pc.ReadFrom(b)
		if err != nil {
			return nil, nil, nil, err
		}

		// Did this machine send this message?
		ip := src.(*net.IPAddr).IP
		if ip.Equal(c.llAddr.IP) {
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

	// Set reasonable defaults if control message or destination are nil.
	if cm == nil {
		cm = c.cm
	}

	addr := &net.IPAddr{IP: dst}
	_, err = c.pc.WriteTo(b, cm, addr)
	return err
}

// linkLocalAddr searches for a valid IPv6 link-local address for the specified
// interface.
func linkLocalAddr(ifi *net.Interface) (*net.IPAddr, error) {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}

	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		if err := checkIPv6(ipn.IP); err != nil {
			continue
		}

		if !ipn.IP.IsLinkLocalUnicast() {
			continue
		}

		return &net.IPAddr{
			IP:   ipn.IP,
			Zone: ifi.Name,
		}, nil
	}

	return nil, fmt.Errorf("ndp: no link local address for interface %q", ifi.Name)
}
