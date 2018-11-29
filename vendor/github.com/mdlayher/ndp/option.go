package ndp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Infinity indicates that a prefix is valid for an infinite amount of time,
// unless a new, finite, value is received in a subsequent router advertisement.
const Infinity = time.Duration(0xffffffff) * time.Second

const (
	// Length of a link-layer address for Ethernet networks.
	ethAddrLen = 6

	// The assumed NDP option length (in units of 8 bytes) for fixed length options.
	llaOptLen = 1
	piOptLen  = 4
	mtuOptLen = 1

	// Type values for each type of valid Option.
	optSourceLLA         = 1
	optTargetLLA         = 2
	optPrefixInformation = 3
	optMTU               = 5
	optRDNSS             = 25
)

// A Direction specifies the direction of a LinkLayerAddress Option as a source
// or target.
type Direction int

// Possible Direction values.
const (
	Source Direction = optSourceLLA
	Target Direction = optTargetLLA
)

// An Option is a Neighbor Discovery Protocol option.
type Option interface {
	// Code specifies the NDP option code for an Option.
	Code() uint8

	// "Code" as a method name isn't actually accurate because NDP options
	// also refer to that field as "Type", but we want to avoid confusion
	// with Message implementations which already use Type.

	// Called when dealing with a Message's Options.
	marshal() ([]byte, error)
	unmarshal(b []byte) error
}

var _ Option = &LinkLayerAddress{}

// A LinkLayerAddress is a Source or Target Link-Layer Address option, as
// described in RFC 4861, Section 4.6.1.
type LinkLayerAddress struct {
	Direction Direction
	Addr      net.HardwareAddr
}

// TODO(mdlayher): deal with non-ethernet links and variable option length?

// Code implements Option.
func (lla *LinkLayerAddress) Code() byte { return byte(lla.Direction) }

func (lla *LinkLayerAddress) marshal() ([]byte, error) {
	if d := lla.Direction; d != Source && d != Target {
		return nil, fmt.Errorf("ndp: invalid link-layer address direction: %d", d)
	}

	if len(lla.Addr) != ethAddrLen {
		return nil, fmt.Errorf("ndp: invalid link-layer address: %q", lla.Addr.String())
	}

	raw := &RawOption{
		Type:   lla.Code(),
		Length: llaOptLen,
		Value:  lla.Addr,
	}

	return raw.marshal()
}

func (lla *LinkLayerAddress) unmarshal(b []byte) error {
	raw := new(RawOption)
	if err := raw.unmarshal(b); err != nil {
		return err
	}

	d := Direction(raw.Type)
	if d != Source && d != Target {
		return fmt.Errorf("ndp: invalid link-layer address direction: %d", d)
	}

	if l := raw.Length; l != llaOptLen {
		return fmt.Errorf("ndp: unexpected link-layer address option length: %d", l)
	}

	*lla = LinkLayerAddress{
		Direction: d,
		Addr:      net.HardwareAddr(raw.Value),
	}

	return nil
}

var _ Option = new(MTU)

// TODO(mdlayher): decide if this should just be a struct type instead.

// An MTU is an MTU option, as described in RFC 4861, Section 4.6.1.
type MTU uint32

// NewMTU creates an MTU Option from an MTU value.
func NewMTU(mtu uint32) *MTU {
	m := MTU(mtu)
	return &m
}

// Code implements Option.
func (m *MTU) Code() byte { return optMTU }

func (m *MTU) marshal() ([]byte, error) {
	raw := &RawOption{
		Type:   m.Code(),
		Length: mtuOptLen,
		// 2 reserved bytes, 4 for MTU.
		Value: make([]byte, 6),
	}

	binary.BigEndian.PutUint32(raw.Value[2:6], uint32(*m))

	return raw.marshal()
}

func (m *MTU) unmarshal(b []byte) error {
	raw := new(RawOption)
	if err := raw.unmarshal(b); err != nil {
		return err
	}

	*m = MTU(binary.BigEndian.Uint32(raw.Value[2:6]))

	return nil
}

var _ Option = &PrefixInformation{}

// A PrefixInformation is a a Prefix Information option, as described in RFC 4861, Section 4.6.1.
type PrefixInformation struct {
	PrefixLength                   uint8
	OnLink                         bool
	AutonomousAddressConfiguration bool
	ValidLifetime                  time.Duration
	PreferredLifetime              time.Duration
	Prefix                         net.IP
}

// Code implements Option.
func (pi *PrefixInformation) Code() byte { return optPrefixInformation }

func (pi *PrefixInformation) marshal() ([]byte, error) {
	// Per the RFC:
	// "The bits in the prefix after the prefix length are reserved and MUST
	// be initialized to zero by the sender and ignored by the receiver."
	//
	// Therefore, any prefix, when masked with its specified length, should be
	// identical to the prefix itself for it to be valid.
	mask := net.CIDRMask(int(pi.PrefixLength), 128)
	if masked := pi.Prefix.Mask(mask); !pi.Prefix.Equal(masked) {
		return nil, fmt.Errorf("ndp: invalid prefix information: %s/%d", pi.Prefix.String(), pi.PrefixLength)
	}

	raw := &RawOption{
		Type:   pi.Code(),
		Length: piOptLen,
		// 30 bytes for PrefixInformation body.
		Value: make([]byte, 30),
	}

	raw.Value[0] = pi.PrefixLength

	if pi.OnLink {
		raw.Value[1] |= (1 << 7)
	}
	if pi.AutonomousAddressConfiguration {
		raw.Value[1] |= (1 << 6)
	}

	valid := pi.ValidLifetime.Seconds()
	binary.BigEndian.PutUint32(raw.Value[2:6], uint32(valid))

	pref := pi.PreferredLifetime.Seconds()
	binary.BigEndian.PutUint32(raw.Value[6:10], uint32(pref))

	// 4 bytes reserved.

	copy(raw.Value[14:30], pi.Prefix)

	return raw.marshal()
}

func (pi *PrefixInformation) unmarshal(b []byte) error {
	raw := new(RawOption)
	if err := raw.unmarshal(b); err != nil {
		return err
	}

	// Guard against incorrect option length.
	if raw.Length != piOptLen {
		return io.ErrUnexpectedEOF
	}

	var (
		oFlag = (raw.Value[1] & 0x80) != 0
		aFlag = (raw.Value[1] & 0x40) != 0

		valid     = time.Duration(binary.BigEndian.Uint32(raw.Value[2:6])) * time.Second
		preferred = time.Duration(binary.BigEndian.Uint32(raw.Value[6:10])) * time.Second
	)

	// Skip reserved area.
	addr := net.IP(raw.Value[14:30])
	if err := checkIPv6(addr); err != nil {
		return err
	}

	// Per the RFC, bits in prefix past prefix length are ignored by the
	// receiver.
	l := raw.Value[0]
	mask := net.CIDRMask(int(l), 128)
	addr = addr.Mask(mask)

	*pi = PrefixInformation{
		PrefixLength: l,
		OnLink:       oFlag,
		AutonomousAddressConfiguration: aFlag,
		ValidLifetime:                  valid,
		PreferredLifetime:              preferred,
		// raw.Value is already a copy of b, so just point to the address.
		Prefix: addr,
	}

	return nil
}

// A RecursiveDNSServer is a Recursive DNS Server option, as described in
// RFC 6106, Section 5.1.
type RecursiveDNSServer struct {
	Lifetime time.Duration
	Servers  []net.IP
}

// Code implements Option.
func (r *RecursiveDNSServer) Code() byte { return optRDNSS }

// Offsets for the RDNSS option.
const (
	rdnssLifetimeOff = 2
	rdnssServersOff  = 6
)

var (
	errRDNSSNoServers = errors.New("ndp: recursive DNS server option requires at least one server")
	errRDNSSBadServer = errors.New("ndp: recursive DNS server option has malformed IPv6 address")
)

func (r *RecursiveDNSServer) marshal() ([]byte, error) {
	slen := len(r.Servers)
	if slen == 0 {
		return nil, errRDNSSNoServers
	}

	raw := &RawOption{
		Type: r.Code(),
		// Always have one length unit to start, and then each IPv6 address
		// occupies two length units.
		Length: 1 + uint8((slen * 2)),
		// Allocate enough space for all data.
		Value: make([]byte, rdnssServersOff+(slen*net.IPv6len)),
	}

	binary.BigEndian.PutUint32(
		raw.Value[rdnssLifetimeOff:rdnssServersOff],
		uint32(r.Lifetime.Seconds()),
	)

	for i := 0; i < len(r.Servers); i++ {
		// Determine the start and end byte offsets for each address,
		// effectively iterating 16 bytes at a time to insert an address.
		var (
			start = rdnssServersOff + (i * net.IPv6len)
			end   = rdnssServersOff + net.IPv6len + (i * net.IPv6len)
		)

		copy(raw.Value[start:end], r.Servers[i])
	}

	return raw.marshal()
}

func (r *RecursiveDNSServer) unmarshal(b []byte) error {
	raw := new(RawOption)
	if err := raw.unmarshal(b); err != nil {
		return err
	}

	// Skip 2 reserved bytes to get lifetime.
	lt := time.Duration(binary.BigEndian.Uint32(
		raw.Value[rdnssLifetimeOff:rdnssServersOff])) * time.Second

	// Determine the number of DNS servers specified using the method described
	// in the RFC.  Remember, length is specified in units of 8 octets.
	//
	// "That is, the number of addresses is equal to (Length - 1) / 2."
	//
	// Make sure at least one server is present, and that the IPv6 addresses are
	// the expected 16 byte length.
	dividend := (int(raw.Length) - 1)
	if dividend%2 != 0 {
		return errRDNSSBadServer
	}

	count := dividend / 2
	if count == 0 {
		return errRDNSSNoServers
	}

	servers := make([]net.IP, 0, count)
	for i := 0; i < count; i++ {
		// Determine the start and end byte offsets for each address,
		// effectively iterating 16 bytes at a time to fetch an address.
		var (
			start = rdnssServersOff + (i * net.IPv6len)
			end   = rdnssServersOff + net.IPv6len + (i * net.IPv6len)
		)

		// The RawOption already made a copy of this data, so convert it
		// directly to an IPv6 address with no further copying needed.
		servers = append(servers, net.IP(raw.Value[start:end]))
	}

	*r = RecursiveDNSServer{
		Lifetime: lt,
		Servers:  servers,
	}

	return nil
}

var _ Option = &RawOption{}

// A RawOption is an Option in its raw and unprocessed format.  Options which
// are not recognized by this package can be represented using a RawOption.
type RawOption struct {
	Type   uint8
	Length uint8
	Value  []byte
}

// Code implements Option.
func (u *RawOption) Code() byte { return u.Type }

func (u *RawOption) marshal() ([]byte, error) {
	// Length specified in units of 8 bytes, and the caller must provide
	// an accurate length.
	l := int(u.Length * 8)
	if 1+1+len(u.Value) != l {
		return nil, io.ErrUnexpectedEOF
	}

	b := make([]byte, u.Length*8)
	b[0] = u.Type
	b[1] = u.Length

	copy(b[2:], u.Value)

	return b, nil
}

func (u *RawOption) unmarshal(b []byte) error {
	if len(b) < 2 {
		return io.ErrUnexpectedEOF
	}

	u.Type = b[0]
	u.Length = b[1]
	// Exclude type and length fields from value's length.
	l := int(u.Length*8) - 2

	// Enforce a valid length value that matches the expected one.
	if lb := len(b[2:]); l != lb {
		return fmt.Errorf("ndp: option value byte length should be %d, but length is %d", l, lb)
	}

	u.Value = make([]byte, l)
	copy(u.Value, b[2:])

	return nil
}

// marshalOptions marshals a slice of Options into a single byte slice.
func marshalOptions(options []Option) ([]byte, error) {
	var b []byte
	for _, o := range options {
		ob, err := o.marshal()
		if err != nil {
			return nil, err
		}

		b = append(b, ob...)
	}

	return b, nil
}

// parseOptions parses a slice of Options from a byte slice.
func parseOptions(b []byte) ([]Option, error) {
	var options []Option
	for i := 0; len(b[i:]) != 0; {
		// Two bytes: option type and option length.
		if len(b[i:]) < 2 {
			return nil, io.ErrUnexpectedEOF
		}

		// Type processed as-is, but length is stored in units of 8 bytes,
		// so expand it to the actual byte length.
		t := b[i]
		l := int(b[i+1]) * 8

		// Verify that we won't advance beyond the end of the byte slice.
		if l > len(b[i:]) {
			return nil, io.ErrUnexpectedEOF
		}

		// Infer the option from its type value and use it for unmarshaling.
		var o Option
		switch t {
		case optSourceLLA, optTargetLLA:
			o = new(LinkLayerAddress)
		case optMTU:
			o = new(MTU)
		case optPrefixInformation:
			o = new(PrefixInformation)
		case optRDNSS:
			o = new(RecursiveDNSServer)
		default:
			o = new(RawOption)
		}

		// Unmarshal at the current offset, up to the expected length.
		if err := o.unmarshal(b[i : i+l]); err != nil {
			return nil, err
		}

		// Advance to the next option's type field.
		i += l

		options = append(options, o)
	}

	return options, nil
}
