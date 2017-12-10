package arp

import (
	"time"

	"github.com/mdlayher/ethernet"
)

// Leader returns true if we are the leader in the daemonSet.
func (a *Announce) Leader() bool {
	a.leaderMu.RLock()
	defer a.leaderMu.RUnlock()
	return a.leader
}

// SetLeader sets the leader boolean to b.
func (a *Announce) SetLeader(b bool) {
	a.leaderMu.Lock()
	defer a.leaderMu.Unlock()
	a.leader = b
}

// Acquire sets the leader bit to true and sends out a unsolicited ARP replies for all VIPs that should
// be announced. It does this repeatedly - every 0.5s - for a duration of 5 seconds.
func (a *Announce) Acquire() {
	start := time.Now()

	a.SetLeader(true)

	for time.Since(start) < 5*time.Second {

		for _, u := range a.Unsolicited() {
			a.client.WriteTo(u, ethernet.Broadcast)
		}

		time.Sleep(500 * time.Millisecond)
	}
}
