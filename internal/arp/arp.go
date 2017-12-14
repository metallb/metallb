package arp

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	hardwareAddr net.HardwareAddr
	client       *arp.Client
	ips          map[string]net.IP // map containing IPs we should announce
	sync.RWMutex                   // protects ips

	leader   bool
	leaderMu sync.RWMutex
}

// New return an initialized Announce.
func New(ip net.IP) (*Announce, error) {
	ifi, err := interfaceByIP(ip)
	if err != nil {
		return nil, fmt.Errorf("arp: can't find interface for %s: %s", ip, err)
	}
	client, err := arp.Dial(ifi)
	if err != nil {
		return nil, err
	}

	return &Announce{
		hardwareAddr: ifi.HardwareAddr,
		client:       client,
		ips:          make(map[string]net.IP),
	}, nil
}

// A dropReason is a reason an Announce drops an incoming ARP packet.
type dropReason int

// Possible dropReason values.
const (
	dropReasonNone dropReason = iota
	dropReasonReadError
	dropReasonARPReply
	dropReasonEthernetDestination
	dropReasonAnnounceIP
	dropReasonNotLeader
)

// Run starts the announcer, making it listen on the interface for ARP requests. It only responds to these
// requests when a.leader is set to true, i.e. we are the current cluster wide leader for sending ARPs.
func (a *Announce) Run() {
	for {
		a.readPacket()
	}
}

// readPacket reads and handles a single ARP packet, and reports any reason why
// the packet was dropped.
func (a *Announce) readPacket() dropReason {
	pkt, eth, err := a.client.Read()

	if err != nil {
		return dropReasonReadError
	}

	// Ignore ARP replies.
	if pkt.Operation != arp.OperationRequest {
		return dropReasonARPReply
	}

	// Ignore ARP requests which are not broadcast or bound directly for this machine.
	if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, a.hardwareAddr) {
		return dropReasonEthernetDestination
	}

	// Ignore ARP requests which do not indicate the target IP that we should announce.
	if !a.Announce(pkt.TargetIP) {
		return dropReasonAnnounceIP
	}

	// We are not the leader, do not reply.
	if !a.Leader() {
		return dropReasonNotLeader
	}

	// pkt.TargetIP has been vetted to be "the one".
	glog.Infof("request: who-has %s?  tell %s (%s). reply: %s is-at %s", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)

	if err := a.Reply(pkt, pkt.TargetIP); err != nil {
		glog.Warningf("Failed to writes ARP response for %s: %s", pkt.TargetIP, err)
	}

	return dropReasonNone
}

// Reply sends a arp reply using the client in a.
func (a *Announce) Reply(pkt *arp.Packet, ip net.IP) error {
	return a.client.Reply(pkt, a.hardwareAddr, ip)
}

// Close closes the arp client in a.
func (a *Announce) Close() error {
	return a.client.Close()
}

// SetBalancer implementes adds ip to the set of announced address.
func (a *Announce) SetBalancer(name string, ip net.IP) {
	a.Lock()
	defer a.Unlock()
	a.ips[name] = ip
}

// DeleteBalancer an address from the set of address we should announce.
func (a *Announce) DeleteBalancer(name string) {
	a.Lock()
	defer a.Unlock()
	if _, ok := a.ips[name]; ok {
		delete(a.ips, name)
	}
}

// Announce checks if ip should be announced.
func (a *Announce) Announce(ip net.IP) bool {
	a.RLock()
	defer a.RUnlock()
	for _, i := range a.ips {
		if i.Equal(ip) {
			return true
		}
	}
	return false
}

// AnnounceName returns true when we have an announcement under name.
func (a *Announce) AnnounceName(name string) bool {
	a.RLock()
	defer a.RUnlock()
	_, ok := a.ips[name]
	return ok
}

// Unsolicited returns a slice of ARP responses that can be send out as unsolicited ARPs.
func (a *Announce) Unsolicited() []*arp.Packet {
	a.RLock()
	defer a.RUnlock()

	arps := []*arp.Packet{}
	for _, ip := range a.ips {
		if a, err := arp.NewPacket(arp.OperationReply, a.hardwareAddr, ip, ethernet.Broadcast, ip); err == nil {
			arps = append(arps, a)
		}
	}
	return arps
}

// interfaceByIP returns the interface that has ip.
func interfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("No interfaces found: %s", err)
	}
	for _, i := range ifaces {
		glog.Infof("Found interface: %v", i)
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			glog.Infof("Address found %s for interface: %v", addr.String(), i)
			switch v := addr.(type) {
			case *net.IPNet:
				if ip.Equal(v.IP) {
					return &i, nil
				}
			case *net.IPAddr:
				if ip.Equal(v.IP) {
					return &i, nil
				}
			}

		}
	}

	return nil, errors.New("not found")
}
