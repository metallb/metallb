package ndp_test

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/ndp"
	"github.com/mdlayher/ndp/internal/ndptest"
)

// A messageSub is a sub-test structure for Message marshal/unmarshal tests.
type messageSub struct {
	name string
	m    ndp.Message
	bs   [][]byte
	ok   bool
}

func TestMarshalParseMessage(t *testing.T) {
	tests := []struct {
		name   string
		header []byte
		subs   []messageSub
	}{
		{
			name:   "NA",
			header: []byte{136, 0x00, 0x00, 0x00},
			subs:   naTests(),
		},
		{
			name:   "NS",
			header: []byte{135, 0x00, 0x00, 0x00},
			subs:   nsTests(),
		},
		{
			name:   "RA",
			header: []byte{134, 0x00, 0x00, 0x00},
			subs:   raTests(),
		},
		{
			name:   "RS",
			header: []byte{133, 0x00, 0x00, 0x00},
			subs:   rsTests(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, st := range tt.subs {
				t.Run(st.name, func(t *testing.T) {
					b, err := ndp.MarshalMessage(st.m)

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

					// ICMPv6 header precedes the message bytes.
					ttb := append(tt.header, ndptest.Merge(st.bs)...)
					if diff := cmp.Diff(ttb, b); diff != "" {
						t.Fatalf("unexpected message bytes (-want +got):\n%s", diff)
					}

					m, err := ndp.ParseMessage(b)
					if err != nil {
						t.Fatalf("failed to unmarshal message: %v", err)
					}

					if diff := cmp.Diff(st.m, m); diff != "" {
						t.Fatalf("unexpected message (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestParseMessageError(t *testing.T) {
	type sub struct {
		name string
		bs   [][]byte
	}

	tests := []struct {
		name   string
		header []byte
		subs   []sub
	}{
		{
			name: "invalid",
			// No common header; these messages are only ICMPv6 headers.
			subs: []sub{
				{
					name: "short",
					bs: [][]byte{{
						255,
					}},
				},
				{
					name: "unknown type",
					bs: [][]byte{{
						255, 0x00, 0x00, 0x00,
					}},
				},
			},
		},
		{
			name:   "NA",
			header: []byte{136, 0x00, 0x00, 0x00},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{ndptest.Zero(16)},
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
			name:   "NS",
			header: []byte{135, 0x00, 0x00, 0x00},
			subs: []sub{
				{
					name: "bad, short",
					bs:   [][]byte{ndptest.Zero(16)},
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
			name:   "RA",
			header: []byte{134, 0x00, 0x00, 0x00},
			subs: []sub{
				{
					name: "short",
					bs:   [][]byte{{0x00}},
				},
			},
		},
		{
			name:   "RS",
			header: []byte{133, 0x00, 0x00, 0x00},
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
					ttb := append(tt.header, ndptest.Merge(st.bs)...)
					if _, err := ndp.ParseMessage(ttb); err == nil {
						t.Fatal("expected an error, but none occurred")
					}
				})
			}
		})
	}
}

func naTests() []messageSub {
	return []messageSub{
		{
			name: "bad, malformed IP address",
			m: &ndp.NeighborAdvertisement{
				TargetAddress: net.IP{192, 168, 1, 1, 0, 0},
			},
		},
		{
			name: "bad, IPv4 address",
			m: &ndp.NeighborAdvertisement{
				TargetAddress: net.IPv4(192, 168, 1, 1),
			},
		},
		{
			name: "ok, no flags",
			m: &ndp.NeighborAdvertisement{
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, router",
			m: &ndp.NeighborAdvertisement{
				Router:        true,
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0x80, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, solicited",
			m: &ndp.NeighborAdvertisement{
				Solicited:     true,
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0x40, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, override",
			m: &ndp.NeighborAdvertisement{
				Override:      true,
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0x20, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, all flags",
			m: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0xe0, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, with target LLA",
			m: &ndp.NeighborAdvertisement{
				Router:        true,
				Solicited:     true,
				Override:      true,
				TargetAddress: ndptest.IP,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Target,
						Addr:      ndptest.MAC,
					},
				},
			},
			bs: [][]byte{
				// NA message.
				{0xe0, 0x00, 0x00, 0x00},
				ndptest.IP,
				// Target LLA option.
				{0x02, 0x01},
				ndptest.MAC,
			},
			ok: true,
		},
	}
}

func nsTests() []messageSub {
	return []messageSub{
		{
			name: "bad, malformed IP address",
			m: &ndp.NeighborSolicitation{
				TargetAddress: net.IP{192, 168, 1, 1, 0, 0},
			},
		},
		{
			name: "bad, IPv4 address",
			m: &ndp.NeighborSolicitation{
				TargetAddress: net.IPv4(192, 168, 1, 1),
			},
		},
		{
			name: "ok, no options",
			m: &ndp.NeighborSolicitation{
				TargetAddress: ndptest.IP,
			},
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
				ndptest.IP,
			},
			ok: true,
		},
		{
			name: "ok, with source LLA",
			m: &ndp.NeighborSolicitation{
				TargetAddress: ndptest.IP,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      ndptest.MAC,
					},
				},
			},
			bs: [][]byte{
				// NS message.
				{0x00, 0x00, 0x00, 0x00},
				ndptest.IP,
				// Source LLA option.
				{0x01, 0x01},
				ndptest.MAC,
			},
			ok: true,
		},
	}
}

func raTests() []messageSub {
	return []messageSub{
		{
			name: "bad, reserved prf",
			m: &ndp.RouterAdvertisement{
				RouterSelectionPreference: 2,
			},
		},
		{
			name: "bad, unknown prf",
			m: &ndp.RouterAdvertisement{
				RouterSelectionPreference: 4,
			},
		},
		{
			name: "ok, no options",
			m: &ndp.RouterAdvertisement{
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
			m: &ndp.RouterAdvertisement{
				CurrentHopLimit:           10,
				ManagedConfiguration:      true,
				OtherConfiguration:        true,
				RouterSelectionPreference: ndp.Medium,
				RouterLifetime:            30 * time.Second,
				ReachableTime:             12345 * time.Millisecond,
				RetransmitTimer:           23456 * time.Millisecond,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      ndptest.MAC,
					},
					ndp.NewMTU(1280),
				},
			},
			bs: [][]byte{
				// RA message.
				{0x0a, 0xc0, 0x00, 0x1e, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x5b, 0xa0},
				// Source LLA option.
				{0x01, 0x01},
				ndptest.MAC,
				// MTU option.
				{0x05, 0x01, 0x00, 0x00},
				{0x00, 0x00, 0x05, 0x00},
			},
			ok: true,
		},
		{
			name: "ok, new flags",
			m: &ndp.RouterAdvertisement{
				MobileIPv6HomeAgent:       true,
				RouterSelectionPreference: ndp.Low,
				NeighborDiscoveryProxy:    true,
			},
			bs: [][]byte{
				{0x0, 0x3c, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			},
			ok: true,
		},
		{
			name: "ok, prf high",
			m: &ndp.RouterAdvertisement{
				RouterSelectionPreference: ndp.High,
			},
			bs: [][]byte{
				{0x0, 0x08, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
			},
			ok: true,
		},
	}
}

func rsTests() []messageSub {
	return []messageSub{
		{
			name: "ok, no options",
			m:    &ndp.RouterSolicitation{},
			bs: [][]byte{
				{0x00, 0x00, 0x00, 0x00},
			},
			ok: true,
		},
		{
			name: "ok, with source LLA",
			m: &ndp.RouterSolicitation{
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      ndptest.MAC,
					},
				},
			},
			bs: [][]byte{
				// RS message.
				{0x00, 0x00, 0x00, 0x00},
				// Source LLA option.
				{0x01, 0x01},
				ndptest.MAC,
			},
			ok: true,
		},
	}
}
