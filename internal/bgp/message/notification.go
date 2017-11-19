package message

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
)

type Notification struct {
	Code uint16
	Data []byte
}

func decodeNotification(r io.Reader) (*Notification, error) {
	var code uint16
	if err := binary.Read(r, binary.BigEndian, &code); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return &Notification{code, data}, nil
}

func (n *Notification) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	hdr := header{
		Len:  uint16(binary.Size(header{})) + 2 + uint16(len(n.Data)),
		Type: 3,
	}
	bs, err := hdr.MarshalBinary()
	if err != nil {
		return nil, err
	}
	b.Write(bs)
	binary.Write(&b, binary.BigEndian, n.Code)
	b.Write(n.Data)
	return b.Bytes(), nil
}
