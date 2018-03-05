package arp

import (
	"bytes"
	"fmt"
	"io"
	"net"

	"github.com/golang/glog"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"go.universe.tf/metallb/internal/iface"
)

type announceFunc func(net.IP) iface.DropReason

type arpResponder struct {
	hardwareAddr net.HardwareAddr
	conn         *arp.Client
	announce     announceFunc
}

func newARP(ifi *net.Interface, ann announceFunc) (*arpResponder, error) {
	client, err := arp.Dial(ifi)
	if err != nil {
		return nil, fmt.Errorf("creating responder for %q: %s", ifi.Name, err)
	}

	ret := &arpResponder{
		hardwareAddr: ifi.HardwareAddr,
		conn:         client,
		announce:     ann,
	}
	go ret.run()
	return ret, nil
}

func (a *arpResponder) Close() error {
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
	for a.processRequest() != iface.DropReasonClosed {
	}
}

func (a *arpResponder) processRequest() iface.DropReason {
	pkt, eth, err := a.conn.Read()
	if err != nil {
		if err == io.EOF {
			return iface.DropReasonClosed
		}
		return iface.DropReasonError
	}

	// Ignore ARP replies.
	if pkt.Operation != arp.OperationRequest {
		return iface.DropReasonARPReply
	}

	// Ignore ARP requests which are not broadcast or bound directly for this machine.
	if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, a.hardwareAddr) {
		return iface.DropReasonEthernetDestination
	}

	// Ignore ARP requests that the announcer tells us to ignore.
	if reason := a.announce(pkt.TargetIP); reason != iface.DropReasonNone {
		return reason
	}

	stats.GotRequest(pkt.TargetIP.String())

	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)
	if err := a.conn.Reply(pkt, a.hardwareAddr, pkt.TargetIP); err != nil {
		glog.Warningf("Failed to write ARP response for %q: %s", pkt.TargetIP, err)
		return iface.DropReasonError
	}

	stats.SentResponse(pkt.TargetIP.String())
	return iface.DropReasonNone
}
