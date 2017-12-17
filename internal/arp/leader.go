package arp

import (
	"time"

	"github.com/mdlayher/ethernet"
)

// Leader returns true if we are the leader in the daemon set.
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

// Relinquish set the leader bit to false and stops the go-routine that sends unsolicited APR replies.
func (a *Announce) Relinquish() {
	a.stop <- true

	a.SetLeader(false)
}

// Acquire sets the leader bit to true and sends out a unsolicited ARP replies for all VIPs that should
// be announced. It does this repeatedly - every 0.5s - for a duration of 5 seconds.
func (a *Announce) Acquire() {
	start := time.Now()

	a.SetLeader(true)

	for time.Since(start) < 5*time.Second {

		for _, u := range a.Packets() {
			a.client.WriteTo(u, ethernet.Broadcast)
		}

		time.Sleep(500 * time.Millisecond)
	}
	go a.unsolicited()
}

// unsolicited sends unsolicited ARP replies every 10 seconds.
func (a *Announce) unsolicited() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, u := range a.Packets() {
				a.client.WriteTo(u, ethernet.Broadcast)
			}

		case <-a.stop:
			return
		}
	}
}
