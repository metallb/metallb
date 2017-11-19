// +build gofuzz

package message

import (
	"bytes"
	"encoding"
)

func Fuzz(bs []byte) int {
	b := bytes.NewBuffer(bs)
	m, err := Decode(b)
	if err != nil {
		return 0
	}
	if b.Len() != 0 {
		return 0
	}
	bs2, err := m.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		panic("decoded something I cannot encode")
	}
	if !bytes.Equal(bs, bs2) {
		panic("decoded message that doesn't reencode the same")
	}
	return 1
}
