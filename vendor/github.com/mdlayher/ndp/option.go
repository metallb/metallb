package ndp

import (
	"encoding"
	"fmt"
	"io"
	"net"
)

const (
	// Length of a link-layer address for Ethernet networks.
	ethAddrLen = 6

	// The assumed NDP option length (in units of 8 bytes) for a source or
	// target link layer address option for Ethernet networks.
	llaOptLen = 1

	// Type values for each type of valid Option.
	optSourceLLA = 1
	optTargetLLA = 2

	// Minimum byte length values for each type of valid Option.
	llaByteLen = 8
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
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	code() uint8
}

var _ Option = &LinkLayerAddress{}

// A LinkLayerAddress is a Source or Target Link-Layer Address option, as
// described in RFC 4861, Section 4.6.1.
type LinkLayerAddress struct {
	Direction Direction
	Addr      net.HardwareAddr
}

// TODO(mdlayher): deal with non-ethernet links and variable option length?

func (lla *LinkLayerAddress) code() byte { return byte(lla.Direction) }

// MarshalBinary implements Option.
func (lla *LinkLayerAddress) MarshalBinary() ([]byte, error) {
	if d := lla.Direction; d != Source && d != Target {
		return nil, fmt.Errorf("ndp: invalid link-layer address direction: %d", d)
	}

	if len(lla.Addr) != ethAddrLen {
		return nil, fmt.Errorf("ndp: invalid link-layer address: %q", lla.Addr.String())
	}

	b := make([]byte, llaByteLen)
	b[0] = lla.code()
	b[1] = llaOptLen
	copy(b[2:], lla.Addr)

	return b, nil
}

// UnmarshalBinary implements Option.
func (lla *LinkLayerAddress) UnmarshalBinary(b []byte) error {
	if len(b) < llaByteLen {
		return io.ErrUnexpectedEOF
	}

	d := Direction(b[0])
	if d != Source && d != Target {
		return fmt.Errorf("ndp: invalid link-layer address direction: %d", d)
	}

	if l := b[1]; l != llaOptLen {
		return fmt.Errorf("ndp: unexpected link-layer address option length: %d", l)
	}

	*lla = LinkLayerAddress{
		Direction: d,
		Addr:      make(net.HardwareAddr, ethAddrLen),
	}

	copy(lla.Addr, b[2:])

	return nil
}

// marshalOptions marshals a slice of Options into a single byte slice.
func marshalOptions(options []Option) ([]byte, error) {
	var b []byte
	for _, o := range options {
		ob, err := o.MarshalBinary()
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

		// Infer the option from its type value and use it for unmarshaling.
		var o Option
		switch t {
		case optSourceLLA, optTargetLLA:
			o = new(LinkLayerAddress)
		default:
			return nil, fmt.Errorf("ndp: unrecognized NDP option type: %d", t)
		}

		// Unmarshal at the current offset, up to the expected length.
		if err := o.UnmarshalBinary(b[i : i+l]); err != nil {
			return nil, err
		}

		// Verify that we won't advance beyond the end of the byte slice, and
		// Advance to the next option's type field.
		if i+l > len(b[i:]) {
			return nil, io.ErrUnexpectedEOF
		}
		i += l

		options = append(options, o)
	}

	return options, nil
}
