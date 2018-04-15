package layer2

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/golang/glog"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

type announceFunc func(net.IP) dropReason

type arpResponder struct {
	intf         string
	hardwareAddr net.HardwareAddr
	conn         *arp.Client
	closed       chan struct{}
	announce     announceFunc
}

func newARPResponder(ifi *net.Interface, ann announceFunc) (*arpResponder, error) {
	client, err := arp.Dial(ifi)
	if err != nil {
		return nil, fmt.Errorf("creating ARP responder for %q: %s", ifi.Name, err)
	}

	ret := &arpResponder{
		intf:         ifi.Name,
		hardwareAddr: ifi.HardwareAddr,
		conn:         client,
		closed:       make(chan struct{}),
		announce:     ann,
	}
	go ret.run()
	return ret, nil
}

func (a *arpResponder) Interface() string { return a.intf }

func (a *arpResponder) Close() error {
	close(a.closed)
	return a.conn.Close()
}

func (a *arpResponder) Gratuitous(ip net.IP) error {
	for _, op := range []arp.Operation{arp.OperationRequest, arp.OperationReply} {
		pkt, err := arp.NewPacket(op, a.hardwareAddr, ip, ethernet.Broadcast, ip)
		if err != nil {
			return fmt.Errorf("assembling %q gratuitous packet for %q: %s", op, ip, err)
		}
		if err = a.conn.WriteTo(pkt, ethernet.Broadcast); err != nil {
			return fmt.Errorf("writing %q gratuitous packet for %q: %s", op, ip, err)
		}
		stats.SentGratuitous(ip.String())
	}
	return nil
}

func (a *arpResponder) run() {
	for a.processRequest() != dropReasonClosed {
	}
}

func (a *arpResponder) processRequest() dropReason {
	pkt, eth, err := a.conn.Read()
	if err != nil {
		// ARP listener doesn't cleanly return EOF when closed, so we
		// need to hook into the call to arpResponder.Close()
		// independently.
		select {
		case <-a.closed:
			return dropReasonClosed
		default:
		}
		if err == io.EOF {
			glog.Infof("DEBUG: %s %s responder closed", a.intf, a.hardwareAddr)
			return dropReasonClosed
		}
		glog.Infof("DEBUG: %s %s responder errored: %s", a.intf, a.hardwareAddr, err)
		return dropReasonError
	}

	// Ignore ARP replies.
	if pkt.Operation != arp.OperationRequest {
		glog.Infof("DEBUG: %s %s responder got a non-request ARP packet", a.intf, a.hardwareAddr, err)
		return dropReasonARPReply
	}

	// Ignore ARP requests which are not broadcast or bound directly for this machine.
	if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, a.hardwareAddr) {
		glog.Infof("DEBUG: %s %s responder ignored packet, %q is not bcast or local ethernet addr", a.intf, a.hardwareAddr, eth.Destination)
		return dropReasonEthernetDestination
	}

	glog.Infof("Request: %s %s who-has %s?  tell %s (%s).", a.intf, a.hardwareAddr, pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr)

	// Ignore ARP requests that the announcer tells us to ignore.
	if reason := a.announce(pkt.TargetIP); reason != dropReasonNone {
		glog.Infof("DEBUG: %s %s responder ignored packet (%s %s %s), not programmed to advertise this IP (reason %d)", a.intf, a.hardwareAddr, pkt.TargetIP, pkt.SenderIP, eth.Destination, reason)
		return reason
	}

	stats.GotRequest(pkt.TargetIP.String())

	glog.Infof("Request: %s %s who-has %s?  tell %s (%s). reply: %s is-at %s", a.intf, a.hardwareAddr, pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)
	if err := a.conn.Reply(pkt, a.hardwareAddr, pkt.TargetIP); err != nil {
		glog.Warningf("Failed to write ARP response for %q: %s", pkt.TargetIP, err)
	} else {
		stats.SentResponse(pkt.TargetIP.String())
	}
	return dropReasonNone
}
