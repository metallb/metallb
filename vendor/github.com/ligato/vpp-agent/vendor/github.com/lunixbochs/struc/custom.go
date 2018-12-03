package struc

import (
	"io"
	"reflect"
)

type Custom interface {
	Pack(p []byte, opt *Options) (int, error)
	Unpack(r io.Reader, length int, opt *Options) error
	Size(opt *Options) int
	String() string
}

type customFallback struct {
	custom Custom
}

func (c customFallback) Pack(p []byte, val reflect.Value, opt *Options) (int, error) {
	return c.custom.Pack(p, opt)
}

func (c customFallback) Unpack(r io.Reader, val reflect.Value, opt *Options) error {
	return c.custom.Unpack(r, 1, opt)
}

func (c customFallback) Sizeof(val reflect.Value, opt *Options) int {
	return c.custom.Size(opt)
}

func (c customFallback) String() string {
	return c.custom.String()
}
