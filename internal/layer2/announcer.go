package layer2

import (
	"net"
	"sync"

	"go.universe.tf/metallb/internal/iface"

	"github.com/golang/glog"
	"github.com/mdlayher/ndp"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	arpResponder *arpResponder
	ndpResponder *ndpResponder

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

	arpResp, err := newARP(ifi, ret.shouldAnnounce)
	if err != nil {
		return nil, err
	}
	ret.arpResponder = arpResp

	ndpResp, err := newNDP(ifi, ret.shouldAnnounce)
	if err != nil {
		// TODO(bug 180): make this fail in a more principled way.
		ndpResp = nil
	}
	ret.ndpResponder = ndpResp

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

	// Kubernetes may inform us that we should advertise this address multiple
	// times, so just no-op any subsequent requests.
	if _, ok := a.ips[name]; ok {
		return
	}

	if ip.To4() == nil {
		// For IPv6, we have to join the sollicited node multicast
		// group for the target IP, so that we receive requests for
		// that IP's MAC address.
		group, err := ndp.SolicitedNodeMulticast(ip)
		if err != nil {
			glog.Errorf("Failed to look up solicited node multicast group for %q: %s", ip, err)
		} else if err = a.ndpResponder.conn.JoinGroup(group); err != nil {
			glog.Errorf("Failed to join solicited node multicast group for %q: %s", ip, err)
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

	if ip.To4() == nil {
		// Leave the solicited multicast group for this IP.
		// TODO(bug 184): solicited node multicast group memberships need to be refcounted.
		group, err := ndp.SolicitedNodeMulticast(ip)
		if err != nil {
			glog.Errorf("Failed to look up solicited node multicast group for %q: %s", ip, err)
		} else if err = a.ndpResponder.conn.LeaveGroup(group); err != nil {
			glog.Errorf("Failed to leave solicited node multicast group for %q: %s", ip, err)
		}
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
