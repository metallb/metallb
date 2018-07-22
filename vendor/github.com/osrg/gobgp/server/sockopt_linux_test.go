// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// +build linux

package server

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func Test_buildTcpMD5Sig(t *testing.T) {
	s, _ := buildTcpMD5Sig("1.2.3.4", "hello")

	if unsafe.Sizeof(s) != 216 {
		t.Error("TCPM5Sig struct size is wrong", unsafe.Sizeof(s))
	}

	buf1 := make([]uint8, 216)
	p := unsafe.Pointer(&s)
	for i := uintptr(0); i < 216; i++ {
		buf1[i] = *(*byte)(unsafe.Pointer(uintptr(p) + i))
	}

	buf2 := []uint8{2, 0, 0, 0, 1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 0, 0, 0, 0, 0, 104, 101, 108, 108, 111, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	if bytes.Equal(buf1, buf2) {
		t.Log("OK")
	} else {
		t.Error("Something wrong v4")
	}
}

func Test_buildTcpMD5Sigv6(t *testing.T) {
	s, _ := buildTcpMD5Sig("fe80::4850:31ff:fe01:fc55", "helloworld")

	buf1 := make([]uint8, 216)
	p := unsafe.Pointer(&s)
	for i := uintptr(0); i < 216; i++ {
		buf1[i] = *(*byte)(unsafe.Pointer(uintptr(p) + i))
	}

	buf2 := []uint8{10, 0, 0, 0, 0, 0, 0, 0, 254, 128, 0, 0, 0, 0, 0, 0, 72, 80, 49, 255, 254, 1, 252, 85, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 0, 0, 0, 104, 101, 108, 108, 111, 119, 111, 114, 108, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	buf2[0] = syscall.AF_INET6

	if bytes.Equal(buf1, buf2) {
		t.Log("OK")
	} else {
		t.Error("Something wrong v6")
	}
}

func Test_DialTCP_FDleak(t *testing.T) {
	openFds := func() int {
		pid := os.Getpid()
		f, err := os.OpenFile(fmt.Sprintf("/proc/%d/fdinfo", pid), os.O_RDONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		names, err := f.Readdirnames(0)
		if err != nil {
			t.Fatal(err)
		}
		return len(names)
	}

	before := openFds()

	for i := 0; i < 10; i++ {
		laddr, _ := net.ResolveTCPAddr("tcp", net.JoinHostPort("127.0.0.1", "0"))
		d := TCPDialer{
			Dialer: net.Dialer{
				LocalAddr: laddr,
				Timeout:   1 * time.Second,
			},
		}
		if _, err := d.DialTCP("127.0.0.1", 1); err == nil {
			t.Fatalf("should not succeed")
		}

	}

	if after := openFds(); before != after {
		t.Fatalf("could be fd leak, %d %d", before, after)
	}
}
