package struc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

type Field struct {
	Name     string
	Ptr      bool
	Index    int
	Type     Type
	defType  Type
	Array    bool
	Slice    bool
	Len      int
	Order    binary.ByteOrder
	Sizeof   []int
	Sizefrom []int
	Fields   Fields
	kind     reflect.Kind
}

func (f *Field) String() string {
	var out string
	if f.Type == Pad {
		return fmt.Sprintf("{type: Pad, len: %d}", f.Len)
	} else {
		out = fmt.Sprintf("type: %s, order: %v", f.Type.String(), f.Order)
	}
	if f.Sizefrom != nil {
		out += fmt.Sprintf(", sizefrom: %v", f.Sizefrom)
	} else if f.Len > 0 {
		out += fmt.Sprintf(", len: %d", f.Len)
	}
	if f.Sizeof != nil {
		out += fmt.Sprintf(", sizeof: %v", f.Sizeof)
	}
	return "{" + out + "}"
}

func (f *Field) Size(val reflect.Value, options *Options) int {
	typ := f.Type.Resolve(options)
	size := 0
	if typ == Struct {
		vals := []reflect.Value{val}
		if f.Slice {
			vals = make([]reflect.Value, val.Len())
			for i := 0; i < val.Len(); i++ {
				vals[i] = val.Index(i)
			}
		}
		for _, val := range vals {
			size += f.Fields.Sizeof(val, options)
		}
	} else if typ == Pad {
		size = f.Len
	} else if f.Slice || f.kind == reflect.String {
		length := val.Len()
		if f.Len > 1 {
			length = f.Len
		}
		size = length * typ.Size()
	} else if typ == CustomType {
		return val.Addr().Interface().(Custom).Size(options)
	} else {
		size = typ.Size()
	}
	align := options.ByteAlign
	if align > 0 && size < align {
		size = align
	}
	return size
}

func (f *Field) packVal(buf []byte, val reflect.Value, length int, options *Options) (size int, err error) {
	order := f.Order
	if options.Order != nil {
		order = options.Order
	}
	if f.Ptr {
		val = val.Elem()
	}
	typ := f.Type.Resolve(options)
	switch typ {
	case Struct:
		return f.Fields.Pack(buf, val, options)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		size = typ.Size()
		var n uint64
		switch f.kind {
		case reflect.Bool:
			if val.Bool() {
				n = 1
			} else {
				n = 0
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n = uint64(val.Int())
		default:
			n = val.Uint()
		}
		switch typ {
		case Bool:
			if n != 0 {
				buf[0] = 1
			} else {
				buf[0] = 0
			}
		case Int8, Uint8:
			buf[0] = byte(n)
		case Int16, Uint16:
			order.PutUint16(buf, uint16(n))
		case Int32, Uint32:
			order.PutUint32(buf, uint32(n))
		case Int64, Uint64:
			order.PutUint64(buf, uint64(n))
		}
	case Float32, Float64:
		size = typ.Size()
		n := val.Float()
		switch typ {
		case Float32:
			order.PutUint32(buf, math.Float32bits(float32(n)))
		case Float64:
			order.PutUint64(buf, math.Float64bits(n))
		}
	case String:
		switch f.kind {
		case reflect.String:
			size = val.Len()
			copy(buf, []byte(val.String()))
		default:
			// TODO: handle kind != bytes here
			size = val.Len()
			copy(buf, val.Bytes())
		}
	case CustomType:
		return val.Addr().Interface().(Custom).Pack(buf, options)
	default:
		panic(fmt.Sprintf("no pack handler for type: %s", typ))
	}
	return
}

func (f *Field) Pack(buf []byte, val reflect.Value, length int, options *Options) (int, error) {
	typ := f.Type.Resolve(options)
	if typ == Pad {
		for i := 0; i < length; i++ {
			buf[i] = 0
		}
		return length, nil
	}
	if f.Slice {
		// special case strings and byte slices for performance
		end := val.Len()
		if !f.Array && typ == Uint8 && (f.defType == Uint8 || f.kind == reflect.String) {
			var tmp []byte
			if f.kind == reflect.String {
				tmp = []byte(val.String())
			} else {
				tmp = val.Bytes()
			}
			copy(buf, tmp)
			if end < length {
				// TODO: allow configuring pad byte?
				rep := bytes.Repeat([]byte{0}, length-end)
				copy(buf[end:], rep)
				return length, nil
			}
			return val.Len(), nil
		}
		pos := 0
		var zero reflect.Value
		if end < length {
			zero = reflect.Zero(val.Type().Elem())
		}
		for i := 0; i < length; i++ {
			cur := zero
			if i < end {
				cur = val.Index(i)
			}
			if n, err := f.packVal(buf[pos:], cur, 1, options); err != nil {
				return pos, err
			} else {
				pos += n
			}
		}
		return pos, nil
	} else {
		return f.packVal(buf, val, length, options)
	}
}

func (f *Field) unpackVal(buf []byte, val reflect.Value, length int, options *Options) error {
	order := f.Order
	if options.Order != nil {
		order = options.Order
	}
	if f.Ptr {
		val = val.Elem()
	}
	typ := f.Type.Resolve(options)
	switch typ {
	case Float32, Float64:
		var n float64
		switch typ {
		case Float32:
			n = float64(math.Float32frombits(order.Uint32(buf)))
		case Float64:
			n = math.Float64frombits(order.Uint64(buf))
		}
		switch f.kind {
		case reflect.Float32, reflect.Float64:
			val.SetFloat(n)
		default:
			return fmt.Errorf("struc: refusing to unpack float into field %s of type %s", f.Name, f.kind.String())
		}
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		var n uint64
		switch typ {
		case Bool, Int8, Uint8:
			n = uint64(buf[0])
		case Int16, Uint16:
			n = uint64(order.Uint16(buf))
		case Int32, Uint32:
			n = uint64(order.Uint32(buf))
		case Int64, Uint64:
			n = uint64(order.Uint64(buf))
		}
		switch f.kind {
		case reflect.Bool:
			val.SetBool(n != 0)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			val.SetInt(int64(n))
		default:
			val.SetUint(n)
		}
	default:
		panic(fmt.Sprintf("no unpack handler for type: %s", typ))
	}
	return nil
}

func (f *Field) Unpack(buf []byte, val reflect.Value, length int, options *Options) error {
	typ := f.Type.Resolve(options)
	if typ == Pad || f.kind == reflect.String {
		if typ == Pad {
			return nil
		} else {
			val.SetString(string(buf))
			return nil
		}
	} else if f.Slice {
		if val.Cap() < length {
			val.Set(reflect.MakeSlice(val.Type(), length, length))
		} else if val.Len() < length {
			val.Set(val.Slice(0, length))
		}
		// special case byte slices for performance
		if !f.Array && typ == Uint8 && f.defType == Uint8 {
			copy(val.Bytes(), buf[:length])
			return nil
		}
		pos := 0
		size := typ.Size()
		for i := 0; i < length; i++ {
			if err := f.unpackVal(buf[pos:pos+size], val.Index(i), 1, options); err != nil {
				return err
			}
			pos += size
		}
		return nil
	} else {
		return f.unpackVal(buf, val, length, options)
	}
}
