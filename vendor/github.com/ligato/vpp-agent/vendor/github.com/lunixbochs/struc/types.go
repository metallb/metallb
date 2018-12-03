package struc

import (
	"fmt"
	"reflect"
)

type Type int

const (
	Invalid Type = iota
	Pad
	Bool
	Int
	Int8
	Uint8
	Int16
	Uint16
	Int32
	Uint32
	Int64
	Uint64
	Float32
	Float64
	String
	Struct
	Ptr

	SizeType
	OffType
	CustomType
)

func (t Type) Resolve(options *Options) Type {
	switch t {
	case OffType:
		switch options.PtrSize {
		case 8:
			return Int8
		case 16:
			return Int16
		case 32:
			return Int32
		case 64:
			return Int64
		default:
			panic(fmt.Sprintf("unsupported ptr bits: %d", options.PtrSize))
		}
	case SizeType:
		switch options.PtrSize {
		case 8:
			return Uint8
		case 16:
			return Uint16
		case 32:
			return Uint32
		case 64:
			return Uint64
		default:
			panic(fmt.Sprintf("unsupported ptr bits: %d", options.PtrSize))
		}
	}
	return t
}

func (t Type) String() string {
	return typeNames[t]
}

func (t Type) Size() int {
	switch t {
	case SizeType, OffType:
		panic("Size_t/Off_t types must be converted to another type using options.PtrSize")
	case Pad, String, Int8, Uint8, Bool:
		return 1
	case Int16, Uint16:
		return 2
	case Int32, Uint32, Float32:
		return 4
	case Int64, Uint64, Float64:
		return 8
	default:
		panic("Cannot resolve size of type:" + t.String())
	}
}

var typeLookup = map[string]Type{
	"pad":     Pad,
	"bool":    Bool,
	"byte":    Uint8,
	"int8":    Int8,
	"uint8":   Uint8,
	"int16":   Int16,
	"uint16":  Uint16,
	"int32":   Int32,
	"uint32":  Uint32,
	"int64":   Int64,
	"uint64":  Uint64,
	"float32": Float32,
	"float64": Float64,

	"size_t": SizeType,
	"off_t":  OffType,
}

var typeNames = map[Type]string{
	CustomType: "Custom",
}

func init() {
	for name, enum := range typeLookup {
		typeNames[enum] = name
	}
}

type Size_t uint64
type Off_t int64

var reflectTypeMap = map[reflect.Kind]Type{
	reflect.Bool:    Bool,
	reflect.Int8:    Int8,
	reflect.Int16:   Int16,
	reflect.Int:     Int32,
	reflect.Int32:   Int32,
	reflect.Int64:   Int64,
	reflect.Uint8:   Uint8,
	reflect.Uint16:  Uint16,
	reflect.Uint:    Uint32,
	reflect.Uint32:  Uint32,
	reflect.Uint64:  Uint64,
	reflect.Float32: Float32,
	reflect.Float64: Float64,
	reflect.String:  String,
	reflect.Struct:  Struct,
	reflect.Ptr:     Ptr,
}
