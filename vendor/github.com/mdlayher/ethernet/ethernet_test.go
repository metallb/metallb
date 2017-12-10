package ethernet

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

func TestFrameMarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		f    *Frame
		b    []byte
		err  error
	}{
		{
			desc: "S-VLAN, no C-VLAN",
			f: &Frame{
				// Contents don't matter.
				ServiceVLAN: &VLAN{},
			},
			err: ErrInvalidVLAN,
		},
		{
			desc: "IPv4, no VLANs",
			f: &Frame{
				Destination: net.HardwareAddr{0, 1, 0, 1, 0, 1},
				Source:      net.HardwareAddr{1, 0, 1, 0, 1, 0},
				EtherType:   EtherTypeIPv4,
				Payload:     bytes.Repeat([]byte{0}, 50),
			},
			b: append([]byte{
				0, 1, 0, 1, 0, 1,
				1, 0, 1, 0, 1, 0,
				0x08, 0x00,
			}, bytes.Repeat([]byte{0}, 50)...),
		},
		{
			desc: "IPv6, C-VLAN: (PRI 1, ID 101)",
			f: &Frame{
				Destination: net.HardwareAddr{1, 0, 1, 0, 1, 0},
				Source:      net.HardwareAddr{0, 1, 0, 1, 0, 1},
				VLAN: &VLAN{
					Priority: 1,
					ID:       101,
				},
				EtherType: EtherTypeIPv6,
				Payload:   bytes.Repeat([]byte{0}, 50),
			},
			b: append([]byte{
				1, 0, 1, 0, 1, 0,
				0, 1, 0, 1, 0, 1,
				0x81, 0x00,
				0x20, 0x65,
				0x86, 0xDD,
			}, bytes.Repeat([]byte{0}, 50)...),
		},
		{
			desc: "ARP, S-VLAN: (PRI 0, DROP, ID 100), C-VLAN: (PRI 1, ID 101)",
			f: &Frame{
				Destination: Broadcast,
				Source:      net.HardwareAddr{0, 1, 0, 1, 0, 1},
				ServiceVLAN: &VLAN{
					DropEligible: true,
					ID:           100,
				},
				VLAN: &VLAN{
					Priority: 1,
					ID:       101,
				},
				EtherType: EtherTypeARP,
				Payload:   bytes.Repeat([]byte{0}, 50),
			},
			b: append([]byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0, 1, 0, 1, 0, 1,
				0x88, 0xa8,
				0x10, 0x64,
				0x81, 0x00,
				0x20, 0x65,
				0x08, 0x06,
			}, bytes.Repeat([]byte{0}, 50)...),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			b, err := tt.f.MarshalBinary()
			if err != nil {
				if want, got := tt.err, err; want != got {
					t.Fatalf("unexpected error: %v != %v", want, got)
				}

				return
			}

			if want, got := tt.b, b; !bytes.Equal(want, got) {
				t.Fatalf("unexpected Frame bytes:\n- want: %v\n-  got: %v", want, got)
			}
		})
	}
}

func TestFrameMarshalFCS(t *testing.T) {
	var tests = []struct {
		desc string
		f    *Frame
		b    []byte
		err  error
	}{
		{
			desc: "IPv4, no VLANs",
			f: &Frame{
				Destination: net.HardwareAddr{0, 1, 0, 1, 0, 1},
				Source:      net.HardwareAddr{1, 0, 1, 0, 1, 0},
				EtherType:   EtherTypeIPv4,
				Payload:     bytes.Repeat([]byte{0}, 50),
			},
			b: append(
				append(
					[]byte{
						0, 1, 0, 1, 0, 1,
						1, 0, 1, 0, 1, 0,
						0x08, 0x00,
					},
					bytes.Repeat([]byte{0}, 50)...,
				),
				[]byte{159, 205, 24, 60}...,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			b, err := tt.f.MarshalFCS()
			if err != nil {
				if want, got := tt.err, err; want != got {
					t.Fatalf("unexpected error: %v != %v", want, got)
				}

				return
			}

			if want, got := tt.b, b; !bytes.Equal(want, got) {
				t.Fatalf("unexpected Frame bytes:\n- want: %v\n-  got: %v", want, got)
			}
		})
	}
}

func TestFrameUnmarshalBinary(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		f    *Frame
		err  error
	}{
		{
			desc: "nil buffer",
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "short buffer",
			b:    bytes.Repeat([]byte{0}, 13),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "1 short S-VLAN",
			b: []byte{
				0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0,
				0x88, 0xa8,
				0x00,
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			desc: "1 short C-VLAN",
			b: []byte{
				0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0,
				0x81, 0x00,
				0x00,
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			desc: "VLAN ID too large",
			b: []byte{
				0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0,
				0x81, 0x00,
				0xff, 0xff,
				0x00, 0x00,
			},
			err: ErrInvalidVLAN,
		},
		{
			desc: "no C-VLAN after S-VLAN",
			b: []byte{
				0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0,
				0x88, 0xa8,
				0x20, 0x65,
				0x08, 0x06,
				0x00, 0x00, 0x00, 0x00,
			},
			err: ErrInvalidVLAN,
		},
		{
			desc: "short C-VLAN after S-VLAN",
			b: []byte{
				0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0,
				0x88, 0xa8,
				0x20, 0x65,
				0x81, 0x00,
				0x00, 0x00,
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			desc: "go-fuzz crasher: VLAN tag without enough bytes for trailing EtherType",
			b:    []byte("190734863281\x81\x0032"),
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "0 VLANs detected, but 1 may have been present",
			b:    bytes.Repeat([]byte{0}, 56),
			f: &Frame{
				Destination: net.HardwareAddr{0, 0, 0, 0, 0, 0},
				Source:      net.HardwareAddr{0, 0, 0, 0, 0, 0},
				Payload:     bytes.Repeat([]byte{0}, 42),
			},
		},
		{
			desc: "IPv4, no VLANs",
			b: append([]byte{
				0, 1, 0, 1, 0, 1,
				1, 0, 1, 0, 1, 0,
				0x08, 0x00,
			}, bytes.Repeat([]byte{0}, 50)...),
			f: &Frame{
				Destination: net.HardwareAddr{0, 1, 0, 1, 0, 1},
				Source:      net.HardwareAddr{1, 0, 1, 0, 1, 0},
				EtherType:   EtherTypeIPv4,
				Payload:     bytes.Repeat([]byte{0}, 50),
			},
		},
		{
			desc: "IPv6, C-VLAN: (PRI 1, ID 101)",
			b: append([]byte{
				1, 0, 1, 0, 1, 0,
				0, 1, 0, 1, 0, 1,
				0x81, 0x00,
				0x20, 0x65,
				0x86, 0xDD,
			}, bytes.Repeat([]byte{0}, 50)...),
			f: &Frame{
				Destination: net.HardwareAddr{1, 0, 1, 0, 1, 0},
				Source:      net.HardwareAddr{0, 1, 0, 1, 0, 1},
				VLAN: &VLAN{
					Priority: 1,
					ID:       101,
				},
				EtherType: EtherTypeIPv6,
				Payload:   bytes.Repeat([]byte{0}, 50),
			},
		},
		{
			desc: "ARP, S-VLAN: (PRI 0, DROP, ID 100), C-VLAN: (PRI 1, ID 101)",
			b: append([]byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0, 1, 0, 1, 0, 1,
				0x88, 0xa8,
				0x10, 0x64,
				0x81, 0x00,
				0x20, 0x65,
				0x08, 0x06,
			}, bytes.Repeat([]byte{0}, 50)...),
			f: &Frame{
				Destination: Broadcast,
				Source:      net.HardwareAddr{0, 1, 0, 1, 0, 1},
				ServiceVLAN: &VLAN{
					DropEligible: true,
					ID:           100,
				},
				VLAN: &VLAN{
					Priority: 1,
					ID:       101,
				},
				EtherType: EtherTypeARP,
				Payload:   bytes.Repeat([]byte{0}, 50),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := new(Frame)
			if err := f.UnmarshalBinary(tt.b); err != nil {
				if want, got := tt.err, err; want != got {
					t.Fatalf("unexpected error: %v != %v", want, got)
				}

				return
			}

			if want, got := tt.f, f; !reflect.DeepEqual(want, got) {
				t.Fatalf("unexpected Frame:\n- want: %v\n-  got: %v", want, got)
			}
		})
	}
}

func TestFrameUnmarshalFCS(t *testing.T) {
	var tests = []struct {
		desc string
		b    []byte
		f    *Frame
		err  error
	}{
		{
			desc: "too short for FCS",
			b:    []byte{1, 2, 3},
			err:  io.ErrUnexpectedEOF,
		},
		{
			desc: "invalid FCS",
			b:    []byte{1, 2, 3, 4},
			err:  ErrInvalidFCS,
		},
		{
			desc: "IPv4, no VLANs",
			b: append(
				append(
					[]byte{
						0, 1, 0, 1, 0, 1,
						1, 0, 1, 0, 1, 0,
						0x08, 0x00,
					},
					bytes.Repeat([]byte{0}, 50)...,
				),
				[]byte{159, 205, 24, 60}...,
			),
			f: &Frame{
				Destination: net.HardwareAddr{0, 1, 0, 1, 0, 1},
				Source:      net.HardwareAddr{1, 0, 1, 0, 1, 0},
				EtherType:   EtherTypeIPv4,
				Payload:     bytes.Repeat([]byte{0}, 50),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f := new(Frame)
			if err := f.UnmarshalFCS(tt.b); err != nil {
				if want, got := tt.err, err; want != got {
					t.Fatalf("unexpected error: %v != %v", want, got)
				}

				return
			}

			if want, got := tt.f, f; !reflect.DeepEqual(want, got) {
				t.Fatalf("unexpected Frame:\n- want: %v\n-  got: %v", want, got)
			}
		})
	}
}

// Benchmarks for Frame.MarshalBinary with varying VLAN tags and payloads

func BenchmarkFrameMarshalBinary(b *testing.B) {
	f := &Frame{
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameMarshalBinary(b, f)
}

func BenchmarkFrameMarshalBinaryCVLAN(b *testing.B) {
	f := &Frame{
		VLAN: &VLAN{
			Priority: PriorityBackground,
			ID:       10,
		},
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameMarshalBinary(b, f)
}

func BenchmarkFrameMarshalBinarySVLANCVLAN(b *testing.B) {
	f := &Frame{
		ServiceVLAN: &VLAN{
			Priority: PriorityBackground,
			ID:       10,
		},
		VLAN: &VLAN{
			Priority: PriorityBestEffort,
			ID:       20,
		},
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameMarshalBinary(b, f)
}

func BenchmarkFrameMarshalBinaryJumboPayload(b *testing.B) {
	f := &Frame{
		Payload: make([]byte, 8192),
	}

	benchmarkFrameMarshalBinary(b, f)
}

func benchmarkFrameMarshalBinary(b *testing.B, f *Frame) {
	f.Destination = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
	f.Source = net.HardwareAddr{0xad, 0xbe, 0xef, 0xde, 0xad, 0xde}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := f.MarshalBinary(); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmarks for Frame.MarshalFCS

func BenchmarkFrameMarshalFCS(b *testing.B) {
	f := &Frame{
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameMarshalFCS(b, f)
}

func benchmarkFrameMarshalFCS(b *testing.B, f *Frame) {
	f.Destination = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
	f.Source = net.HardwareAddr{0xad, 0xbe, 0xef, 0xde, 0xad, 0xde}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := f.MarshalFCS(); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmarks for Frame.UnmarshalBinary with varying VLAN tags and payloads

func BenchmarkFrameUnmarshalBinary(b *testing.B) {
	f := &Frame{
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameUnmarshalBinary(b, f)
}

func BenchmarkFrameUnmarshalBinaryCVLAN(b *testing.B) {
	f := &Frame{
		VLAN: &VLAN{
			Priority: PriorityBackground,
			ID:       10,
		},

		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameUnmarshalBinary(b, f)
}

func BenchmarkFrameUnmarshalBinarySVLANCVLAN(b *testing.B) {
	f := &Frame{
		ServiceVLAN: &VLAN{
			Priority: PriorityBackground,
			ID:       10,
		},
		VLAN: &VLAN{
			Priority: PriorityBestEffort,
			ID:       20,
		},
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameUnmarshalBinary(b, f)
}

func BenchmarkFrameUnmarshalBinaryJumboPayload(b *testing.B) {
	f := &Frame{
		Payload: make([]byte, 8192),
	}

	benchmarkFrameUnmarshalBinary(b, f)
}

func benchmarkFrameUnmarshalBinary(b *testing.B, f *Frame) {
	f.Destination = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
	f.Source = net.HardwareAddr{0xad, 0xbe, 0xef, 0xde, 0xad, 0xde}

	fb, err := f.MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := f.UnmarshalBinary(fb); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmarks for Frame.UnmarshalFCS

func BenchmarkFrameUnmarshalFCS(b *testing.B) {
	f := &Frame{
		Payload: []byte{0, 1, 2, 3, 4},
	}

	benchmarkFrameUnmarshalFCS(b, f)
}

func benchmarkFrameUnmarshalFCS(b *testing.B, f *Frame) {
	f.Destination = net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}
	f.Source = net.HardwareAddr{0xad, 0xbe, 0xef, 0xde, 0xad, 0xde}

	fb, err := f.MarshalFCS()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := f.UnmarshalFCS(fb); err != nil {
			b.Fatal(err)
		}
	}
}
