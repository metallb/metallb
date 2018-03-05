package arp

import (
	"net"
	"sync"

	"go.universe.tf/metallb/internal/iface"

	"github.com/golang/glog"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	responder *arpResponder

	sync.RWMutex
	ips    map[string]net.IP // map containing IPs we should announce
	leader bool
}

// New returns an initialized Announce.
func New(ifi *net.Interface) (*Announce, error) {
	glog.Infof("creating ARP announcer on interface %q", ifi.Name)

	ret := &Announce{
		ips: make(map[string]net.IP),
	}

	resp, err := newARP(ifi, ret.shouldAnnounce)
	if err != nil {
		return nil, err
	}
	ret.responder = resp

	return ret, nil
}

func (a *Announce) shouldAnnounce(ip net.IP) iface.DropReason {
	a.RLock()
	defer a.RUnlock()
	if !a.leader {
		return iface.DropReasonNotLeader
	}
	for _, i := range a.ips {
		if i.Equal(ip) {
			return iface.DropReasonNone
		}
	}
	return iface.DropReasonAnnounceIP
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

// AnnounceName returns true when we have an announcement under name.
func (a *Announce) AnnounceName(name string) bool {
	a.RLock()
	defer a.RUnlock()
	_, ok := a.ips[name]
	return ok
}
