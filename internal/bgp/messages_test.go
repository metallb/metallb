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
