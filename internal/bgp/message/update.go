package message

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type Update struct {
	Withdraw   []*net.IPNet
	Advertise  []*net.IPNet
	Attributes []Attribute
}

func decodeUpdate(r io.Reader) (*Update, error) {
	var len uint16
	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return nil, err
	}
	wdr, err := decodePrefixes(&io.LimitedReader{
		R: r,
		N: int64(len),
	})
	if err != nil {
		return nil, err
	}

	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return nil, err
	}
	attrs, err := decodeAttributes(&io.LimitedReader{
		R: r,
		N: int64(len),
	})
	if err != nil {
		return nil, err
	}

	adv, err := decodePrefixes(r)
	if err != nil {
		return nil, err
	}

	return &Update{
		Withdraw:   wdr,
		Advertise:  adv,
		Attributes: attrs,
	}, nil
}

func (u *Update) MarshalBinary() ([]byte, error) {
	wdr, err := encodePrefixes(u.Withdraw)
	if err != nil {
		return nil, err
	}

	attrs, err := encodeAttributes(u.Attributes)
	if err != nil {
		return nil, err
	}

	adv, err := encodePrefixes(u.Advertise)
	if err != nil {
		return nil, err
	}

	hdr := header{
		Len:  uint16(binary.Size(header{})) + 4 + uint16(len(wdr)) + uint16(len(attrs)) + uint16(len(adv)),
		Type: 2,
	}
	bs, err := hdr.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.Write(bs)
	binary.Write(&b, binary.BigEndian, uint16(len(wdr)))
	b.Write(wdr)
	binary.Write(&b, binary.BigEndian, uint16(len(attrs)))
	b.Write(attrs)
	b.Write(adv)

	return b.Bytes(), nil
}

func decodePrefixes(r io.Reader) ([]*net.IPNet, error) {
	var ret []*net.IPNet
	for {
		var pfxLen uint8
		if err := binary.Read(r, binary.BigEndian, &pfxLen); err != nil {
			if err == io.EOF {
				// Clean EOF at prefix binary, list is finished.
				return ret, nil
			}
			return nil, err
		}
		if pfxLen > 32 {
			return nil, fmt.Errorf("invalid prefix length %d, must be between 0 and 32", pfxLen)
		}

		ip := make(net.IP, 4)
		blen := pfxLen / 8
		if pfxLen%8 != 0 {
			blen++
		}
		if _, err := io.ReadFull(r, ip[:blen]); err != nil {
			return nil, err
		}

		m := net.CIDRMask(int(pfxLen), 32)
		if !ip.Equal(ip.Mask(m)) {
			// Note: strictly, the BGP spec says the value of the
			// masked bits is "irrelevant", but this makes UPDATE
			// parsing non-idempotent. In practice, I struggle to
			// think of a sane implementation that would not clear
			// these bits, so I'm declaring that it's an error until
			// something breaks.
			return nil, fmt.Errorf("invalid CIDR prefix %s/%d, IP has non-zero masked bits", ip, pfxLen)
		}

		ret = append(ret, &net.IPNet{
			IP:   ip,
			Mask: m,
		})
	}
}

func encodePrefixes(nets []*net.IPNet) ([]byte, error) {
	var b bytes.Buffer
	for _, n := range nets {
		if n.IP.To4() == nil {
			return nil, fmt.Errorf("can't serialize IPv6 prefix address %q", n)
		}
		o, _ := n.Mask.Size()
		b.WriteByte(byte(o))
		bytes := o / 8
		if o%8 != 0 {
			bytes++
		}
		b.Write(n.IP.To4()[:bytes])
	}
	return b.Bytes(), nil
}

type Attribute struct {
	Code uint16
	Data []byte
}

func decodeAttributes(r io.Reader) ([]Attribute, error) {
	var ret []Attribute
	for {
		// Read the code in a slightly strange way, to see if we get a
		// clean EOF with 0 bytes read. If so, it's a successful
		// conclusion.
		bs := make([]byte, 2)
		n, err := io.ReadFull(r, bs)
		if err != nil {
			if err == io.EOF && n == 0 {
				return ret, nil
			}
			return nil, err
		}
		code := binary.BigEndian.Uint16(bs)
		var l uint16
		if code&0x1000 == 0 {
			var l8 uint8
			if err := binary.Read(r, binary.BigEndian, &l8); err != nil {
				return nil, err
			}
			l = uint16(l8)
		} else {
			if err := binary.Read(r, binary.BigEndian, &l); err != nil {
				return nil, err
			}
		}

		bs = make([]byte, l)
		if _, err := io.ReadFull(r, bs); err != nil {
			return nil, err
		}

		ret = append(ret, Attribute{code, bs})
	}
}

func encodeAttributes(attrs []Attribute) ([]byte, error) {
	var b bytes.Buffer
	for _, attr := range attrs {
		binary.Write(&b, binary.BigEndian, uint16(attr.Code))
		if attr.Code&0x1000 == 0 {
			b.WriteByte(byte(len(attr.Data)))
		} else {
			binary.Write(&b, binary.BigEndian, uint16(len(attr.Data)))
		}
		b.Write(attr.Data)
	}
	return b.Bytes(), nil
}
