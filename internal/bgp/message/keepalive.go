package message

import (
	"encoding/binary"
	"io"
)

type Keepalive struct{}

func decodeKeepalive(r io.Reader) (*Keepalive, error) {
	return &Keepalive{}, nil
}

func (k *Keepalive) MarshalBinary() ([]byte, error) {
	hdr := header{
		Len:  uint16(binary.Size(header{})),
		Type: 4,
	}
	return hdr.MarshalBinary()
}
