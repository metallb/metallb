package arp

import (
	"bytes"
	"errors"
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
		return nil, err
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

// Run starts the announcer, making it listen on the interface for ARP requests. It only responds to these
// requests when a.leader is set to true, i.e. we are the current cluster wide leader for sending ARPs.
func (a *Announce) Run() {
	for {
		pkt, eth, err := a.client.Read()

		if err != nil {
			continue
		}

		// Ignore ARP replies.
		if pkt.Operation != arp.OperationRequest {
			continue
		}

		// Ignore ARP requests which are not broadcast or bound directly for this machine.
		if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, a.hardwareAddr) {
			continue
		}

		// Ignore ARP requests which do not indicate the target IP that we should announce.
		if !a.Announce(pkt.TargetIP) {
			continue
		}

		// We are not the leader, do not reply.
		if !a.Leader() {
			continue
		}

		// pkt.TargetIP has been vetted to be "the one".
		glog.Infof("request: who-has %s?  tell %s (%s). reply: %s is-at %s", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr, pkt.TargetIP, a.hardwareAddr)

		if err := a.Reply(pkt, pkt.TargetIP); err != nil {
			glog.Warningf("Failed to writes ARP response for %s: %s", pkt.TargetIP, err)
		}
	}
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

// Unsolicited returns a slice of ARP responses that can be send out as unsolicited ARPs.
func (a *Announce) Unsolicited() []*arp.Packet {
	return nil
}

// interfaceByIP returns the interface that has ip.
func interfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
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
