package struc

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

type Options struct {
	ByteAlign int
	PtrSize   int
	Order     binary.ByteOrder
}

func (o *Options) Validate() error {
	if o.PtrSize == 0 {
		o.PtrSize = 32
	} else {
		switch o.PtrSize {
		case 8, 16, 32, 64:
		default:
			return fmt.Errorf("Invalid Options.PtrSize: %d. Must be in (8, 16, 32, 64)", o.PtrSize)
		}
	}
	return nil
}

var emptyOptions = &Options{}

func prep(data interface{}) (reflect.Value, Packer, error) {
	value := reflect.ValueOf(data)
	for value.Kind() == reflect.Ptr {
		next := value.Elem().Kind()
		if next == reflect.Struct || next == reflect.Ptr {
			value = value.Elem()
		} else {
			break
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		fields, err := parseFields(value)
		return value, fields, err
	default:
		if !value.IsValid() {
			return reflect.Value{}, nil, fmt.Errorf("Invalid reflect.Value for %+v", data)
		}
		if c, ok := data.(Custom); ok {
			return value, customFallback{c}, nil
		}
		return value, binaryFallback(value), nil
	}
}

func Pack(w io.Writer, data interface{}) error {
	return PackWithOptions(w, data, nil)
}

func PackWithOptions(w io.Writer, data interface{}, options *Options) error {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return err
	}
	val, packer, err := prep(data)
	if err != nil {
		return err
	}
	if val.Type().Kind() == reflect.String {
		val = val.Convert(reflect.TypeOf([]byte{}))
	}
	size := packer.Sizeof(val, options)
	buf := make([]byte, size)
	if _, err := packer.Pack(buf, val, options); err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

func Unpack(r io.Reader, data interface{}) error {
	return UnpackWithOptions(r, data, nil)
}

func UnpackWithOptions(r io.Reader, data interface{}, options *Options) error {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return err
	}
	val, packer, err := prep(data)
	if err != nil {
		return err
	}
	return packer.Unpack(r, val, options)
}

func Sizeof(data interface{}) (int, error) {
	return SizeofWithOptions(data, nil)
}

func SizeofWithOptions(data interface{}, options *Options) (int, error) {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return 0, err
	}
	val, packer, err := prep(data)
	if err != nil {
		return 0, err
	}
	return packer.Sizeof(val, options), nil
}
