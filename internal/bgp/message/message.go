package message

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func Decode(r io.Reader) (interface{}, error) {
	hdr, err := decodeHeader(r)
	if err != nil {
		return nil, err
	}

	pkt := &io.LimitedReader{
		R: r,
		N: int64(hdr.Len) - 19,
	}

	var ret interface{}
	switch hdr.Type {
	case 1:
		ret, err = decodeOpen(pkt)

	case 2:
		ret, err = decodeUpdate(pkt)

	case 3:
		ret, err = decodeNotification(pkt)

	case 4:
		ret, err = decodeKeepalive(pkt)

	default:
		return nil, fmt.Errorf("unknown BGP message type %d", hdr.Type)
	}
	if err != nil {
		return nil, err
	}

	if pkt.N != 0 {
		return nil, fmt.Errorf("%d trailing garbage bytes", pkt.N)
	}
	return ret, nil
}

type header struct {
	Marker1 uint64
	Marker2 uint64
	Len     uint16
	Type    uint8
}

func decodeHeader(r io.Reader) (*header, error) {
	var hdr header
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return nil, err
	}
	if hdr.Marker1 != 0xffffffffffffffff || hdr.Marker2 != 0xffffffffffffffff {
		return nil, errors.New("invalid BGP message marker")
	}
	if hdr.Len < 19 {
		return nil, fmt.Errorf("invalid BGP message length %d, must be at least 19 bytes", hdr.Len)
	}
	return &hdr, nil
}

func (h *header) MarshalBinary() ([]byte, error) {
	h.Marker1 = 0xffffffffffffffff
	h.Marker2 = 0xffffffffffffffff

	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, h)
	return b.Bytes(), nil
}
