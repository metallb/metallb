package layer2

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

type announceFunc func(net.IP) dropReason

type arpCoordinator struct {
	stop     chan struct{}
	announce announceFunc

	mu         sync.Mutex
	responders map[int]*arpResponder
}

func newARP(ann announceFunc) *arpCoordinator {
	ret := &arpCoordinator{
		stop:       make(chan struct{}),
		announce:   ann,
		responders: map[int]*arpResponder{},
	}

	go ret.periodicScan()
	return ret
}

func (a *arpCoordinator) Gratuitous(ip net.IP) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, client := range a.responders {
		if err := client.Gratuitous(ip); err != nil {
			return err
		}
	}

	return nil
}

func (a *arpCoordinator) periodicScan() {
	for {
		select {
		case <-a.stop:
			return
		default:
		}
		if err := a.updateInterfaces(); err != nil {
			glog.Errorf("Updating interfaces: %s", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func (a *arpCoordinator) updateInterfaces() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ifs, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("Couldn't list interfaces: %s", err)
	}
	keep := map[int]bool{}
	for _, ifi := range ifs {
		if !useInterfaceForARP(ifi) {
			continue
		}
		if a.responders[ifi.Index] == nil {
			resp, err := newARPResponder(&ifi, a.announce)
			if err != nil {
				return fmt.Errorf("Creating ARP responder for %q: %s", ifi.Name, err)
			}
			a.responders[ifi.Index] = resp
			glog.Infof("Created ARP listener on interface %d (%q)", ifi.Index, ifi.Name)
		}
		keep[ifi.Index] = true
	}

	for i, client := range a.responders {
		if !keep[i] {
			client.Close()
			delete(a.responders, i)
			glog.Infof("Deleted ARP listener on interface %d", i)
		}
	}

	return nil
}

func useInterfaceForARP(ifi net.Interface) bool {
	addrs, err := ifi.Addrs()
	if err != nil {
		glog.Errorf("Couldn't get addresses for %s: %s", ifi.Name, err)
		return false
	}

	for _, a := range addrs {
		ipaddr, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ipaddr.IP.To4() == nil {
			continue
		}
		if !ipaddr.IP.IsGlobalUnicast() {
			continue
		}
		return true
	}

	return false
}

type arpResponder struct {
	hardwareAddr net.HardwareAddr
	conn         *arp.Client
	announce     announceFunc
}

func newARPResponder(ifi *net.Interface, ann announceFunc) (*arpResponder, error) {
	client, err := arp.Dial(ifi)
	if err != nil {
		return nil, fmt.Errorf("creating ARP responder for %q: %s", ifi.Name, err)
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
	for a.processRequest() != dropReasonClosed {
	}
}

func (a *arpResponder) processRequest() dropReason {
	pkt, eth, err := a.conn.Read()
	if err != nil {
		if err == io.EOF {
			return dropReasonClosed
		}
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
	if reason := a.announce(pkt.TargetIP); reason != dropReasonNone {
		return reason
	}

	stats.GotRequest(pkt.TargetIP.String())

	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)
	if err := a.conn.Reply(pkt, a.hardwareAddr, pkt.TargetIP); err != nil {
		glog.Warningf("Failed to write ARP response for %q: %s", pkt.TargetIP, err)
	} else {
		stats.SentResponse(pkt.TargetIP.String())
	}
	return dropReasonNone
}
