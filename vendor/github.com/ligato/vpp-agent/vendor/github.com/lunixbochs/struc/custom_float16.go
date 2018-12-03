package struc

import (
	"encoding/binary"
	"io"
	"math"
	"strconv"
)

type Float16 float64

func (f *Float16) Pack(p []byte, opt *Options) (int, error) {
	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}
	sign := uint16(0)
	if *f < 0 {
		sign = 1
	}
	var frac, exp uint16
	if math.IsInf(float64(*f), 0) {
		exp = 0x1f
		frac = 0
	} else if math.IsNaN(float64(*f)) {
		exp = 0x1f
		frac = 1
	} else {
		bits := math.Float64bits(float64(*f))
		exp64 := (bits >> 52) & 0x7ff
		if exp64 != 0 {
			exp = uint16((exp64 - 1023 + 15) & 0x1f)
		}
		frac = uint16((bits >> 42) & 0x3ff)
	}
	var out uint16
	out |= sign << 15
	out |= exp << 10
	out |= frac & 0x3ff
	order.PutUint16(p, out)
	return 2, nil
}
func (f *Float16) Unpack(r io.Reader, length int, opt *Options) error {
	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}
	var tmp [2]byte
	if _, err := r.Read(tmp[:]); err != nil {
		return err
	}
	val := order.Uint16(tmp[:2])
	sign := (val >> 15) & 1
	exp := int16((val >> 10) & 0x1f)
	frac := val & 0x3ff
	if exp == 0x1f {
		if frac != 0 {
			*f = Float16(math.NaN())
		} else {
			*f = Float16(math.Inf(int(sign)*-2 + 1))
		}
	} else {
		var bits uint64
		bits |= uint64(sign) << 63
		bits |= uint64(frac) << 42
		if exp > 0 {
			bits |= uint64(exp-15+1023) << 52
		}
		*f = Float16(math.Float64frombits(bits))
	}
	return nil
}
func (f *Float16) Size(opt *Options) int {
	return 2
}
func (f *Float16) String() string {
	return strconv.FormatFloat(float64(*f), 'g', -1, 32)
}
