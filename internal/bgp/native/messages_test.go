// SPDX-License-Identifier:Apache-2.0

package native

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.universe.tf/metallb/internal/bgp"
	"go.universe.tf/metallb/internal/bgp/community"
)

// Just test that sendOpen and readOpen can at least talk to each other.
func TestOpen(t *testing.T) {
	var b bytes.Buffer
	wantHold := 4 * time.Second
	wantASN := uint32(12345)
	if err := sendOpen(&b, wantASN, net.ParseIP("1.2.3.4"), wantHold); err != nil {
		t.Fatalf("Send open: %s", err)
	}
	op, err := readOpen(&b)
	if err != nil {
		t.Fatalf("Read open: %s", err)
	}
	if op.holdTime != wantHold {
		t.Errorf("Wrong hold-time, want %q, got %q", wantHold, op.holdTime)
	}
	if op.asn != wantASN {
		t.Errorf("Wrong ASN, want %d, got %d", wantASN, op.asn)
	}
}

func TestPcapInterop(t *testing.T) {
	ms, err := filepath.Glob("testdata/open-*")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		bs, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read %q: %s", m, err)
		}
		b := bytes.NewBuffer(bs)
		_, err = readOpen(b)
		if err != nil {
			t.Errorf("Read %q: %s", m, err)
		}
	}
}

func TestOpenFourByteASN(t *testing.T) {
	tests := []struct {
		fbasn    bool
		asn      uint32
		openData []byte
	}{
		{
			// BGP OPEN from MetalLB, with 4-byte ASN support, running on 2-byte ASN
			true,
			65002,
			[]byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0x00, 0x31, 0x01, 0x04, 0xfd, 0xea, 0x00, 0x5a, 0xb9, 0xec, 0xf0, 0x27, 0x14, 0x02, 0x12, 0x01,
				0x04, 0x00, 0x01, 0x00, 0x01, 0x01, 0x04, 0x00, 0x02, 0x00, 0x01, 0x41, 0x04, 0x00, 0x00, 0xfd,
				0xea,
			},
		},
		{
			// BGP OPEN from Arista EOS 4.13.10M, no 4-byte ASN support
			false,
			65001,
			[]byte{
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0x00, 0x2b, 0x01, 0x04, 0xfd, 0xe9, 0x00, 0xb4, 0xb9, 0xec, 0xf0, 0x40, 0x0e, 0x02,
				0x0c, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, 0x02, 0x00, 0x40, 0x02, 0x00, 0xb4,
			},
		},
	}

	for i, test := range tests {
		b := bytes.NewBuffer(test.openData)
		open, err := readOpen(b)
		if err != nil {
			t.Errorf("%d: readOpen: %v", i, err)
			continue
		}

		if want, got := test.fbasn, open.fbasn; want != got {
			t.Errorf("%d: OPEN 4-byte ASN capability is %v, wanted %v", i, got, want)
		}
		if want, got := test.asn, open.asn; want != got {
			t.Errorf("%d: OPEN ASN is %v, wanted %v", i, got, want)
		}
	}
}

// TestSendUpdate makes sure that sendUpdate fails if large communities are provided. The E2E tests take care of
// testing further functionality. A decodeUpdate method would be needed for a more complete unit test.
func TestSendUpdate(t *testing.T) {
	tcs := map[string]struct {
		asn         uint32
		ibgp        bool
		fbasn       bool
		nextHop     net.IP
		adv         *bgp.Advertisement
		errorString string
	}{
		"send update with legacy communities should succeed": {
			asn:     65000,
			ibgp:    false,
			fbasn:   false,
			nextHop: net.ParseIP("192.168.123.10"),
			adv: &bgp.Advertisement{
				Prefix: func() *net.IPNet {
					_, ipnet, _ := net.ParseCIDR("172.16.0.0/24")
					return ipnet
				}(),
				LocalPref: 100,
				Communities: func() []community.BGPCommunity {
					c1, _ := community.New("0:1234")
					c2, _ := community.New("0:2345")
					return []community.BGPCommunity{c1, c2}
				}(),
				Peers: []string{},
			},
			errorString: "",
		},
		"send update with large communities should result in an error": {
			asn:     65000,
			ibgp:    false,
			fbasn:   false,
			nextHop: net.ParseIP("192.168.123.10"),
			adv: &bgp.Advertisement{
				Prefix: func() *net.IPNet {
					_, ipnet, _ := net.ParseCIDR("172.16.0.0/24")
					return ipnet
				}(),
				LocalPref: 100,
				Communities: func() []community.BGPCommunity {
					c1, _ := community.New("0:1234")
					c2, _ := community.New("large:123:234:567")
					return []community.BGPCommunity{c1, c2}
				}(),
				Peers: []string{},
			},
			errorString: "invalid community type for BGP native mode",
		},
	}
	for d, tc := range tcs {
		var b bytes.Buffer
		err := sendUpdate(&b, tc.asn, tc.ibgp, tc.fbasn, tc.nextHop, tc.adv)
		if tc.errorString == "" && err != nil {
			t.Fatalf("%s(%s): send update, err: %q", t.Name(), d, err)
		}
		if tc.errorString != "" && (err == nil || !strings.Contains(err.Error(), tc.errorString)) {
			t.Fatalf("%s(%s): send update expected to see error %q but got %q instead", t.Name(), d, tc.errorString, err)
		}
	}
}

func FuzzReadOpen(f *testing.F) {
	ms, err := filepath.Glob("testdata/open-*")
	if err != nil {
		f.Fatal(err)
	}

	for _, m := range ms {
		bs, err := os.ReadFile(m)
		if err != nil {
			f.Fatalf("read %q: %s", m, err)
		}
		f.Add(bs)
	}

	input0 := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x31, 0x01, 0x04, 0xfd, 0xea, 0x00, 0x5a, 0xb9, 0xec, 0xf0, 0x27, 0x14, 0x02, 0x12, 0x01,
		0x04, 0x00, 0x01, 0x00, 0x01, 0x01, 0x04, 0x00, 0x02, 0x00, 0x01, 0x41, 0x04, 0x00, 0x00, 0xfd,
		0xea,
	}

	input1 := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x2b, 0x01, 0x04, 0xfd, 0xe9, 0x00, 0xb4, 0xb9, 0xec, 0xf0, 0x40, 0x0e, 0x02,
		0x0c, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, 0x02, 0x00, 0x40, 0x02, 0x00, 0xb4,
	}
	f.Add(input0)
	f.Add(input1)

	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = readOpen(bytes.NewBuffer(input))
	})
}
