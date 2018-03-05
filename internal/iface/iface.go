package iface

import (
	"fmt"
	"net"

	"github.com/golang/glog"
)

// DropReason is the reason why a packet was dropped.
type DropReason int

// Various reasons why a packet was dropped.
const (
	DropReasonNone DropReason = iota
	DropReasonClosed
	DropReasonError
	DropReasonARPReply
	DropReasonMessageType
	DropReasonNoSourceLL
	DropReasonEthernetDestination
	DropReasonAnnounceIP
	DropReasonNotLeader
)

// ByIP returns the interface that has ip.
func ByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("no interfaces found: %s", err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if ip.Equal(v.IP) {
					glog.Infof("Address found %s for interface: %v", addr.String(), i)
					return &i, nil
				}
			case *net.IPAddr:
				if ip.Equal(v.IP) {
					glog.Infof("Address found %s for interface: %v", addr.String(), i)
					return &i, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("address not found in %d interfaces", len(ifaces))
}
