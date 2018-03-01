package ndp

import (
	"fmt"
	"net"
	"sync"

	"go.universe.tf/metallb/internal/iface"

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
func New(ifi *net.Interface) (*Announce, error) {
	glog.Infof("creating NDP announcer on interface %q", ifi.Name)

	// Use link-local address as the source IPv6 address for NDP communications.
	conn, _, err := ndp.Dial(ifi, ndp.LinkLocal)
	if err != nil {
		return nil, err
	}

	ret := &Announce{
		hardwareAddr: ifi.HardwareAddr,
		conn:         conn,
		ips:          make(map[string]net.IP),
		stop:         make(chan bool),
	}
	go ret.Run()
	return ret, nil
}

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
func (a *Announce) readMessage() iface.DropReason {
	msg, _, src, err := a.conn.ReadFrom()
	if err != nil {
		return iface.DropReasonError
	}

	// Ignore all messages other than neighbor solicitations.
	ns, ok := msg.(*ndp.NeighborSolicitation)
	if !ok {
		return iface.DropReasonMessageType
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
		return iface.DropReasonNoSourceLL
	}

	// Ignore ndp requests which do not indicate the target IP that we should announce.
	if !a.Announce(ns.TargetAddress) {
		return iface.DropReasonAnnounceIP
	}

	stats.GotRequest(ns.TargetAddress.String())

	// We are not the leader, do not reply.
	if !a.Leader() {
		return iface.DropReasonNotLeader
	}

	// pkt.TargetIP has been vetted to be "the one".
	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", ns.TargetAddress, src, nsLLAddr, ns.TargetAddress, a.hardwareAddr)

	// This request was solicited, but should not override previous entries.
	var (
		solicited = true
		override  = false
	)

	if err := a.advertise(src, ns.TargetAddress, solicited, override); err != nil {
		glog.Warningf("Failed to write NDP neighbor advertisement for %s: %s", ns.TargetAddress, err)
	}

	stats.SentResponse(ns.TargetAddress.String())

	return iface.DropReasonNone
}

// advertise sends a NDP neighbor advertisement to dst for IP target using the
// client in a.
func (a *Announce) advertise(dst, target net.IP, solicited, override bool) error {
	m := &ndp.NeighborAdvertisement{
		Solicited:     solicited,
		Override:      override,
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

	// Kubernetes may inform us that we should advertise this address multiple
	// times, so just no-op any subsequent requests.
	if _, ok := a.ips[name]; ok {
		return
	}

	// To receive neighbor solicitations for this address, we have to join its
	// solicited-node multicast group.
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		panic(fmt.Sprintf("ndp: failed to create solicited node multicast group for %s: %v", ip, err))
	}

	if err := a.conn.JoinGroup(group); err != nil {
		panic(fmt.Sprintf("ndp: failed to join solicited node multicast group for %s: %v", ip, err))
	}

	a.ips[name] = ip
}

// DeleteBalancer deletes an address from the set of addresses we should announce.
func (a *Announce) DeleteBalancer(name string) {
	a.Lock()
	defer a.Unlock()

	ip, ok := a.ips[name]
	if !ok {
		// IP already removed from our set, no-op.
		return
	}

	// No longer announcing this address; leave its solicited-node multicast
	// group and clean it up.
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		panic(fmt.Sprintf("ndp: failed to create solicited node multicast group for %s: %v", ip, err))
	}

	if err := a.conn.LeaveGroup(group); err != nil {
		panic(fmt.Sprintf("ndp: failed to join solicited node multicast group for %s: %v", ip, err))
	}

	delete(a.ips, name)
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

	// We are sending unsolicited advertisements, and clients should update
	// their neighbor cache.
	var (
		solicited = false
		override  = true
	)

	for _, ip := range a.ips {
		_ = a.advertise(net.IPv6linklocalallnodes, ip, solicited, override)
		stats.SentGratuitous(ip.String())
	}
}
