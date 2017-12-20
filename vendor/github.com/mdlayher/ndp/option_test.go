package ndp_test

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp"
)

func TestLinkLayerAddressMarshalUnmarshalBinary(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		lla  *ndp.LinkLayerAddress
		b    []byte
		ok   bool
	}{
		{
			name: "bad, invalid direction",
			lla: &ndp.LinkLayerAddress{
				Direction: 10,
			},
		},
		{
			name: "bad, invalid address",
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      net.HardwareAddr{0xde, 0xad, 0xbe, 0xef},
			},
		},
		{
			name: "ok, source",
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      addr,
			},
			b: append([]byte{
				0x01,
				0x01,
			}, addr...),
			ok: true,
		},
		{
			name: "ok, target",
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      addr,
			},
			b: append([]byte{
				0x02,
				0x01,
			}, addr...),
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.lla.MarshalBinary()

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
				t.Fatalf("unexpected Option bytes (-want +got):\n%s", diff)
			}

			lla := new(ndp.LinkLayerAddress)
			if err := lla.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.lla, lla); diff != "" {
				t.Fatalf("unexpected link-layer address (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLinkLayerAddressUnmarshalBinary(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		b    []byte
		lla  *ndp.LinkLayerAddress
		ok   bool
	}{
		{
			name: "bad, short",
			b:    addr,
		},
		{
			name: "bad, invalid direction",
			b: append([]byte{
				0x10,
				0x01,
			}, addr...),
		},
		{
			name: "bad, invalid length",
			b: append([]byte{
				0x01,
				0x10,
			}, addr...),
		},
		{
			name: "ok, source",
			b: append([]byte{
				0x01,
				0x01,
			}, addr...),
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      addr,
			},
			ok: true,
		},
		{
			name: "ok, target",
			b: append([]byte{
				0x02,
				0x01,
			}, addr...),
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      addr,
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lla := new(ndp.LinkLayerAddress)
			err := lla.UnmarshalBinary(tt.b)

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

			if diff := cmp.Diff(tt.lla, lla); diff != "" {
				t.Fatalf("unexpected link-layer address (-want +got):\n%s", diff)
			}
		})
	}
}
