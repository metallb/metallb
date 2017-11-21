package message

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type Open struct {
	ASN          uint16
	HoldTime     time.Duration
	RouterID     net.IP
	Capabilities []Capability
}

func decodeOpen(r io.Reader) (*Open, error) {
	msg := struct {
		Version  uint8
		ASN      uint16
		HoldTime uint16
		RouterID [4]byte
		OptLen   uint8
		OptType  uint8
		CapLen   uint8
	}{}
	if err := binary.Read(r, binary.BigEndian, &msg); err != nil {
		return nil, err
	}

	if msg.Version != 4 {
		return nil, fmt.Errorf("unknown BGP version %d", msg.Version)
	}
	if msg.ASN == 0 {
		return nil, errors.New("invalid ASN 0")
	}
	if msg.HoldTime != 0 && msg.HoldTime < 3 {
		return nil, fmt.Errorf("invalid HoldTime %d, must be 0 or >=3", msg.HoldTime)
	}
	if msg.OptType != 2 {
		return nil, fmt.Errorf("unknown BGP OPEN option %d", msg.OptType)
	}

	lr := &io.LimitedReader{
		R: r,
		N: int64(msg.CapLen),
	}
	caps, err := decodeCapabilities(lr)
	if err != nil {
		return nil, err
	}
	if lr.N != 0 {
		return nil, fmt.Errorf("%d trailing garbage bytes left after capabilities", lr.N)
	}

	return &Open{
		ASN:          msg.ASN,
		HoldTime:     time.Duration(msg.HoldTime) * time.Second,
		RouterID:     net.IP(msg.RouterID[:]),
		Capabilities: caps,
	}, nil
}

func (o *Open) MarshalBinary() ([]byte, error) {
	if o.ASN == 0 {
		return nil, errors.New("invalid ASN 0")
	}
	if o.HoldTime != 0 && o.HoldTime < 3*time.Second {
		return nil, errors.New("invalid HoldTime")
	}
	if o.RouterID.To4() == nil {
		return nil, fmt.Errorf("invalid RouterID %q", o.RouterID)
	}

	caps := encodeCapabilities(o.Capabilities)

	msg := struct {
		Version  uint8
		ASN      uint16
		HoldTime uint16
		RouterID uint32
		OptLen   uint8
		OptType  uint8
		CapLen   uint8
	}{
		Version:  4,
		ASN:      o.ASN,
		HoldTime: uint16(o.HoldTime.Seconds()),
		RouterID: binary.BigEndian.Uint32(o.RouterID.To4()),
		OptLen:   2 + uint8(len(caps)),
		OptType:  2, // Capabilities
		CapLen:   uint8(len(caps)),
	}
	hdr := header{
		Len:  uint16(binary.Size(header{})) + uint16(binary.Size(msg)) + uint16(len(caps)),
		Type: 1,
	}

	var b bytes.Buffer

	bs, err := hdr.MarshalBinary()
	if err != nil {
		return nil, err
	}
	b.Write(bs)
	binary.Write(&b, binary.BigEndian, msg)
	b.Write(caps)

	return b.Bytes(), nil
}

type Capability struct {
	Code uint8
	Data []byte
}

func Capability4ByteASN(asn uint32) Capability {
	ret := Capability{
		Code: 65,
		Data: make([]byte, 4),
	}
	binary.BigEndian.PutUint32(ret.Data, asn)
	return ret
}

func decodeCapabilities(r io.Reader) ([]Capability, error) {
	ret := []Capability{}

	for {
		var typ uint8
		if err := binary.Read(r, binary.BigEndian, &typ); err != nil {
			if err == io.EOF {
				return ret, nil
			}
			return nil, err
		}
		var len uint8
		if err := binary.Read(r, binary.BigEndian, &len); err != nil {
			return nil, err
		}
		bs := make([]byte, len)
		if _, err := io.ReadFull(r, bs); err != nil {
			return nil, err
		}
		ret = append(ret, Capability{typ, bs})
	}
}

func encodeCapabilities(caps []Capability) []byte {
	var b bytes.Buffer
	for _, cap := range caps {
		binary.Write(&b, binary.BigEndian, uint8(cap.Code))
		binary.Write(&b, binary.BigEndian, uint8(len(cap.Data)))
		b.Write(cap.Data)
	}
	return b.Bytes()
}
