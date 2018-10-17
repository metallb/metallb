package bgp

import (
	"fmt"
	"io"
	"net"
)

type mhIpv6 int

func (mh mhIpv6) sendUpdate(w io.Writer, asn uint32, ibgp bool, defaultNextHop net.IP, adv *Advertisement) error {
	fmt.Println("NYI ipv6 sendUpdate")
	return nil
}
