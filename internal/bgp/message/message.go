package message

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"time"
)

type ErrorCode uint16

type openOptionType uint8

func Decode(r io.Reader) (interface{}, error) {
	msg := struct {
		Marker1 uint64
		Marker2 uint64
		Len     uint16
		Type    uint8
	}{}

	if err := binary.Read(r, binary.BigEndian, &msg); err != nil {
		return nil, err
	}

	if msg.Marker1 != 0xffffffffffffffff || msg.Marker2 != 0xffffffffffffffff {
		return nil, errors.New("invalid BGP message marker")
	}

	if msg.Len < 19 {
		return nil, fmt.Errorf("invalid BGP message length %d, must be at least 19 bytes", msg.Len)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(msg.Len) - 19,
	}

	var (
		ret interface{}
		err error
	)
	switch msg.Type {
	case 1:
		ret, err = decodeOpen(lr)

	case 2:
		ret, err = decodeUpdate(lr)

	case 3:
		ret, err = decodeNotification(lr)

	case 4:
		ret, err = &Keepalive{}, nil

	default:
		return nil, fmt.Errorf("unknown BGP message type %d", msg.Type)
	}

	if err != nil {
		return nil, err
	}
	if lr.N != 0 {
		return nil, fmt.Errorf("wrong message length %d, %d bytes left over", msg.Len, lr.N)
	}
	return ret, nil
}

type Open struct {
	ASN      uint32
	HoldTime time.Duration
	RouterID net.IP
}

func decodeOpen(r io.Reader) (*Open, error) {
	msg := struct {
		Version  uint8
		ASN      uint16
		HoldTime uint16
		RouterID [4]byte
		OptLen   uint8
	}{}
	if err := binary.Read(r, binary.BigEndian, &msg); err != nil {
		return nil, err
	}

	if msg.Version != 4 {
		return nil, fmt.Errorf("unknown BGP version %d", msg.Version)
	}
	if msg.ASN != 23456 {
		return nil, fmt.Errorf("unexpected 2-byte ASN %d, want AS_TRANS (23456)", msg.ASN)
	}
	if msg.HoldTime != 0 && msg.HoldTime < 3 {
		return nil, fmt.Errorf("invalid HoldTime %d, must be 0 or >=3", msg.HoldTime)
	}

	ret := &Open{
		HoldTime: time.Duration(msg.HoldTime) * time.Second,
		RouterID: net.IP(msg.RouterID[:]),
	}

	or := &io.LimitedReader{
		R: r,
		N: int64(msg.OptLen),
	}
	for {
		optHdr := struct {
			Type openOptionType
			Len  uint8
		}{}
		if err := binary.Read(or, binary.BigEndian, &optHdr); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if optHdr.Type != 65 {
			return nil, fmt.Errorf("unknown BGP OPEN option %d", optHdr.Type)
		}
		if optHdr.Len != 4 {
			return nil, fmt.Errorf("wrong option length (%d) for 4-byte ASN option", optHdr.Len)
		}
		if err := binary.Read(or, binary.BigEndian, &ret.ASN); err != nil {
			return nil, err
		}
	}
	if or.N != 0 {
		return nil, fmt.Errorf("wrong open options length %d, %d bytes left over", msg.OptLen, or.N)
	}

	if ret.ASN == 0 {
		return nil, errors.New("peer does not support 4-byte ASNs")
	}

	return ret, nil
}

func (m *Open) MarshalBinary() ([]byte, error) {
	if m.ASN == 0 {
		return nil, errors.New("ASN must be non-zero")
	}
	if m.HoldTime != 0 && m.HoldTime < 3*time.Second {
		return nil, fmt.Errorf("invalid hold time %s, must be zero or >=3s", m.HoldTime)
	}
	if m.RouterID.To4() == nil {
		return nil, fmt.Errorf("invalid Router ID %q, must be an IPv4 address", m.RouterID)
	}

	msg := struct {
		Marker1 uint64
		Marker2 uint64
		MsgLen  uint16
		Type    uint8

		Version   uint8
		ASTrans   uint16
		HoldTime  uint16
		RouterID  uint32
		OptLen    uint8
		ASN4bType uint8
		ASN4bLen  uint8
		ASN4b     uint32
	}{
		Marker1:   0xffffffffffffffff,
		Marker2:   0xffffffffffffffff,
		MsgLen:    0, // filled below
		Type:      1, // BGP OPEN
		Version:   4,
		ASTrans:   23456,
		HoldTime:  uint16(m.HoldTime.Seconds()),
		RouterID:  binary.BigEndian.Uint32(m.RouterID.To4()),
		OptLen:    6,
		ASN4bType: 65,
		ASN4bLen:  4,
		ASN4b:     uint32(m.ASN),
	}
	// TODO: fairly sure the encoding for 4b ASN is wrong, needs to be
	// nested in a capability statement.

	msg.MsgLen = uint16(binary.Size(msg))

	var b bytes.Buffer
	if err := binary.Write(&b, binary.BigEndian, msg); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type Keepalive struct{}

func (m *Keepalive) MarshalBinary() ([]byte, error) {
	return []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // Marker
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // Marker (contd)
		0, 19, // Length
		4, // Type
	}, nil
}

type Notification struct {
	Code ErrorCode
	Data []byte
}

func decodeNotification(r io.Reader) (*Notification, error) {
	var code ErrorCode
	if err := binary.Read(r, binary.BigEndian, &code); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return &Notification{
		Code: code,
		Data: data,
	}, nil
}

func (m *Notification) MarshalBinary() ([]byte, error) {
	msg := struct {
		Marker1 uint64
		Marker2 uint64
		MsgLen  uint16
		Type    uint8
		Code    ErrorCode
	}{
		Marker1: 0xffffffffffffffff,
		Marker2: 0xffffffffffffffff,
		MsgLen:  uint16(21 + len(m.Data)),
		Type:    3, // BGP NOTIFICATION
		Code:    m.Code,
	}
	var b bytes.Buffer
	if err := binary.Write(&b, binary.BigEndian, msg); err != nil {
		return nil, err
	}

	if _, err := b.Write(m.Data); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

type Attribute struct {
	Code uint16
	Data []byte
}

type Update struct {
	Withdraw   []*net.IPNet
	Advertise  []*net.IPNet
	Attributes []Attribute
}

func decodeUpdate(r io.Reader) (*Update, error) {
	var len uint16
	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return nil, err
	}
	wdr, err := decodePrefixes(&io.LimitedReader{
		R: r,
		N: int64(len),
	})
	if err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return nil, err
	}
	attrs, err := decodePathAttrs(&io.LimitedReader{
		R: r,
		N: int64(len),
	})
	if err != nil {
		return nil, err
	}

	adv, err := decodePrefixes(r)
	if err != nil {
		return nil, err
	}

	return &Update{
		Withdraw:   wdr,
		Advertise:  adv,
		Attributes: attrs,
	}, nil
}

func (m *Update) MarshalBinary() ([]byte, error) {
	msg := struct {
		Marker1 uint64
		Marker2 uint64
		MsgLen  uint16
		Type    uint8
	}{
		Marker1: 0xffffffffffffffff,
		Marker2: 0xffffffffffffffff,
		Type:    2, // BGP UPDATE
	}

	wdr, err := encodePrefixes(m.Withdraw)
	if err != nil {
		return nil, err
	}

	adv, err := encodePrefixes(m.Advertise)
	if err != nil {
		return nil, err
	}

	attrs, err := encodePathAttrs(m.Attributes)
	if err != nil {
		return nil, err
	}

	msg.MsgLen = uint16(4 + binary.Size(msg) + len(wdr) + len(adv) + len(attrs))

	var b bytes.Buffer
	if err := binary.Write(&b, binary.BigEndian, msg); err != nil {
		return nil, err
	}

	l := uint16(len(wdr))
	if err := binary.Write(&b, binary.BigEndian, l); err != nil {
		return nil, err
	}
	if _, err := b.Write(wdr); err != nil {
		return nil, err
	}

	l = uint16(len(attrs))
	if err := binary.Write(&b, binary.BigEndian, l); err != nil {
		return nil, err
	}
	if _, err := b.Write(attrs); err != nil {
		return nil, err
	}

	if _, err := b.Write(adv); err != nil {
		return nil, err
	}

	return b.Bytes(), err
}

func decodePrefixes(r io.Reader) ([]*net.IPNet, error) {
	var ret []*net.IPNet
	for {
		var pfxLen uint8
		if err := binary.Read(r, binary.BigEndian, &pfxLen); err != nil {
			if err == io.EOF {
				// Clean EOF at a prefix boundary, the list is finished.
				return ret, nil
			}
			return nil, err
		}
		if pfxLen > 32 {
			return nil, fmt.Errorf("invalid prefix length %d, must be between 0 and 32", pfxLen)
		}
		ip := make(net.IP, 4)
		blen := pfxLen / 8
		if pfxLen%8 != 0 {
			blen++
		}
		if _, err := io.ReadFull(r, ip[:blen]); err != nil {
			return nil, err
		}
		m := net.CIDRMask(int(pfxLen), 32)
		if !ip.Equal(ip.Mask(m)) {
			// Note: strictly, the BGP spec says the value of the
			// masked bits is "irrelevant", but this makes UPDATE
			// parsing non-idempotent. In practice, I struggle to
			// think of a sane implementation that would not clear
			// these bits, so I'm declaring that it's an error until
			// something breaks.
			return nil, fmt.Errorf("invalid CIDR prefix %s/%d, IP has non-zero masked bits", ip, pfxLen)
		}
		ret = append(ret, &net.IPNet{
			IP:   ip,
			Mask: m,
		})
	}
}

func encodePrefixes(nets []*net.IPNet) ([]byte, error) {
	var b bytes.Buffer
	for _, n := range nets {
		o, _ := n.Mask.Size()
		b.WriteByte(byte(o))
		bytes := o / 8
		if o%8 != 0 {
			bytes++
		}
		b.Write(n.IP.To4()[:bytes])
	}
	return b.Bytes(), nil
}

func decodePathAttrs(r io.Reader) ([]Attribute, error) {
	var ret []Attribute
	for {
		var code uint16
		if err := binary.Read(r, binary.BigEndian, &code); err != nil {
			if err == io.EOF {
				return ret, nil
			}
			return nil, err
		}

		var l uint16
		if code&0x1000 == 0 {
			var l8 uint8
			if err := binary.Read(r, binary.BigEndian, &l8); err != nil {
				return nil, err
			}
			l = uint16(l8)
		} else {
			if err := binary.Read(r, binary.BigEndian, &l); err != nil {
				return nil, err
			}
		}

		bs := make([]byte, l)
		if _, err := io.ReadFull(r, bs); err != nil {
			return nil, err
		}

		ret = append(ret, Attribute{code, bs})
	}
}

func encodePathAttrs(attrs []Attribute) ([]byte, error) {
	var b bytes.Buffer
	for _, attr := range attrs {
		if err := binary.Write(&b, binary.BigEndian, attr.Code); err != nil {
			return nil, err
		}
		// TODO: am I supposed to figure out the size and decide how to set this flag myself?
		if attr.Code&0x1000 == 0 {
			if err := binary.Write(&b, binary.BigEndian, uint8(len(attr.Data))); err != nil {
				return nil, err
			}
		} else {
			if err := binary.Write(&b, binary.BigEndian, uint16(len(attr.Data))); err != nil {
				return nil, err
			}
		}
		if _, err := b.Write(attr.Data); err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}
