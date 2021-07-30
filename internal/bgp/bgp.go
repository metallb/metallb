package bgp

import (
	"net"
	"reflect"
)

// Advertisement represents one network path and its BGP attributes.
type Advertisement struct {
	// The prefix being advertised to the peer.
	Prefix *net.IPNet
	// The address of the router to which the peer should forward traffic.
	NextHop net.IP
	// The local preference of this route. Only propagated to IBGP
	// peers (i.e. where the peer ASN matches the local ASN).
	LocalPref uint32
	// BGP communities to attach to the path.
	Communities []uint32
}

// Equal returns true if a and b are equivalent advertisements.
func (a *Advertisement) Equal(b *Advertisement) bool {
	if a.Prefix.String() != b.Prefix.String() {
		return false
	}
	if !a.NextHop.Equal(b.NextHop) {
		return false
	}
	if a.LocalPref != b.LocalPref {
		return false
	}
	return reflect.DeepEqual(a.Communities, b.Communities)
}
