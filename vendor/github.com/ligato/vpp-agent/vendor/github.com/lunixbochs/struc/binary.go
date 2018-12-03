package struc

import (
	"encoding/binary"
	"io"
	"reflect"
)

type byteWriter struct {
	buf []byte
	pos int
}

func (b byteWriter) Write(p []byte) (int, error) {
	capacity := len(b.buf) - b.pos
	if capacity < len(p) {
		p = p[:capacity]
	}
	if len(p) > 0 {
		copy(b.buf[b.pos:], p)
		b.pos += len(p)
	}
	return len(p), nil
}

type binaryFallback reflect.Value

func (b binaryFallback) String() string {
	return b.String()
}

func (b binaryFallback) Sizeof(val reflect.Value, options *Options) int {
	return binary.Size(val.Interface())
}

func (b binaryFallback) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	tmp := byteWriter{buf: buf}
	var order binary.ByteOrder = binary.BigEndian
	if options.Order != nil {
		order = options.Order
	}
	err := binary.Write(tmp, order, val.Interface())
	return tmp.pos, err
}

func (b binaryFallback) Unpack(r io.Reader, val reflect.Value, options *Options) error {
	var order binary.ByteOrder = binary.BigEndian
	if options.Order != nil {
		order = options.Order
	}
	return binary.Read(r, order, val.Interface())
}
