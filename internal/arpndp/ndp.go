package arpndp

import (
	"fmt"
	"net"

	"github.com/golang/glog"
	"github.com/mdlayher/ndp"
	"go.universe.tf/metallb/internal/iface"
)

type ndpResponder struct {
	hardwareAddr net.HardwareAddr
	conn         *ndp.Conn
	announce     announceFunc
}

func newNDP(ifi *net.Interface, ann announceFunc) (*ndpResponder, error) {
	// Use link-local address as the source IPv6 address for NDP communications.
	conn, _, err := ndp.Dial(ifi, ndp.LinkLocal)
	if err != nil {
		return nil, fmt.Errorf("creating NDP responder for %q: %s", ifi.Name, err)
	}

	ret := &ndpResponder{
		hardwareAddr: ifi.HardwareAddr,
		conn:         conn,
		announce:     ann,
	}
	go ret.run()
	return ret, nil
}

func (n *ndpResponder) Close() error {
	return n.conn.Close()
}

func (n *ndpResponder) Gratuitous(ip net.IP) error {
	err := n.advertise(net.IPv6linklocalallnodes, ip, true)
	stats.SentGratuitous(ip.String())
	return err
}

func (n *ndpResponder) run() {
	for n.processRequest() != iface.DropReasonClosed {
	}
}

func (n *ndpResponder) processRequest() iface.DropReason {
	msg, _, src, err := n.conn.ReadFrom()
	if err != nil {
		return iface.DropReasonError
	}

	ns, ok := msg.(*ndp.NeighborSolicitation)
	if !ok {
		return iface.DropReasonMessageType
	}

	// Retrieve sender's source link-layer address
	var nsLLAddr net.HardwareAddr
	for _, o := range ns.Options {
		// Ignore other options, including target link-layer address instead of source.
		lla, ok := o.(*ndp.LinkLayerAddress)
		if !ok {
			continue
		}
		if lla.Direction != ndp.Source {
			continue
		}

		nsLLAddr = lla.Addr
		break
	}
	if nsLLAddr == nil {
		return iface.DropReasonNoSourceLL
	}

	// Ignore NDP requests that the announcer tells us to ignore.
	if reason := n.announce(ns.TargetAddress); reason != iface.DropReasonNone {
		return reason
	}

	stats.GotRequest(ns.TargetAddress.String())

	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", ns.TargetAddress, src, nsLLAddr, ns.TargetAddress, n.hardwareAddr)

	if err := n.advertise(src, ns.TargetAddress, false); err != nil {
		glog.Warningf("Failed to write NDP neighbor advertisement for %s: %s", ns.TargetAddress, err)
	} else {
		stats.SentResponse(ns.TargetAddress.String())
	}
	return iface.DropReasonNone
}

func (n *ndpResponder) advertise(dst, target net.IP, gratuitous bool) error {
	m := &ndp.NeighborAdvertisement{
		Solicited:     !gratuitous, // <Adam Jensen> I never asked for this...
		Override:      gratuitous,  // Should clients replace existing cache entries
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      n.hardwareAddr,
			},
		},
	}
	return n.conn.WriteTo(m, nil, dst)
}
