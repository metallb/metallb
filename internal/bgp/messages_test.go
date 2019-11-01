package bgp

import (
	"bytes"
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"
	"time"
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
		bs, err := ioutil.ReadFile(m)
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
