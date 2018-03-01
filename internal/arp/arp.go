package arp

import (
	"bytes"
	"net"
	"sync"

	"go.universe.tf/metallb/internal/iface"

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
	stop         chan bool         // stop unsolicited arp spamming

	leader   bool
	leaderMu sync.RWMutex
}

// New returns an initialized Announce.
func New(ifi *net.Interface) (*Announce, error) {
	glog.Infof("creating ARP announcer on interface %q", ifi.Name)

	client, err := arp.Dial(ifi)
	if err != nil {
		return nil, err
	}

	ret := &Announce{
		hardwareAddr: ifi.HardwareAddr,
		client:       client,
		ips:          make(map[string]net.IP),
		stop:         make(chan bool),
	}
	go ret.run()
	return ret, nil
}

// run starts the announcer, making it listen on the interface for ARP requests. It only responds to these
// requests when a.leader is set to true, i.e. we are the current cluster wide leader for sending ARPs.
func (a *Announce) run() {
	for {
		a.readPacket()
	}
}

// readPacket reads and handles a single ARP packet, and reports any reason why
// the packet was dropped.
func (a *Announce) readPacket() iface.DropReason {
	pkt, eth, err := a.client.Read()

	if err != nil {
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

	// Ignore ARP requests which do not indicate the target IP that we should announce.
	if !a.Announce(pkt.TargetIP) {
		return iface.DropReasonAnnounceIP
	}

	stats.GotRequest(pkt.TargetIP.String())

	// We are not the leader, do not reply.
	if !a.Leader() {
		return iface.DropReasonNotLeader
	}

	// pkt.TargetIP has been vetted to be "the one".
	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)

	if err := a.reply(pkt, pkt.TargetIP); err != nil {
		glog.Warningf("Failed to write ARP response for %s: %s", pkt.TargetIP, err)
	}

	stats.SentResponse(pkt.TargetIP.String())

	return iface.DropReasonNone
}

// reply sends a arp reply using the client in a.
func (a *Announce) reply(pkt *arp.Packet, ip net.IP) error {
	return a.client.Reply(pkt, a.hardwareAddr, ip)
}

// Close closes the arp client in a.
func (a *Announce) Close() error {
	return a.client.Close()
}

// SetBalancer adds ip to the set of announced addresses.
func (a *Announce) SetBalancer(name string, ip net.IP) {
	a.Lock()
	defer a.Unlock()
	a.ips[name] = ip
}

// DeleteBalancer deletes an address from the set of addresses we should announce.
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

// Packets returns a slice of ARP packets that can be send out as unsolicited ARPs.
func (a *Announce) Packets() []*arp.Packet {
	a.RLock()
	defer a.RUnlock()

	arps := []*arp.Packet{}
	for _, ip := range a.ips {
		if a, err := arp.NewPacket(arp.OperationReply, a.hardwareAddr, ip, ethernet.Broadcast, ip); err == nil {
			arps = append(arps, a)
		}
		if a, err := arp.NewPacket(arp.OperationRequest, a.hardwareAddr, ip, ethernet.Broadcast, ip); err == nil {
			arps = append(arps, a)
		}
	}
	return arps
}
