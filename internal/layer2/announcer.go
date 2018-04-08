package layer2

import (
	"net"
	"sync"

	"github.com/golang/glog"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	arp *arpCoordinator
	ndp *ndpCoordinator

	sync.RWMutex
	ips    map[string]net.IP // map containing IPs we should announce
	leader bool
}

// New returns an initialized Announce.
func New(ifi *net.Interface) (*Announce, error) {
	glog.Infof("creating layer 2 announcer on interface %q", ifi.Name)

	ret := &Announce{
		ips: make(map[string]net.IP),
	}

	ret.arp = newARP(ret.shouldAnnounce)
	ret.ndp = newNDP(ret.shouldAnnounce)

	return ret, nil
}

func (a *Announce) shouldAnnounce(ip net.IP) dropReason {
	a.RLock()
	defer a.RUnlock()
	if !a.leader {
		return dropReasonNotLeader
	}
	for _, i := range a.ips {
		if i.Equal(ip) {
			return dropReasonNone
		}
	}
	return dropReasonAnnounceIP
}

// SetBalancer adds ip to the set of announced addresses.
func (a *Announce) SetBalancer(name string, ip net.IP) {
	a.Lock()
	defer a.Unlock()

	// Kubernetes may inform us that we should advertise this address multiple
	// times, so just no-op any subsequent requests.
	if _, ok := a.ips[name]; ok {
		return
	}

	if a.ndp != nil {
		if err := a.ndp.Watch(ip); err != nil {
			glog.Errorf("configuring announcement for %q: %s", ip, err)
		}
	}

	a.ips[name] = ip
}

// DeleteBalancer deletes an address from the set of addresses we should announce.
func (a *Announce) DeleteBalancer(name string) {
	a.Lock()
	defer a.Unlock()

	ip, ok := a.ips[name]
	if !ok {
		return
	}

	if err := a.ndp.Unwatch(ip); err != nil {
		glog.Errorf("removing announcement for %q: %s", ip, err)
	}

	delete(a.ips, name)
}

// AnnounceName returns true when we have an announcement under name.
func (a *Announce) AnnounceName(name string) bool {
	a.RLock()
	defer a.RUnlock()
	_, ok := a.ips[name]
	return ok
}

// dropReason is the reason why a layer2 protocol packet was not
// responded to.
type dropReason int

// Various reasons why a packet was dropped.
const (
	dropReasonNone dropReason = iota
	dropReasonClosed
	dropReasonError
	dropReasonARPReply
	dropReasonMessageType
	dropReasonNoSourceLL
	dropReasonEthernetDestination
	dropReasonAnnounceIP
	dropReasonNotLeader
)
