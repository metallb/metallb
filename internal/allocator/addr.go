package allocator // import "go.universe.tf/metallb/internal/allocator"

import (
	"net"
)

// Addr holds either an *net.TCPAddr or *net.UDPAddr.
type Addr struct {
	net.Addr
}

func (a *Addr) String() string {
	switch a.Addr.(type) {
	case *net.TCPAddr:
		return "tcp://" + a.Addr.String()
	case *net.UDPAddr:
		return "udp://" + a.Addr.String()
	}
	// default to TCP
	return "tcp://" + a.Addr.String()
}

// Equal return true if the IP in a is equal to ip.
func (a *Addr) Equal(ip net.IP) bool { return a.IP().Equal(ip) }

// IP returns the IP address of a.
func (a *Addr) IP() net.IP {
	switch x := a.Addr.(type) {
	case *net.TCPAddr:
		return x.IP
	case *net.UDPAddr:
		return x.IP
	}
	return nil
}
