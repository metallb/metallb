// +build ignore

package main

import (
	"encoding"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.universe.tf/metallb/internal/bgp/message"
)

var (
	workdir = flag.String("workdir", "fuzz-data/corpus", "Fuzz corpus directory")
	num     = flag.Int("num", 100000, "number of samples to generate")
)

func fatalf(msg string, args ...interface{}) {
	fmt.Printf(msg+"\n", args...)
	os.Exit(1)
}

func ip(r *rand.Rand) net.IP {
	ip := make([]byte, 4)
	if _, err := r.Read(ip); err != nil {
		fatalf("generating IP: %s", err)
	}
	return net.IP(ip)
}

func ipnet(r *rand.Rand) *net.IPNet {
	ret := &net.IPNet{
		IP:   ip(r),
		Mask: net.CIDRMask(r.Intn(33), 32),
	}
	ret.IP = ret.IP.Mask(ret.Mask)
	return ret
}

func write(fname string, v encoding.BinaryMarshaler) {
	bs, err := v.MarshalBinary()
	if err != nil {
		fatalf("Marshal for %q: %s", fname, err)
	}
	if err = ioutil.WriteFile(fmt.Sprintf("%s/%s", *workdir, fname), bs, 0644); err != nil {
		fatalf("Write %q failed: %s", fname, err)
	}
}

func main() {
	flag.Parse()

	ms, err := filepath.Glob(fmt.Sprintf("%s/autogen-*", *workdir))
	if err != nil {
		fatalf("Glob for genfiles: %s", err)
	}

	for _, m := range ms {
		if err = os.Remove(m); err != nil {
			fatalf("Removing old genfile %q: %s", m, err)
		}
	}

	r := rand.New(rand.NewSource(42))

	for i := 0; i < *num; i++ {
		ip := make([]byte, 4)
		if _, err = r.Read(ip); err != nil {
			fatalf("generating IP: %s", err)
		}
		m := &message.Open{
			ASN:      r.Uint32(),
			HoldTime: time.Duration(rand.Uint32()) * time.Second,
			RouterID: net.IP(ip),
		}
		write(fmt.Sprintf("autogen-open-%d", i), m)
	}

	write("autogen-keepalive", &message.Keepalive{})

	for i := 0; i < 65535; i += 41 {
		n := &message.Notification{
			Code: message.ErrorCode(i),
			Data: make([]byte, r.Intn(100)),
		}
		if _, err := r.Read(n.Data); err != nil {
			fatalf("generating notification data: %s", err)
		}
		write(fmt.Sprintf("autogen-notification-%d", i), n)
	}

	for i := 0; i < *num; i++ {
		u := &message.Update{}
		nWdr := r.Intn(100)
		for j := 0; j < nWdr; j++ {
			u.Withdraw = append(u.Withdraw, ipnet(r))
		}
		nAdv := r.Intn(100)
		for j := 0; j < nAdv; j++ {
			u.Advertise = append(u.Advertise, ipnet(r))
		}
		nAttr := r.Intn(100)
		for j := 0; j < nAttr; j++ {
			l := r.Intn(100)
			a := message.Attribute{
				Code: uint16(r.Uint32()),
				Data: make([]byte, l),
			}
			if _, err := r.Read(a.Data); err != nil {
				fatalf("generating attribute data: %s", err)
			}
			u.Attributes = append(u.Attributes, a)
		}
		write(fmt.Sprintf("autogen-update-%d", i), u)
	}
}
