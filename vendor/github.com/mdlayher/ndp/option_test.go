package ndp_test

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp"
)

func TestLinkLayerAddressMarshalUnmarshalBinary(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	tests := []struct {
		name string
		lla  *ndp.LinkLayerAddress
		bs   [][]byte
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
			bs: [][]byte{
				{0x01, 0x01},
				addr,
			},
			ok: true,
		},
		{
			name: "ok, target",
			lla: &ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      addr,
			},
			bs: [][]byte{
				{0x02, 0x01},
				addr,
			},
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

			ttb := merge(tt.bs)
			if diff := cmp.Diff(ttb, b); diff != "" {
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

func TestMTUMarshalUnmarshalBinary(t *testing.T) {
	tests := []struct {
		name string
		m    *ndp.MTU
		bs   [][]byte
		ok   bool
	}{
		{
			name: "ok",
			m:    ndp.NewMTU(1500),
			bs: [][]byte{
				{0x05, 0x01, 0x00, 0x00},
				{0x00, 0x00, 0x05, 0xdc},
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.m.MarshalBinary()

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
				t.Fatalf("unexpected Option bytes (-want +got):\n%s", diff)
			}

			m := new(ndp.MTU)
			if err := m.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.m, m); diff != "" {
				t.Fatalf("unexpected MTU (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrefixInformationMarshalUnmarshalBinary(t *testing.T) {
	ip := mustIPv6("2001:db8::1")
	prefix := mustIPv6("2001:db8::")

	tests := []struct {
		name string
		pi   *ndp.PrefixInformation
		bs   [][]byte
		ok   bool
	}{
		{
			name: "bad, prefix length",
			pi: &ndp.PrefixInformation{
				// Host IP specified.
				PrefixLength: 64,
				Prefix:       ip,
			},
		},
		{
			name: "ok",
			pi: &ndp.PrefixInformation{
				// Prefix IP specified.
				PrefixLength: 32,
				OnLink:       true,
				AutonomousAddressConfiguration: true,
				ValidLifetime:                  ndp.Infinity,
				PreferredLifetime:              20 * time.Minute,
				Prefix:                         prefix,
			},
			bs: [][]byte{
				// Option type and length.
				{0x03, 0x04},
				// Prefix Length.
				{32},
				// Flags, O and A set.
				{0xc0},
				// Valid lifetime.
				{0xff, 0xff, 0xff, 0xff},
				// Preferred lifetime.
				{0x00, 0x00, 0x04, 0xb0},
				// Reserved.
				{0x00, 0x00, 0x00, 0x00},
				// Prefix.
				prefix,
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.pi.MarshalBinary()

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
				t.Fatalf("unexpected Option bytes (-want +got):\n%s", diff)
			}

			pi := new(ndp.PrefixInformation)
			if err := pi.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.pi, pi); diff != "" {
				t.Fatalf("unexpected prefix information (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrefixInformationUnmarshalBinaryPrefixLength(t *testing.T) {
	prefix := mustIPv6("2001:db8::")
	l := uint8(16)

	bs := [][]byte{
		// Option type and length.
		{0x03, 0x04},
		// Prefix Length, shorter than the prefix itself, so the prefix
		// should be cut off.
		{l},
		// Flags, O and A set.
		{0xc0},
		// Valid lifetime.
		{0x00, 0x00, 0x02, 0x58},
		// Preferred lifetime.
		{0x00, 0x00, 0x04, 0xb0},
		// Reserved.
		{0x00, 0x00, 0x00, 0x00},
		// Prefix.
		prefix,
	}

	pi := new(ndp.PrefixInformation)
	if err := pi.UnmarshalBinary(merge(bs)); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Assume that unmarshaling ignores any prefix bits longer than the
	// specified length.
	want := mustIPv6("2001::")

	if diff := cmp.Diff(want, pi.Prefix); diff != "" {
		t.Fatalf("unexpected prefix (-want +got):\n%s", diff)
	}
}

func TestRawOptionMarshalUnmarshalBinary(t *testing.T) {
	tests := []struct {
		name string
		ro   *ndp.RawOption
		bs   [][]byte
		ok   bool
	}{
		{
			name: "bad, length",
			ro: &ndp.RawOption{
				Type:   1,
				Length: 1,
				Value:  zero(7),
			},
		},
		{
			name: "ok",
			ro: &ndp.RawOption{
				Type:   10,
				Length: 2,
				Value:  zero(14),
			},
			bs: [][]byte{
				{0x0a, 0x02},
				zero(14),
			},
			ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.ro.MarshalBinary()

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
				t.Fatalf("unexpected Option bytes (-want +got):\n%s", diff)
			}

			ro := new(ndp.RawOption)
			if err := ro.UnmarshalBinary(b); err != nil {
				t.Fatalf("failed to unmarshal binary: %v", err)
			}

			if diff := cmp.Diff(tt.ro, ro); diff != "" {
				t.Fatalf("unexpected raw option (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptionUnmarshalBinaryError(t *testing.T) {
	addr := net.HardwareAddr{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad}

	type sub struct {
		name string
		bs   [][]byte
	}

	tests := []struct {
		name string
		o    ndp.Option
		subs []sub
	}{
		{
			name: "raw option",
			o:    &ndp.RawOption{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01}},
				},
				{
					name: "misleading length",
					bs:   [][]byte{{0x10, 0x10}},
				},
			},
		},
		{
			name: "link layer address",
			o:    &ndp.LinkLayerAddress{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01, 0x01, 0xff}},
				},
				{
					name: "invalid direction",
					bs: [][]byte{
						{0x10, 0x01},
						addr,
					},
				},
				{
					name: "long",
					bs: [][]byte{
						{0x01, 0x02},
						zero(16),
					},
				},
			},
		},
		{
			name: "mtu",
			o:    new(ndp.MTU),
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01}},
				},
			},
		},
		{
			name: "prefix information",
			o:    &ndp.PrefixInformation{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subs {
				t.Run(st.name, func(t *testing.T) {
					err := tt.o.UnmarshalBinary(merge(st.bs))

					if err == nil {
						t.Fatal("expected an error, but none occurred")
					} else {
						t.Logf("OK error: %v", err)
					}
				})
			}
		})
	}
}
