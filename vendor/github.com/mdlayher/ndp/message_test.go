package ndp_test

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp"
)

func TestMarshalParseMessage(t *testing.T) {
	ip := mustIPv6("2001:db8:dead:beef:f00::00d")
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		m    ndp.Message
		bs   [][]byte
	}{
		{
			name: "neighbor advertisement",
			m: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ip,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Target,
						Addr:      addr,
					},
				},
			},
			bs: [][]byte{
				// ICMPv6 header and NA message.
				{
					136, 0x00, 0x00, 0x00,
					0xe0, 0x00, 0x00, 0x00,
				},
				ip,
				// Target LLA option.
				{
					0x02, 0x01,
				},
				addr,
			},
		},
		{
			name: "neighbor solicitation",
			m: &ndp.NeighborSolicitation{
				TargetAddress: ip,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
				},
			},
			bs: [][]byte{
				// ICMPv6 header and NS message.
				{
					135, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
				ip,
				// Source LLA option.
				{
					0x01, 0x01,
				},
				addr,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := ndp.MarshalMessage(tt.m)
			if err != nil {
				t.Fatalf("failed to marshal message: %v", err)
			}

			// Append all byte slices from the test fixture.
			var ttb []byte
			for _, bb := range tt.bs {
				ttb = append(ttb, bb...)
			}

			if diff := cmp.Diff(ttb, b); diff != "" {
				t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
			}

			m, err := ndp.ParseMessage(b)
			if err != nil {
				t.Fatalf("failed to unmarshal message: %v", err)
			}

			if diff := cmp.Diff(tt.m, m); diff != "" {
				t.Fatalf("unexpected message (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	ip := mustIPv6("::")

	tests := []struct {
		name string
		bs   [][]byte
		m    ndp.Message
		ok   bool
	}{
		{
			name: "bad, short",
			bs: [][]byte{{
				255,
			}},
		},
		{
			name: "bad, unknown type",
			bs: [][]byte{{
				255, 0x00, 0x00, 0x00,
			}},
		},
		{
			name: "bad, fuzz crasher",
			bs: [][]byte{
				[]byte("\x880000000000000000000" + "0000\x01\x01"),
			},
		},
		{
			name: "bad, short option",
			bs: [][]byte{
				{
					136, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
				ip,
				{
					0xff,
				},
			},
		},
		{
			name: "bad, unknown option",
			bs: [][]byte{
				{
					136, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
				ip,
				{
					0xff, 0x01,
				},
			},
		},
		{
			name: "ok, neighbor advertisement",
			bs: [][]byte{
				{
					136, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
				ip,
			},
			m:  &ndp.NeighborAdvertisement{},
			ok: true,
		},
		{
			name: "ok, neighbor advertisement",
			bs: [][]byte{
				{
					135, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
				ip,
			},
			m:  &ndp.NeighborSolicitation{},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Append all byte slices from the test fixture.
			var ttb []byte
			for _, bb := range tt.bs {
				ttb = append(ttb, bb...)
			}

			m, err := ndp.ParseMessage(ttb)

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			if typ := reflect.TypeOf(m); typ != reflect.TypeOf(tt.m) {
				t.Fatalf("unexpected message type: %T", typ)
			}
		})
	}
}

func TestNeighborAdvertisementMarshalUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8::1")
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	bfn := func(b []byte) []byte {
		return append(b, ip...)
	}

	tests := []struct {
		name string
		na   *ndp.NeighborAdvertisement
		b    []byte
		ok   bool
	}{
		{
			name: "bad, malformed IP address",
			na: &ndp.NeighborAdvertisement{
				TargetAddress: net.IP{192, 168, 1, 1, 0, 0},
			},
		},
		{
			name: "bad, IPv4 address",
			na: &ndp.NeighborAdvertisement{
				TargetAddress: net.IPv4(192, 168, 1, 1),
			},
		},
		{
			name: "ok, no flags",
			na: &ndp.NeighborAdvertisement{
				TargetAddress: ip,
			},
			b:  bfn([]byte{0x00, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, router",
			na: &ndp.NeighborAdvertisement{
				Router:        true,
				TargetAddress: ip,
			},
			b:  bfn([]byte{0x80, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, solicited",
			na: &ndp.NeighborAdvertisement{
				Solicited:     true,
				TargetAddress: ip,
			},
			b:  bfn([]byte{0x40, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, override",
			na: &ndp.NeighborAdvertisement{
				Override:      true,
				TargetAddress: ip,
			},
			b:  bfn([]byte{0x20, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, all flags",
			na: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ip,
			},
			b:  bfn([]byte{0xe0, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, with target LLA",
			na: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ip,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Target,
						Addr:      addr,
					},
				},
			},
			b: append(
				// ICMPv6 and NA.
				bfn([]byte{0xe0, 0x00, 0x00, 0x00}),
				// Target LLA option.
				[]byte{
					0x02, 0x01,
					0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				}...,
			),
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.na.MarshalBinary()

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			if diff := cmp.Diff(tt.b, b); diff != "" {
				t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
			}

			na := new(ndp.NeighborAdvertisement)
			if err := na.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.na, na); diff != "" {
				t.Fatalf("unexpected neighbor advertisement (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNeighborAdvertisementUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8:dead:beef:f00::00d")

	tests := []struct {
		name string
		b    []byte
		na   *ndp.NeighborAdvertisement
		ok   bool
	}{
		{
			name: "bad, short",
			b:    ip,
		},
		{
			name: "bad, IPv4 mapped",
			b: append([]byte{
				0xe0, 0x00, 0x00, 0x00,
			}, net.IPv4(192, 168, 1, 1)...),
		},
		{
			name: "ok",
			b: append([]byte{
				0xe0, 0x00, 0x00, 0x00,
			}, ip...),
			na: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ip,
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			na := new(ndp.NeighborAdvertisement)
			err := na.UnmarshalBinary(tt.b)

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			if diff := cmp.Diff(tt.na, na); diff != "" {
				t.Fatalf("unexpected neighbor advertisement (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNeighborSolicitationMarshalUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8::1")
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	bfn := func(b []byte) []byte {
		return append(b, ip...)
	}

	tests := []struct {
		name string
		ns   *ndp.NeighborSolicitation
		b    []byte
		ok   bool
	}{
		{
			name: "bad, malformed IP address",
			ns: &ndp.NeighborSolicitation{
				TargetAddress: net.IP{192, 168, 1, 1, 0, 0},
			},
		},
		{
			name: "bad, IPv4 address",
			ns: &ndp.NeighborSolicitation{
				TargetAddress: net.IPv4(192, 168, 1, 1),
			},
		},
		{
			name: "ok, no options",
			ns: &ndp.NeighborSolicitation{
				TargetAddress: ip,
			},
			b:  bfn([]byte{0x00, 0x00, 0x00, 0x00}),
			ok: true,
		},
		{
			name: "ok, with source LLA",
			ns: &ndp.NeighborSolicitation{
				TargetAddress: ip,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
				},
			},
			b: append(
				// ICMPv6 and NA.
				bfn([]byte{0x00, 0x00, 0x00, 0x00}),
				// Source LLA option.
				[]byte{
					0x01, 0x01,
					0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				}...,
			),
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.ns.MarshalBinary()

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			if diff := cmp.Diff(tt.b, b); diff != "" {
				t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
			}

			na := new(ndp.NeighborSolicitation)
			if err := na.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.ns, na); diff != "" {
				t.Fatalf("unexpected neighbor advertisement (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNeighborSolicitationUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8:dead:beef:f00::00d")

	tests := []struct {
		name string
		b    []byte
		ns   *ndp.NeighborSolicitation
		ok   bool
	}{
		{
			name: "bad, short",
			b:    ip,
		},
		{
			name: "bad, IPv4 mapped",
			b: append([]byte{
				0xe0, 0x00, 0x00, 0x00,
			}, net.IPv4(192, 168, 1, 1)...),
		},
		{
			name: "ok",
			b: append([]byte{
				0x00, 0x00, 0x00, 0x00,
			}, ip...),
			ns: &ndp.NeighborSolicitation{
				TargetAddress: ip,
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := new(ndp.NeighborSolicitation)
			err := ns.UnmarshalBinary(tt.b)

			if err != nil && tt.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && !tt.ok {
				t.Fatal("expected an error, but none occurred")
			}
			if err != nil {
				t.Logf("OK error: %v", err)
				return
			}

			if diff := cmp.Diff(tt.ns, ns); diff != "" {
				t.Fatalf("unexpected neighbor solicitation (-want +got):\n%s", diff)
			}
		})
	}
}

func mustIPv6(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		panic(fmt.Sprintf("invalid IPv6 address: %q", s))
	}

	return ip
}
