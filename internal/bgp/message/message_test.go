package message

import (
	"bytes"
	"encoding"
	"io/ioutil"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func globs(paths ...string) ([]string, error) {
	var ret []string
	for _, p := range paths {
		ms, err := filepath.Glob(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, ms...)
	}
	return ret, nil
}

func TestFuzzCrashers(t *testing.T) {
	ms, err := globs("fuzz-data/crashers/*", "samples/*")
	if err != nil {
		t.Fatalf("glob: %s", err)
	}

	for _, f := range ms {
		if strings.HasSuffix(f, ".output") || strings.HasSuffix(f, ".quoted") {
			continue
		}

		bs, err := ioutil.ReadFile(f)
		if err != nil {
			t.Errorf("read %q: %s", f, err)
			continue
		}

		b := bytes.NewBuffer(bs)
		m, err := Decode(b)
		if err != nil {
			t.Errorf("decoding %q failed: %s", f, err)
			continue
		}
		if b.Len() != 0 {
			t.Errorf("%q has %d trailing garbage bytes after packet", f, b.Len())
			continue
		}

		bs2, err := m.(encoding.BinaryMarshaler).MarshalBinary()
		if err != nil {
			t.Errorf("%q decoded as %#v, but cannot reencode: %s", f, m, err)
			continue
		}
		if !bytes.Equal(bs, bs2) {
			t.Errorf("decode+encode of %q is not idempotent\nwant %#v\ngot  %#v\ndecoded was %#v", f, bs, bs2, m)
			ioutil.WriteFile("foo1", bs, 0644)
			ioutil.WriteFile("foo2", bs2, 0644)
		}
	}
}

func TestMsgSerialization(t *testing.T) {
	tests := []struct {
		desc           string
		msg            encoding.BinaryMarshaler
		wantMarshalErr bool
	}{
		{
			desc: "valid OPEN",
			msg: &Open{
				ASN:      1234,
				HoldTime: 42 * time.Second,
				RouterID: net.ParseIP("2.3.4.5").To4(),
			},
		},
		{
			desc: "bad OPEN ASN",
			msg: &Open{
				ASN:      0,
				HoldTime: 42 * time.Second,
				RouterID: net.ParseIP("2.3.4.5").To4(),
			},
			wantMarshalErr: true,
		},
		{
			desc: "bad OPEN HoldTime",
			msg: &Open{
				ASN:      1234,
				HoldTime: 1 * time.Second,
				RouterID: net.ParseIP("2.3.4.5").To4(),
			},
			wantMarshalErr: true,
		},
		{
			desc: "bad OPEN RouterID",
			msg: &Open{
				ASN:      1234,
				HoldTime: 42 * time.Second,
				RouterID: net.ParseIP("::1"),
			},
			wantMarshalErr: true,
		},
		{
			desc: "nil OPEN RouterID",
			msg: &Open{
				ASN:      1234,
				HoldTime: 42 * time.Second,
				RouterID: nil,
			},
			wantMarshalErr: true,
		},
		{
			desc: "Keepalive",
			msg:  &Keepalive{},
		},
	}

	for _, test := range tests {
		bs, err := test.msg.MarshalBinary()
		if err != nil {
			if !test.wantMarshalErr {
				t.Errorf("%s: marshal message: %s", test.desc, err)
			}
			continue
		}

		buf := bytes.NewBuffer(bs)
		v, err := Decode(buf)
		if err != nil {
			t.Errorf("%s: decode message: %s", test.desc, err)
			continue
		}
		if buf.Len() != 0 {
			t.Errorf("%s: decode did not consume all bytes", test.desc)
		}
		if !reflect.DeepEqual(v, test.msg) {
			t.Errorf("%s: encode+decode is not idempotent, want %#v, got %#v", test.desc, test.msg, v)
		}
	}
}
