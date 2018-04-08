package layer2

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	sync.RWMutex
	arps   map[int]*arpResponder
	ndps   map[int]*ndpResponder
	ips    map[string]net.IP // map containing IPs we should announce
	leader bool
}

// New returns an initialized Announce.
func New(ifi *net.Interface) (*Announce, error) {
	glog.Infof("creating layer 2 announcer on interface %q", ifi.Name)

	ret := &Announce{
		arps: map[int]*arpResponder{},
		ndps: map[int]*ndpResponder{},
		ips:  make(map[string]net.IP),
	}
	go ret.interfaceScan()

	return ret, nil
}

func (a *Announce) interfaceScan() {
	for {
		if err := a.updateInterfaces(); err != nil {
			glog.Errorf("Updating interfaces: %s", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func (a *Announce) updateInterfaces() error {
	ifs, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("Couldn't list interfaces: %s", err)
	}

	a.Lock()
	defer a.Unlock()

	keepARP, keepNDP := map[int]bool{}, map[int]bool{}
	for _, intf := range ifs {
		ifi := intf
		addrs, err := ifi.Addrs()
		if err != nil {
			return fmt.Errorf("couldn't get addresses for %q: %s", ifi.Name, err)
		}

		for _, a := range addrs {
			ipaddr, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if !ipaddr.IP.IsGlobalUnicast() {
				continue
			}
			if ipaddr.IP.To4() == nil {
				keepNDP[ifi.Index] = true
			} else {
				keepARP[ifi.Index] = true
			}
		}

		if keepARP[ifi.Index] && a.arps[ifi.Index] == nil {
			resp, err := newARPResponder(&ifi, a.shouldAnnounce)
			if err != nil {
				return fmt.Errorf("creating ARP responder for %q: %s", ifi.Name, err)
			}
			a.arps[ifi.Index] = resp
			glog.Infof("created ARP listener on interface %d (%q)", ifi.Index, ifi.Name)
		}
		if keepNDP[ifi.Index] && a.ndps[ifi.Index] == nil {
			resp, err := newNDPResponder(&ifi, a.shouldAnnounce)
			if err != nil {
				return fmt.Errorf("creating NDP responder for %q: %s", ifi.Name, err)
			}
			a.ndps[ifi.Index] = resp
			glog.Infof("created NDP listener on interface %d (%q)", ifi.Index, ifi.Name)
		}
	}

	for i, client := range a.arps {
		if !keepARP[i] {
			client.Close()
			delete(a.arps, i)
			glog.Infof("Deleted ARP listener on interface %d", i)
		}
	}
	for i, client := range a.ndps {
		if !keepNDP[i] {
			client.Close()
			delete(a.ndps, i)
			glog.Infof("Deleted NDP listener on interface %d", i)
		}
	}

	return nil
}

func (a *Announce) gratuitous(ip net.IP) error {
	a.Lock()
	defer a.Unlock()

	if ip.To4() == nil {
		for _, client := range a.arps {
			if err := client.Gratuitous(ip); err != nil {
				return err
			}
		}
	} else {
		for _, client := range a.ndps {
			if err := client.Gratuitous(ip); err != nil {
				return err
			}
		}
	}
	return nil
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

	for _, client := range a.ndps {
		if err := client.Watch(ip); err != nil {
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

	for _, client := range a.ndps {
		if err := client.Unwatch(ip); err != nil {
			glog.Errorf("configuring announcement for %q: %s", ip, err)
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
