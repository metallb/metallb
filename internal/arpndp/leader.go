package arp

import (
	"time"

	"github.com/golang/glog"
)

// Leader returns true if we are the leader in the daemon set.
func (a *Announce) Leader() bool {
	a.RLock()
	defer a.RUnlock()
	return a.leader
}

// SetLeader sets the leader boolean to b.
func (a *Announce) SetLeader(b bool) {
	a.Lock()
	defer a.Unlock()
	a.leader = b
	if a.leader {
		go a.Acquire()
	}
}

// Acquire sends out a unsolicited ARP replies for all VIPs that should be announced.
func (a *Announce) Acquire() {
	go a.spam()
}

// spam broadcasts unsolicited ARP replies for 5 seconds.
func (a *Announce) spam() {
	start := time.Now()
	for time.Since(start) < 5*time.Second {

		if !a.Leader() {
			return
		}

		for _, ip := range a.ips {
			var err error
			if ip.To4() == nil {
				err = a.ndpResponder.Gratuitous(ip)
			} else {
				err = a.arpResponder.Gratuitous(ip)
			}
			if err != nil {
				glog.Errorf("Broadcasting gratuitous ARP/NDP for %q: %s", ip, err)
			}
		}
		time.Sleep(1100 * time.Millisecond)
	}
}
