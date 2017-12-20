package ndp

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"
	"github.com/mdlayher/ndp"
)

// Announce is used to "announce" new IPs mapped to the node's MAC address.
type Announce struct {
	hardwareAddr net.HardwareAddr
	conn         *ndp.Conn
	ips          map[string]net.IP // map containing IPs we should announce
	sync.RWMutex                   // protects ips
	stop         chan bool         // stop unsolicited ndp spamming

	leader   bool
	leaderMu sync.RWMutex
}

// New returns an initialized Announce.
func New(ip net.IP) (*Announce, error) {
	ifi, err := interfaceByIP(ip)
	if err != nil {
		return nil, fmt.Errorf("ndp: can't find interface for %s: %s", ip, err)
	}
	conn, _, err := ndp.Dial(ifi)
	if err != nil {
		return nil, err
	}

	return &Announce{
		hardwareAddr: ifi.HardwareAddr,
		conn:         conn,
		ips:          make(map[string]net.IP),
		stop:         make(chan bool),
	}, nil
}

// A dropReason is a reason an Announce drops an incoming NDP packet.
type dropReason int

// Possible dropReason values.
const (
	dropReasonNone dropReason = iota
	dropReasonError
	dropReasonMessageType
	dropReasonNoSourceLL
	dropReasonAnnounceIP
	dropReasonNotLeader
)

// Run starts the announcer, making it listen on the interface for NDP requests.
// It only responds to these requests when a.leader is set to true, i.e. we are
// the current cluster wide leader for sending NDP messages.
func (a *Announce) Run() {
	for {
		a.readMessage()
	}
}

// readMessage reads and handles a single NDP message, and reports any reason why
// the packet was dropped.
func (a *Announce) readMessage() dropReason {
	msg, _, src, err := a.conn.ReadFrom()
	if err != nil {
		return dropReasonError
	}

	// Ignore all messages other than neighbor solicitations.
	ns, ok := msg.(*ndp.NeighborSolicitation)
	if !ok {
		return dropReasonMessageType
	}

	// Retrieve the sender's source link-layer address.
	var nsLLAddr net.HardwareAddr
	for _, o := range ns.Options {
		// Ignore other options, including target link-layer address instead of source.
		lla, ok := o.(*ndp.LinkLayerAddress)
		if !ok {
			continue
		}
		if lla.Direction != ndp.Source {
			continue
		}

		nsLLAddr = lla.Addr
		break
	}
	if nsLLAddr == nil {
		return dropReasonNoSourceLL
	}

	// Ignore ndp requests which do not indicate the target IP that we should announce.
	if !a.Announce(ns.TargetAddress) {
		return dropReasonAnnounceIP
	}

	// We are not the leader, do not reply.
	if !a.Leader() {
		return dropReasonNotLeader
	}

	// pkt.TargetIP has been vetted to be "the one".
	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", ns.TargetAddress, src, nsLLAddr, ns.TargetAddress, a.hardwareAddr)

	// This request was solicited.
	solicited := true
	if err := a.advertise(src, ns.TargetAddress, solicited); err != nil {
		glog.Warningf("Failed to write NDP neighbor advertisement for %s: %s", ns.TargetAddress, err)
	}

	return dropReasonNone
}

// advertise sends a NDP neighbor advertisement to dst for IP target using the
// client in a.
func (a *Announce) advertise(dst, target net.IP, solicited bool) error {
	m := &ndp.NeighborAdvertisement{
		Solicited:     solicited,
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      a.hardwareAddr,
			},
		},
	}

	return a.conn.WriteTo(m, nil, dst)
}

// Close closes the ndp client in a.
func (a *Announce) Close() error {
	return a.conn.Close()
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

// Advertise sends unsolicited NDP neighbor advertisements for all IPs.
func (a *Announce) Advertise() {
	a.RLock()
	defer a.RUnlock()

	solicited := false
	for _, ip := range a.ips {
		_ = a.advertise(net.IPv6linklocalallnodes, ip, solicited)
	}
}

// interfaceByIP returns the interface that has ip.
func interfaceByIP(ip net.IP) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("no interfaces found: %s", err)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if ip.Equal(v.IP) {
					glog.Infof("Address found %s for interface: %v", addr.String(), i)
					return &i, nil
				}
			case *net.IPAddr:
				if ip.Equal(v.IP) {
					glog.Infof("Address found %s for interface: %v", addr.String(), i)
					return &i, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("address not found in %d interfaces", len(ifaces))
}
