package ndp

// Package ndp_test not used because we need access to direct option marshaling
// and unmarshaling functions.

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp/internal/ndptest"
)

// An optionSub is a sub-test structure for Option marshal/unmarshal tests.
type optionSub struct {
	name string
	os   []Option
	bs   [][]byte
	ok   bool
}

func TestOptionMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		subs []optionSub
	}{
		{
			name: "raw option",
			subs: roTests(),
		},
		{
			name: "link layer address",
			subs: llaTests(),
		},
		{
			name: "MTU",
			subs: []optionSub{{
				name: "ok",
				os: []Option{
					NewMTU(1500),
				},
				bs: [][]byte{
					{0x05, 0x01, 0x00, 0x00},
					{0x00, 0x00, 0x05, 0xdc},
				},
				ok: true,
			}},
		},
		{
			name: "prefix information",
			subs: piTests(),
		},
		{
			name: "recursive DNS servers",
			subs: rdnssTests(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subs {
				t.Run(st.name, func(t *testing.T) {
					b, err := marshalOptions(st.os)

					if err != nil && st.ok {
						t.Fatalf("unexpected error: %v", err)
					}
					if err == nil && !st.ok {
						t.Fatal("expected an error, but none occurred")
					}
					if err != nil {
						t.Logf("OK error: %v", err)
						return
					}

					ttb := ndptest.Merge(st.bs)
					if diff := cmp.Diff(ttb, b); diff != "" {
						t.Fatalf("unexpected options bytes (-want +got):\n%s", diff)
					}

					got, err := parseOptions(b)
					if err != nil {
						t.Fatalf("failed to unmarshal options: %v", err)
					}

					if diff := cmp.Diff(st.os, got); diff != "" {
						t.Fatalf("unexpected options (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestOptionUnmarshalError(t *testing.T) {
	type sub struct {
		name string
		bs   [][]byte
	}

	tests := []struct {
		name string
		o    Option
		subs []sub
	}{
		{
			name: "raw option",
			o:    &RawOption{},
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
			o:    &LinkLayerAddress{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01, 0x01, 0xff}},
				},
				{
					name: "invalid direction",
					bs: [][]byte{
						{0x10, 0x01},
						ndptest.MAC,
					},
				},
				{
					name: "long",
					bs: [][]byte{
						{0x01, 0x02},
						ndptest.Zero(16),
					},
				},
			},
		},
		{
			name: "mtu",
			o:    new(MTU),
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01}},
				},
			},
		},
		{
			name: "prefix information",
			o:    &PrefixInformation{},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x01}},
				},
			},
		},
		{
			name: "rdnss",
			o:    &RecursiveDNSServer{},
			subs: []sub{
				{
					name: "no servers",
					bs: [][]byte{
						{25, 1},
						// Reserved.
						{0x00, 0x00},
						// Lifetime.
						ndptest.Zero(4),
						// No servers.
					},
				},
				{
					name: "bad first server",
					bs: [][]byte{
						{25, 2},
						// Reserved.
						{0x00, 0x00},
						// Lifetime.
						ndptest.Zero(4),
						// First server, half an IPv6 address.
						ndptest.Zero(8),
					},
				},
				{
					name: "bad second server",
					bs: [][]byte{
						{25, 4},
						// Reserved.
						{0x00, 0x00},
						// Lifetime.
						ndptest.Zero(4),
						// First server.
						ndptest.Zero(16),
						// Second server, half an IPv6 address.
						ndptest.Zero(8),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subs {
				t.Run(st.name, func(t *testing.T) {
					err := tt.o.unmarshal(ndptest.Merge(st.bs))

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

func TestPrefixInformationUnmarshalPrefixLength(t *testing.T) {
	// Assume that unmarshaling ignores any prefix bits longer than the
	// specified length.
	var (
		prefix = ndptest.MustIPv6("2001:db8::")
		l      = uint8(16)
		want   = ndptest.MustIPv6("2001::")
	)

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

	pi := new(PrefixInformation)
	if err := pi.unmarshal(ndptest.Merge(bs)); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Assume that unmarshaling ignores any prefix bits longer than the
	// specified length.
	if diff := cmp.Diff(want, pi.Prefix); diff != "" {
		t.Fatalf("unexpected prefix (-want +got):\n%s", diff)
	}
}

func llaTests() []optionSub {
	return []optionSub{
		{
			name: "bad, invalid direction",
			os: []Option{
				&LinkLayerAddress{
					Direction: 10,
				},
			},
		},
		{
			name: "bad, invalid address",
			os: []Option{
				&LinkLayerAddress{
					Direction: Source,
					Addr:      net.HardwareAddr{0xde, 0xad, 0xbe, 0xef},
				},
			},
		},
		{
			name: "ok, source",
			os: []Option{
				&LinkLayerAddress{
					Direction: Source,
					Addr:      ndptest.MAC,
				},
			},
			bs: [][]byte{
				{0x01, 0x01},
				ndptest.MAC,
			},
			ok: true,
		},
		{
			name: "ok, target",
			os: []Option{
				&LinkLayerAddress{
					Direction: Target,
					Addr:      ndptest.MAC,
				},
			},
			bs: [][]byte{
				{0x02, 0x01},
				ndptest.MAC,
			},
			ok: true,
		},
	}
}

func piTests() []optionSub {
	return []optionSub{
		{
			name: "bad, prefix length",
			os: []Option{
				&PrefixInformation{
					// Host IP specified.
					PrefixLength: 64,
					Prefix:       ndptest.IP,
				},
			},
		},
		{
			name: "ok",
			os: []Option{
				&PrefixInformation{
					// Prefix IP specified.
					PrefixLength: 32,
					OnLink:       true,
					AutonomousAddressConfiguration: true,
					ValidLifetime:                  Infinity,
					PreferredLifetime:              20 * time.Minute,
					Prefix:                         ndptest.Prefix,
				},
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
				ndptest.Prefix,
			},
			ok: true,
		},
	}
}

func roTests() []optionSub {
	return []optionSub{
		{
			name: "bad, length",
			os: []Option{
				&RawOption{
					Type:   1,
					Length: 1,
					Value:  ndptest.Zero(7),
				},
			},
		},
		{
			name: "ok",
			os: []Option{
				&RawOption{
					Type:   10,
					Length: 2,
					Value:  ndptest.Zero(14),
				},
			},
			bs: [][]byte{
				{0x0a, 0x02},
				ndptest.Zero(14),
			},
			ok: true,
		},
	}
}

func rdnssTests() []optionSub {
	first := net.ParseIP("2001:db8::1")
	second := net.ParseIP("2001:db8::2")

	return []optionSub{
		{
			name: "bad, no servers",
			os: []Option{
				&RecursiveDNSServer{
					Lifetime: 1 * time.Second,
				},
			},
		},
		{
			name: "ok, one server",
			os: []Option{
				&RecursiveDNSServer{
					Lifetime: 1 * time.Hour,
					Servers: []net.IP{
						first,
					},
				},
			},
			bs: [][]byte{
				{25, 3},
				{0x00, 0x00},
				{0x00, 0x00, 0x0e, 0x10},
				first,
			},
			ok: true,
		},
		{
			name: "ok, two servers",
			os: []Option{
				&RecursiveDNSServer{
					Lifetime: 24 * time.Hour,
					Servers: []net.IP{
						first,
						second,
					},
				},
			},
			bs: [][]byte{
				{25, 5},
				{0x00, 0x00},
				{0x00, 0x01, 0x51, 0x80},
				first,
				second,
			},
			ok: true,
		},
	}
}
