package ndp

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

const (
	// Length of an ICMPv6 header.
	icmpLen = 4

	// Minimum byte length values for each type of valid Message.
	naLen = 20
	nsLen = 20
	raLen = 12
	rsLen = 4
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
	case ipv6.ICMPTypeRouterAdvertisement:
		m = new(RouterAdvertisement)
	case ipv6.ICMPTypeRouterSolicitation:
		m = new(RouterSolicitation)
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

var _ Message = &RouterAdvertisement{}

// A RouterAdvertisement is a Router Advertisement message as
// described in RFC 4861, Section 4.1.
type RouterAdvertisement struct {
	CurrentHopLimit      uint8
	ManagedConfiguration bool
	OtherConfiguration   bool
	RouterLifetime       time.Duration
	ReachableTime        time.Duration
	RetransmitTimer      time.Duration
	Options              []Option
}

func (ra *RouterAdvertisement) icmpType() ipv6.ICMPType { return ipv6.ICMPTypeRouterAdvertisement }

// MarshalBinary implements Message.
func (ra *RouterAdvertisement) MarshalBinary() ([]byte, error) {
	b := make([]byte, raLen)

	b[0] = ra.CurrentHopLimit

	if ra.ManagedConfiguration {
		b[1] |= (1 << 7)
	}
	if ra.OtherConfiguration {
		b[1] |= (1 << 6)
	}

	lifetime := ra.RouterLifetime.Seconds()
	binary.BigEndian.PutUint16(b[2:4], uint16(lifetime))

	reach := ra.ReachableTime / time.Millisecond
	binary.BigEndian.PutUint32(b[4:8], uint32(reach))

	retrans := ra.RetransmitTimer / time.Millisecond
	binary.BigEndian.PutUint32(b[8:12], uint32(retrans))

	ob, err := marshalOptions(ra.Options)
	if err != nil {
		return nil, err
	}

	b = append(b, ob...)

	return b, nil
}

// UnmarshalBinary implements Message.
func (ra *RouterAdvertisement) UnmarshalBinary(b []byte) error {
	if len(b) < raLen {
		return io.ErrUnexpectedEOF
	}

	// Skip message body for options.
	options, err := parseOptions(b[raLen:])
	if err != nil {
		return err
	}

	var (
		mFlag = (b[1] & 0x80) != 0
		oFlag = (b[1] & 0x40) != 0

		lifetime = time.Duration(binary.BigEndian.Uint16(b[2:4])) * time.Second
		reach    = time.Duration(binary.BigEndian.Uint32(b[4:8])) * time.Millisecond
		retrans  = time.Duration(binary.BigEndian.Uint32(b[8:12])) * time.Millisecond
	)

	*ra = RouterAdvertisement{
		CurrentHopLimit:      b[0],
		ManagedConfiguration: mFlag,
		OtherConfiguration:   oFlag,
		RouterLifetime:       lifetime,
		ReachableTime:        reach,
		RetransmitTimer:      retrans,
		Options:              options,
	}

	return nil
}

var _ Message = &RouterSolicitation{}

// A RouterSolicitation is a Router Solicitation message as
// described in RFC 4861, Section 4.1.
type RouterSolicitation struct {
	Options []Option
}

func (rs *RouterSolicitation) icmpType() ipv6.ICMPType { return ipv6.ICMPTypeRouterSolicitation }

// MarshalBinary implements Message.
func (rs *RouterSolicitation) MarshalBinary() ([]byte, error) {
	// b contains reserved area.
	b := make([]byte, rsLen)

	ob, err := marshalOptions(rs.Options)
	if err != nil {
		return nil, err
	}

	b = append(b, ob...)

	return b, nil
}

// UnmarshalBinary implements Message.
func (rs *RouterSolicitation) UnmarshalBinary(b []byte) error {
	if len(b) < rsLen {
		return io.ErrUnexpectedEOF
	}

	// Skip reserved area.
	options, err := parseOptions(b[rsLen:])
	if err != nil {
		return err
	}

	*rs = RouterSolicitation{
		Options: options,
	}

	return nil
}

// checkIPv6 verifies that ip is an IPv6 address.
func checkIPv6(ip net.IP) error {
	if ip.To16() == nil || ip.To4() != nil {
		return fmt.Errorf("ndp: invalid IPv6 address: %q", ip.String())
	}

	return nil
}
