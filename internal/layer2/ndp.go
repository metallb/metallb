package layer2

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/mdlayher/ndp"
)

type ndpCoordinator struct {
	stop     chan struct{}
	announce announceFunc

	mu         sync.Mutex
	responders map[int]*ndpResponder
}

func newNDP(ann announceFunc) *ndpCoordinator {
	ret := &ndpCoordinator{
		stop:       make(chan struct{}),
		announce:   ann,
		responders: map[int]*ndpResponder{},
	}

	go ret.periodicScan()
	return ret
}

func (a *ndpCoordinator) Gratuitous(ip net.IP) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, client := range a.responders {
		if err := client.Gratuitous(ip); err != nil {
			return err
		}
	}

	return nil
}

func (a *ndpCoordinator) Watch(ip net.IP) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, client := range a.responders {
		if err := client.Watch(ip); err != nil {
			return err
		}
	}

	return nil
}

func (a *ndpCoordinator) Unwatch(ip net.IP) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, client := range a.responders {
		if err := client.Unwatch(ip); err != nil {
			return err
		}
	}

	return nil
}

func (a *ndpCoordinator) periodicScan() {
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

func (a *ndpCoordinator) updateInterfaces() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ifs, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("Couldn't list interfaces: %s", err)
	}
	keep := map[int]bool{}
	for _, ifi := range ifs {
		if !useInterfaceForNDP(ifi) {
			continue
		}
		if a.responders[ifi.Index] == nil {
			fifi := ifi
			resp, err := newNDPResponder(&fifi, a.announce)
			if err != nil {
				return fmt.Errorf("Creating NDP responder for %q: %s", ifi.Name, err)
			}
			a.responders[ifi.Index] = resp
			glog.Infof("Created NDP listener on interface %d (%q)", ifi.Index, ifi.Name)
		}
		keep[ifi.Index] = true
	}

	for i, client := range a.responders {
		if !keep[i] {
			client.Close()
			delete(a.responders, i)
			glog.Infof("Deleted NDP listener on interface %d", i)
		}
	}

	return nil
}

func useInterfaceForNDP(ifi net.Interface) bool {
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
		if ipaddr.IP.To4() != nil {
			continue
		}
		if !ipaddr.IP.IsGlobalUnicast() {
			continue
		}
		return true
	}

	return false
}

type ndpResponder struct {
	hardwareAddr net.HardwareAddr
	conn         *ndp.Conn
	announce     announceFunc
	// Refcount of how many watchers for each solicited node
	// multicast group.
	solicitedNodeGroups map[string]int64
}

func newNDPResponder(ifi *net.Interface, ann announceFunc) (*ndpResponder, error) {
	// Use link-local address as the source IPv6 address for NDP communications.
	conn, _, err := ndp.Dial(ifi, ndp.LinkLocal)
	if err != nil {
		return nil, fmt.Errorf("creating NDP responder for %q: %s", ifi.Name, err)
	}

	ret := &ndpResponder{
		hardwareAddr:        ifi.HardwareAddr,
		conn:                conn,
		announce:            ann,
		solicitedNodeGroups: map[string]int64{},
	}
	go ret.run()
	return ret, nil
}

func (n *ndpResponder) Close() error {
	return n.conn.Close()
}

func (n *ndpResponder) Gratuitous(ip net.IP) error {
	err := n.advertise(net.IPv6linklocalallnodes, ip, true)
	stats.SentGratuitous(ip.String())
	return err
}

func (n *ndpResponder) Watch(ip net.IP) error {
	if ip.To4() != nil {
		return nil
	}
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		return fmt.Errorf("looking up solicited node multicast group for %q: %s", ip, err)
	}
	if n.solicitedNodeGroups[group.String()] == 0 {
		if err = n.conn.JoinGroup(group); err != nil {
			return fmt.Errorf("joining solicited node multicast group for %q: %s", ip, err)
		}
	}
	n.solicitedNodeGroups[group.String()]++
	return nil
}

func (n *ndpResponder) Unwatch(ip net.IP) error {
	if ip.To4() != nil {
		return nil
	}
	group, err := ndp.SolicitedNodeMulticast(ip)
	if err != nil {
		return fmt.Errorf("looking up solicited node multicast group for %q: %s", ip, err)
	}
	n.solicitedNodeGroups[group.String()]--
	if n.solicitedNodeGroups[group.String()] == 0 {
		if err = n.conn.LeaveGroup(group); err != nil {
			return fmt.Errorf("leaving solicited node multicast group for %q: %s", ip, err)
		}
	}
	return nil
}

func (n *ndpResponder) run() {
	for n.processRequest() != dropReasonClosed {
	}
}

func (n *ndpResponder) processRequest() dropReason {
	msg, _, src, err := n.conn.ReadFrom()
	if err != nil {
		return dropReasonError
	}

	ns, ok := msg.(*ndp.NeighborSolicitation)
	if !ok {
		return dropReasonMessageType
	}

	// Retrieve sender's source link-layer address
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

	// Ignore NDP requests that the announcer tells us to ignore.
	if reason := n.announce(ns.TargetAddress); reason != dropReasonNone {
		return reason
	}

	stats.GotRequest(ns.TargetAddress.String())

	glog.Infof("Request: who-has %s?  tell %s (%s). reply: %s is-at %s", ns.TargetAddress, src, nsLLAddr, ns.TargetAddress, n.hardwareAddr)

	if err := n.advertise(src, ns.TargetAddress, false); err != nil {
		glog.Warningf("Failed to write NDP neighbor advertisement for %s: %s", ns.TargetAddress, err)
	} else {
		stats.SentResponse(ns.TargetAddress.String())
	}
	return dropReasonNone
}

func (n *ndpResponder) advertise(dst, target net.IP, gratuitous bool) error {
	m := &ndp.NeighborAdvertisement{
		Solicited:     !gratuitous, // <Adam Jensen> I never asked for this...
		Override:      gratuitous,  // Should clients replace existing cache entries
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      n.hardwareAddr,
			},
		},
	}
	return n.conn.WriteTo(m, nil, dst)
}
