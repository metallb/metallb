package ndp_test

import (
	"bytes"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

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
		{
			name: "router advertisement",
			m: &ndp.RouterAdvertisement{
				CurrentHopLimit:      10,
				ManagedConfiguration: true,
				OtherConfiguration:   true,
				RouterLifetime:       30 * time.Second,
				ReachableTime:        12345 * time.Millisecond,
				RetransmitTimer:      23456 * time.Millisecond,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
					ndp.NewMTU(1280),
				},
			},
			bs: [][]byte{
				// ICMPv6 header and RA message.
				{
					134, 0x00, 0x00, 0x00,
					0x0a, 0xc0, 0x00, 0x1e, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x5b, 0xa0,
				},
				// Source LLA option.
				{
					0x01, 0x01,
				},
				addr,
				// MTU option.
				{
					0x05, 0x01, 0x00, 0x00,
					0x00, 0x00, 0x05, 0x00,
				},
			},
		},
		{
			name: "router solicitation",
			m: &ndp.RouterSolicitation{
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
				},
			},
			bs: [][]byte{
				// ICMPv6 header and RS message.
				{
					133, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
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

			ttb := merge(tt.bs)
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
			name: "ok, neighbor solicitation",
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
		{
			name: "ok, router advertisement",
			bs: [][]byte{
				{
					134, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
			},
			m:  &ndp.RouterAdvertisement{},
			ok: true,
		},
		{
			name: "ok, router solicitation",
			bs: [][]byte{
				{
					133, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00,
				},
			},
			m:  &ndp.RouterSolicitation{},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttb := merge(tt.bs)
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

	tests := []struct {
		name string
		na   *ndp.NeighborAdvertisement
		bs   [][]byte
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
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
				ip,
			},
			ok: true,
		},
		{
			name: "ok, router",
			na: &ndp.NeighborAdvertisement{
				Router:        true,
				TargetAddress: ip,
			},
			bs: [][]byte{
				{0x80, 0x00, 0x00, 0x00},
				ip,
			},
			ok: true,
		},
		{
			name: "ok, solicited",
			na: &ndp.NeighborAdvertisement{
				Solicited:     true,
				TargetAddress: ip,
			},
			bs: [][]byte{
				{0x40, 0x00, 0x00, 0x00},
				ip,
			},
			ok: true,
		},
		{
			name: "ok, override",
			na: &ndp.NeighborAdvertisement{
				Override:      true,
				TargetAddress: ip,
			},
			bs: [][]byte{
				{0x20, 0x00, 0x00, 0x00},
				ip,
			},
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
			bs: [][]byte{
				{0xe0, 0x00, 0x00, 0x00},
				ip,
			},
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
			bs: [][]byte{
				// NA message.
				{0xe0, 0x00, 0x00, 0x00},
				ip,
				// Target LLA option.
				{
					0x02, 0x01,
					0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				},
			},
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

			ttb := merge(tt.bs)
			if diff := cmp.Diff(ttb, b); diff != "" {
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

func TestNeighborSolicitationMarshalUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8::1")
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		ns   *ndp.NeighborSolicitation
		bs   [][]byte
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
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
				ip,
			},
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
			bs: [][]byte{
				// NS message.
				[]byte{0x00, 0x00, 0x00, 0x00},
				ip,
				// Source LLA option.
				[]byte{
					0x01, 0x01,
					0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				},
			},
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

			ttb := merge(tt.bs)
			if diff := cmp.Diff(ttb, b); diff != "" {
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

func TestRouterAdvertisementMarshalUnmarshalBinary(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		ra   *ndp.RouterAdvertisement
		bs   [][]byte
		ok   bool
	}{
		{
			name: "ok, no options",
			ra: &ndp.RouterAdvertisement{
				CurrentHopLimit:      10,
				ManagedConfiguration: true,
				OtherConfiguration:   true,
				RouterLifetime:       30 * time.Second,
				ReachableTime:        12345 * time.Millisecond,
				RetransmitTimer:      23456 * time.Millisecond,
			},
			bs: [][]byte{
				{0x0a, 0xc0, 0x00, 0x1e, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x5b, 0xa0},
			},
			ok: true,
		},
		{
			name: "ok, with options",
			ra: &ndp.RouterAdvertisement{
				CurrentHopLimit:      10,
				ManagedConfiguration: true,
				OtherConfiguration:   true,
				RouterLifetime:       30 * time.Second,
				ReachableTime:        12345 * time.Millisecond,
				RetransmitTimer:      23456 * time.Millisecond,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
					ndp.NewMTU(1280),
				},
			},
			bs: [][]byte{
				// RA message.
				{0x0a, 0xc0, 0x00, 0x1e, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x5b, 0xa0},
				// Source LLA option.
				{0x01, 0x01},
				addr,
				// MTU option.
				{0x05, 0x01, 0x00, 0x00},
				{0x00, 0x00, 0x05, 0x00},
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.ra.MarshalBinary()

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

			ttb := merge(tt.bs)
			if diff := cmp.Diff(ttb, b); diff != "" {
				t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
			}

			ra := new(ndp.RouterAdvertisement)
			if err := ra.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.ra, ra); diff != "" {
				t.Fatalf("unexpected router advertisement (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRouterSolicitationMarshalUnmarshalBinary(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		rs   *ndp.RouterSolicitation
		bs   [][]byte
		ok   bool
	}{
		{
			name: "ok, no options",
			rs:   &ndp.RouterSolicitation{},
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
			},
			ok: true,
		},
		{
			name: "ok, with source LLA",
			rs: &ndp.RouterSolicitation{
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      addr,
					},
				},
			},
			bs: [][]byte{
				// RS message.
				[]byte{0x00, 0x00, 0x00, 0x00},
				// Source LLA option.
				[]byte{
					0x01, 0x01,
					0xde, 0xad, 0xbe, 0xef, 0xde, 0xad,
				},
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.rs.MarshalBinary()

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

			ttb := merge(tt.bs)
			if diff := cmp.Diff(ttb, b); diff != "" {
				t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
			}

			rs := new(ndp.RouterSolicitation)
			if err := rs.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.rs, rs); diff != "" {
				t.Fatalf("unexpected router solicitation (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMessageUnmarshalBinaryError(t *testing.T) {
	type sub struct {
		name string
		bs   [][]byte
	}

	tests := []struct {
		name string
		m    ndp.Message
		subs []sub
	}{
		{
			name: "NA",
			m:    &ndp.NeighborAdvertisement{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{zero(16)},
				},
				{
					name: "IPv4",
					bs: [][]byte{
						{0xe0, 0x00, 0x00, 0x00},
						net.IPv4(192, 168, 1, 1),
					},
				},
			},
		},
		{
			name: "NS",
			m:    &ndp.NeighborSolicitation{},
			subs: []sub{
				{
					name: "bad, short",
					bs:   [][]byte{zero(16)},
				},
				{
					name: "bad, IPv4",
					bs: [][]byte{
						{0xe0, 0x00, 0x00, 0x00},
						net.IPv4(192, 168, 1, 1),
					},
				},
			},
		},
		{
			name: "RA",
			m:    &ndp.RouterAdvertisement{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x00}},
				},
			},
		},
		{
			name: "RS",
			m:    &ndp.RouterSolicitation{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x00}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subs {
				t.Run(st.name, func(t *testing.T) {
					err := tt.m.UnmarshalBinary(merge(st.bs))

					if err == nil {
						t.Fatal("expected an error, but none occurred")
					} else {
						t.Logf("OK error: %v", err)
						return
					}
				})
			}
		})
	}
}

func merge(bs [][]byte) []byte {
	var b []byte
	for _, bb := range bs {
		b = append(b, bb...)
	}

	return b
}

func zero(n int) []byte {
	return bytes.Repeat([]byte{0x00}, n)
}

func mustIPv6(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		panic(fmt.Sprintf("invalid IPv6 address: %q", s))
	}

	return ip
}
