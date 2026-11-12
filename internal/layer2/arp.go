// SPDX-License-Identifier:Apache-2.0

package layer2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
)

type announceFunc func(netip.Addr, string) dropReason

type arpClient struct {
	logger       log.Logger
	intf         string
	hardwareAddr net.HardwareAddr
	conn         net.PacketConn
	closed       chan struct{}
	announce     announceFunc
}

func newARPClient(logger log.Logger, ifi *net.Interface, ann announceFunc) (*arpClient, error) {
	// Unlike arp.Dial, a raw packet listener does not require the interface
	// to have its own IPv4 address.
	conn, err := packet.Listen(ifi, packet.Raw, int(ethernet.EtherTypeARP), nil)
	if err != nil {
		return nil, fmt.Errorf("creating ARP responder for %q: %s", ifi.Name, err)
	}

	ret := &arpClient{
		logger:       logger,
		intf:         ifi.Name,
		hardwareAddr: ifi.HardwareAddr,
		conn:         conn,
		closed:       make(chan struct{}),
		announce:     ann,
	}
	go ret.run()
	return ret, nil
}

func (a *arpClient) Interface() string { return a.intf }

func (a *arpClient) Close() error {
	close(a.closed)
	return a.conn.Close()
}

func (a *arpClient) Gratuitous(ip netip.Addr) error {
	for _, op := range []arp.Operation{arp.OperationRequest, arp.OperationReply} {
		pkt, err := arp.NewPacket(op, a.hardwareAddr, ip, ethernet.Broadcast, ip)
		if err != nil {
			return fmt.Errorf("assembling %q gratuitous packet for %q: %s", op, ip, err)
		}
		if err = a.writePacket(pkt, ethernet.Broadcast); err != nil {
			return fmt.Errorf("writing %q gratuitous packet for %q: %s", op, ip, err)
		}
		stats.SentGratuitous(ip.String())
	}
	return nil
}

func (a *arpClient) run() {
	for a.processRequest() != dropReasonClosed {
	}
}

func (a *arpClient) processRequest() dropReason {
	// An Ethernet/IPv4 ARP packet fits in 128 bytes, including padding.
	var buf [128]byte
	n, _, err := a.conn.ReadFrom(buf[:])
	if err != nil {
		// ARP listener doesn't cleanly return EOF when closed, so we
		// need to hook into the call to arpClient.Close()
		// independently.
		select {
		case <-a.closed:
			return dropReasonClosed
		default:
		}
		if errors.Is(err, io.EOF) {
			return dropReasonClosed
		}
		return dropReasonError
	}

	var eth ethernet.Frame
	if err := eth.UnmarshalBinary(buf[:n]); err != nil || eth.EtherType != ethernet.EtherTypeARP {
		return dropReasonError
	}
	var pkt arp.Packet
	if err := pkt.UnmarshalBinary(eth.Payload); err != nil {
		return dropReasonError
	}

	// Ignore ARP replies.
	if pkt.Operation != arp.OperationRequest {
		return dropReasonARPReply
	}

	// Ignore ARP requests which are not broadcast or bound directly for this machine.
	if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, a.hardwareAddr) {
		return dropReasonEthernetDestination
	}

	// Ignore ARP requests that the announcer tells us to ignore.
	reason := a.announce(pkt.TargetIP, a.intf)
	if reason == dropReasonNotMatchInterface {
		level.Debug(a.logger).Log("op", "arpRequestIgnore", "ip", pkt.TargetIP, "interface", a.intf, "reason", "notMatchInterface")
	}
	if reason != dropReasonNone {
		return reason
	}

	stats.GotRequest(pkt.TargetIP.String())
	level.Debug(a.logger).Log("interface", a.intf, "ip", pkt.TargetIP, "senderIP", pkt.SenderIP, "senderMAC", pkt.SenderHardwareAddr, "responseMAC", a.hardwareAddr, "msg", "got ARP request for service IP, sending response")

	reply, err := arp.NewPacket(arp.OperationReply, a.hardwareAddr, pkt.TargetIP, pkt.SenderHardwareAddr, pkt.SenderIP)
	if err == nil {
		err = a.writePacket(reply, pkt.SenderHardwareAddr)
	}
	if err != nil {
		level.Error(a.logger).Log("op", "arpReply", "interface", a.intf, "ip", pkt.TargetIP, "senderIP", pkt.SenderIP, "senderMAC", pkt.SenderHardwareAddr, "responseMAC", a.hardwareAddr, "error", err, "msg", "failed to send ARP reply")
	} else {
		stats.SentResponse(pkt.TargetIP.String())
	}
	return dropReasonNone
}

func (a *arpClient) writePacket(pkt *arp.Packet, dst net.HardwareAddr) error {
	payload, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}
	frame := ethernet.Frame{
		Destination: dst,
		Source:      a.hardwareAddr,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     payload,
	}
	buf, err := frame.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = a.conn.WriteTo(buf, &packet.Addr{HardwareAddr: dst})
	return err
}
