package ndp

import (
	"encoding"
	"fmt"
	"io"
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

const (
	// Length of an ICMPv6 header.
	icmpLen = 4

	// Minimum byte length values for each type of valid Message.
	naLen = 20
	nsLen = 20
)

// A Message is a Neighbor Discovery Protocol message.
type Message interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	icmpType() ipv6.ICMPType
}

// MarshalMessage marshals a Message into its binary form and prepends an
// ICMPv6 message with the correct type.
//
// It is assumed that the operating system or caller will calculate and place
// the ICMPv6 checksum in the result.
func MarshalMessage(m Message) ([]byte, error) {
	mb, err := m.MarshalBinary()
	if err != nil {
		return nil, err
	}

	im := icmp.Message{
		Type: m.icmpType(),
		// Always zero.
		Code: 0,
		// Calculated by caller or OS.
		Checksum: 0,
		Body: &icmp.DefaultMessageBody{
			Data: mb,
		},
	}

	// Pseudo-header always nil so checksum is calculated by caller or OS.
	return im.Marshal(nil)
}

// ParseMessage parses a Message from its binary form after determining its
// type from a leading ICMPv6 message.
func ParseMessage(b []byte) (Message, error) {
	if len(b) < icmpLen {
		return nil, io.ErrUnexpectedEOF
	}

	// TODO(mdlayher): verify checksum?

	var m Message
	switch t := ipv6.ICMPType(b[0]); t {
	case ipv6.ICMPTypeNeighborAdvertisement:
		m = new(NeighborAdvertisement)
	case ipv6.ICMPTypeNeighborSolicitation:
		m = new(NeighborSolicitation)
	default:
		return nil, fmt.Errorf("ndp: unrecognized ICMPv6 type: %d", t)
	}

	if err := m.UnmarshalBinary(b[icmpLen:]); err != nil {
		return nil, err
	}

	return m, nil
}

var _ Message = &NeighborAdvertisement{}

// A NeighborAdvertisement is a Neighbor Advertisement message as
// described in RFC 4861, Section 4.4.
type NeighborAdvertisement struct {
	Router        bool
	Solicited     bool
	Override      bool
	TargetAddress net.IP
	Options       []Option
}

func (na *NeighborAdvertisement) icmpType() ipv6.ICMPType { return ipv6.ICMPTypeNeighborAdvertisement }

// MarshalBinary implements Message.
func (na *NeighborAdvertisement) MarshalBinary() ([]byte, error) {
	if err := checkIPv6(na.TargetAddress); err != nil {
		return nil, err
	}

	b := make([]byte, naLen)

	if na.Router {
		b[0] |= (1 << 7)
	}
	if na.Solicited {
		b[0] |= (1 << 6)
	}
	if na.Override {
		b[0] |= (1 << 5)
	}

	copy(b[4:], na.TargetAddress)

	ob, err := marshalOptions(na.Options)
	if err != nil {
		return nil, err
	}

	b = append(b, ob...)

	return b, nil
}

// UnmarshalBinary implements Message.
func (na *NeighborAdvertisement) UnmarshalBinary(b []byte) error {
	if len(b) < naLen {
		return io.ErrUnexpectedEOF
	}

	// Skip flags and reserved area.
	addr := b[4:naLen]
	if err := checkIPv6(addr); err != nil {
		return err
	}

	options, err := parseOptions(b[naLen:])
	if err != nil {
		return err
	}

	*na = NeighborAdvertisement{
		Router:    (b[0] & 0x80) != 0,
		Solicited: (b[0] & 0x40) != 0,
		Override:  (b[0] & 0x20) != 0,

		TargetAddress: make(net.IP, net.IPv6len),

		Options: options,
	}

	copy(na.TargetAddress, addr)

	return nil
}

var _ Message = &NeighborSolicitation{}

// A NeighborSolicitation is a Neighbor Solicitation message as
// described in RFC 4861, Section 4.3.
type NeighborSolicitation struct {
	TargetAddress net.IP
	Options       []Option
}

func (ns *NeighborSolicitation) icmpType() ipv6.ICMPType { return ipv6.ICMPTypeNeighborSolicitation }

// MarshalBinary implements Message.
func (ns *NeighborSolicitation) MarshalBinary() ([]byte, error) {
	if err := checkIPv6(ns.TargetAddress); err != nil {
		return nil, err
	}

	b := make([]byte, naLen)
	copy(b[4:], ns.TargetAddress)

	ob, err := marshalOptions(ns.Options)
	if err != nil {
		return nil, err
	}

	b = append(b, ob...)

	return b, nil
}

// UnmarshalBinary implements Message.
func (ns *NeighborSolicitation) UnmarshalBinary(b []byte) error {
	if len(b) < nsLen {
		return io.ErrUnexpectedEOF
	}

	// Skip reserved area.
	addr := b[4:nsLen]
	if err := checkIPv6(addr); err != nil {
		return err
	}

	options, err := parseOptions(b[nsLen:])
	if err != nil {
		return err
	}

	*ns = NeighborSolicitation{
		TargetAddress: make(net.IP, net.IPv6len),

		Options: options,
	}

	copy(ns.TargetAddress, addr)

	return nil
}

// checkIPv6 verifies that ip is an IPv6 address.
func checkIPv6(ip net.IP) error {
	if ip.To16() == nil || ip.To4() != nil {
		return fmt.Errorf("ndp: invalid IPv6 address: %q", ip.String())
	}

	return nil
}
